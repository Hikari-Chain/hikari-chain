package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	govtypes "github.com/Hikari-Chain/hikari-chain/x/gov/types"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// Shield implements the MsgServer.Shield method.
// It moves coins from the sender's public balance into the privacy pool.
// Phase 1: Creates a simple indexed deposit with stealth address and Pedersen commitment.
// Phase 2: Adds the commitment to a Merkle tree for unlinkability.
func (k msgServer) Shield(goCtx context.Context, msg *types.MsgShield) (*types.MsgShieldResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Get and validate parameters
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get params")
	}

	if !params.Enabled {
		return nil, types.ErrModuleDisabled
	}

	// Validate sender address
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errors.Wrap(err, "invalid sender address")
	}

	// Validate denomination is allowed
	denom := msg.Amount.Denom
	allowed := false
	for _, allowedDenom := range params.AllowedDenoms {
		if denom == allowedDenom {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.Wrapf(types.ErrDenomNotAllowed, "denomination %s is not allowed for privacy operations", denom)
	}

	// Validate amount is positive
	if !msg.Amount.IsPositive() {
		return nil, errors.Wrap(types.ErrInvalidAmount, "amount must be positive")
	}

	// Check minimum shield amount
	minAmountStr, exists := params.MinShieldAmounts[denom]
	if exists && minAmountStr != "" {
		minAmount, ok := math.NewIntFromString(minAmountStr)
		if !ok {
			return nil, errors.Wrapf(types.ErrInvalidAmount, "invalid minimum shield amount for %s", denom)
		}
		if msg.Amount.Amount.LT(minAmount) {
			return nil, errors.Wrapf(types.ErrAmountBelowMinimum, "amount %s is below minimum %s for %s", msg.Amount.Amount.String(), minAmount.String(), denom)
		}
	}

	// Validate one-time address (stealth address)
	if err := validateECPoint(&msg.OneTimeAddress.Address); err != nil {
		return nil, errors.Wrap(types.ErrInvalidOneTimeAddress, err.Error())
	}
	if err := validateECPoint(&msg.OneTimeAddress.TxPublicKey); err != nil {
		return nil, errors.Wrap(types.ErrInvalidOneTimeAddress, err.Error())
	}

	// Validate Pedersen commitment
	if err := validateECPoint(&msg.Commitment.Commitment); err != nil {
		return nil, errors.Wrap(types.ErrInvalidCommitment, err.Error())
	}

	// Validate encrypted note
	if len(msg.EncryptedNote.EncryptedData) == 0 {
		return nil, errors.Wrap(types.ErrInvalidNote, "encrypted data is empty")
	}
	if len(msg.EncryptedNote.EncryptedData) > int(params.MaxMemoSize)+48 { // 48 = 8 (amount) + 32 (blinding) + 16 (auth tag) - extra overhead
		return nil, errors.Wrap(types.ErrMemoTooLarge, "encrypted note exceeds maximum size")
	}
	if len(msg.EncryptedNote.Nonce) != 12 {
		return nil, errors.Wrap(types.ErrInvalidNote, "nonce must be 12 bytes for AES-GCM")
	}
	if err := validateECPoint(&msg.EncryptedNote.EphemeralKey); err != nil {
		return nil, errors.Wrap(types.ErrInvalidNote, err.Error())
	}

	// Burn coins from sender's public balance
	coinsToShield := sdk.NewCoins(msg.Amount)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, coinsToShield); err != nil {
		return nil, errors.Wrap(err, "failed to send coins to privacy module")
	}
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, coinsToShield); err != nil {
		return nil, errors.Wrap(err, "failed to burn coins")
	}

	// Get next deposit index and increment
	depositIndex, err := k.IncrementDepositIndex(ctx, denom)
	if err != nil {
		return nil, errors.Wrap(err, "failed to increment deposit index")
	}

	// Create private deposit
	deposit := &types.PrivateDeposit{
		Denom:           denom,
		Index:           depositIndex,
		Commitment:      msg.Commitment,
		OneTimeAddress:  msg.OneTimeAddress,
		EncryptedNote:   msg.EncryptedNote,
		Nullifier:       nil, // Not set until spent
		CreatedAtHeight: ctx.BlockHeight(),
		TxHash:          fmt.Sprintf("%X", ctx.TxBytes()),
	}

	// Store the deposit
	if err := k.SetDeposit(ctx, deposit); err != nil {
		return nil, errors.Wrap(err, "failed to store deposit")
	}

	// Emit event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeShield,
			sdk.NewAttribute(types.AttributeKeySender, msg.Sender),
			sdk.NewAttribute(types.AttributeKeyDenom, denom),
			sdk.NewAttribute(types.AttributeKeyAmount, msg.Amount.Amount.String()),
			sdk.NewAttribute(types.AttributeKeyDepositIndex, fmt.Sprintf("%d", depositIndex)),
			sdk.NewAttribute(types.AttributeKeyBlockHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	})

	k.Logger(ctx).Info("shielded coins to privacy pool",
		"sender", msg.Sender,
		"amount", msg.Amount.String(),
		"deposit_index", depositIndex,
	)

	// Phase 2: Would also update Merkle tree and return root
	// For Phase 1, merkle_root is empty
	return &types.MsgShieldResponse{
		Denom:        denom,
		DepositIndex: depositIndex,
		MerkleRoot:   nil,
	}, nil
}

// PrivateTransfer implements the MsgServer.PrivateTransfer method.
// It transfers funds within the privacy pool from input deposits to output deposits.
// Phase 1: Uses simple nullifiers and signatures, deposit indices are visible.
// Phase 2: Uses zk-SNARKs for full unlinkability.
func (k msgServer) PrivateTransfer(goCtx context.Context, msg *types.MsgPrivateTransfer) (*types.MsgPrivateTransferResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Get and validate parameters
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get params")
	}

	if !params.Enabled {
		return nil, types.ErrModuleDisabled
	}

	// Validate denomination is allowed
	denom := msg.Denom
	allowed := false
	for _, allowedDenom := range params.AllowedDenoms {
		if denom == allowedDenom {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.Wrapf(types.ErrDenomNotAllowed, "denomination %s is not allowed", denom)
	}

	// Validate inputs
	if len(msg.Inputs) == 0 {
		return nil, types.ErrEmptyInputs
	}
	if uint32(len(msg.Inputs)) > params.MaxDepositsPerTx {
		return nil, errors.Wrapf(types.ErrTooManyInputs, "got %d inputs, max %d", len(msg.Inputs), params.MaxDepositsPerTx)
	}

	// Validate outputs
	if len(msg.Outputs) == 0 {
		return nil, types.ErrEmptyOutputs
	}
	if uint32(len(msg.Outputs)) > params.MaxDepositsPerTx {
		return nil, errors.Wrapf(types.ErrTooManyOutputs, "got %d outputs, max %d", len(msg.Outputs), params.MaxDepositsPerTx)
	}

	// Process each input
	for i, input := range msg.Inputs {
		// Validate nullifier
		if len(input.Nullifier) == 0 {
			return nil, errors.Wrapf(types.ErrInvalidNullifier, "input %d has empty nullifier", i)
		}

		// Check if nullifier already used (double-spend check)
		used, err := k.CheckNullifierUsed(ctx, input.Nullifier)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check nullifier for input %d", i)
		}
		if used {
			return nil, errors.Wrapf(types.ErrNullifierAlreadyUsed, "input %d nullifier already used", i)
		}

		// Phase 1: Validate deposit exists and signature
		if params.Phase == "phase1" {
			deposit, err := k.GetDeposit(ctx, denom, input.DepositIndex)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get deposit for input %d", i)
			}
			if deposit == nil {
				return nil, errors.Wrapf(types.ErrDepositNotFound, "deposit %d not found for input %d", input.DepositIndex, i)
			}

			// Validate signature (Phase 1 only)
			// Verifies ECDSA signature over nullifier, proving ownership of the one-time private key
			if len(input.Signature) == 0 {
				return nil, errors.Wrapf(types.ErrInvalidSignature, "input %d missing signature (required in Phase 1)", i)
			}

			// Verify the signature proves ownership of the one-time private key
			if err := k.VerifyNullifierSignature(deposit, input.Nullifier, input.Signature); err != nil {
				return nil, errors.Wrapf(types.ErrInvalidSignature, "input %d signature verification failed: %v", i, err)
			}

			// Update the deposit to mark it as spent with the nullifier
			deposit.Nullifier = input.Nullifier
			if err := k.SetDeposit(ctx, deposit); err != nil {
				return nil, errors.Wrapf(err, "failed to update deposit %d with nullifier", i)
			}
		}

		// Mark nullifier as used
		usedNullifier := &types.UsedNullifier{
			Nullifier:     input.Nullifier,
			SpentAtHeight: ctx.BlockHeight(),
			SpentTxHash:   fmt.Sprintf("%X", ctx.TxBytes()),
			Denom:         denom,
		}
		if err := k.SetNullifierUsed(ctx, usedNullifier); err != nil {
			return nil, errors.Wrapf(err, "failed to mark nullifier as used for input %d", i)
		}
	}

	// Validate balance commitment
	// In Phase 1: We verify that C_balance = sum(C_inputs) - sum(C_outputs) has form 0*H + b*G
	// In Phase 2: This would be verified inside the zk-SNARK proof
	if err := validateECPoint(&msg.BalanceCommitment.Commitment); err != nil {
		return nil, errors.Wrap(types.ErrInvalidBalanceCommitment, err.Error())
	}

	// Phase 2: Verify zk-SNARK proof
	if params.Phase == "phase2" {
		if msg.ZkProof == nil || len(msg.ZkProof.Proof) == 0 {
			return nil, errors.Wrap(types.ErrInvalidZKProof, "zk proof required in Phase 2")
		}
		// TODO: Implement zk-SNARK verification using Groth16 or PLONK
		// This would verify:
		// - All inputs exist in Merkle tree
		// - Nullifiers correctly derived
		// - Sum(inputs) = Sum(outputs)
		// - All commitments well-formed
	}

	// Create output deposits
	outputIndices := make([]uint64, len(msg.Outputs))
	for i, output := range msg.Outputs {
		// Validate output
		if output.Denom != denom {
			return nil, errors.Wrapf(types.ErrInvalidDenom, "output %d has mismatched denom: expected %s, got %s", i, denom, output.Denom)
		}

		if err := validateECPoint(&output.OneTimeAddress.Address); err != nil {
			return nil, errors.Wrapf(types.ErrInvalidOneTimeAddress, "output %d: %s", i, err.Error())
		}
		if err := validateECPoint(&output.OneTimeAddress.TxPublicKey); err != nil {
			return nil, errors.Wrapf(types.ErrInvalidOneTimeAddress, "output %d: %s", i, err.Error())
		}
		if err := validateECPoint(&output.Commitment.Commitment); err != nil {
			return nil, errors.Wrapf(types.ErrInvalidCommitment, "output %d: %s", i, err.Error())
		}

		if len(output.EncryptedNote.EncryptedData) == 0 {
			return nil, errors.Wrapf(types.ErrInvalidNote, "output %d has empty encrypted data", i)
		}
		if len(output.EncryptedNote.Nonce) != 12 {
			return nil, errors.Wrapf(types.ErrInvalidNote, "output %d has invalid nonce length", i)
		}
		if err := validateECPoint(&output.EncryptedNote.EphemeralKey); err != nil {
			return nil, errors.Wrapf(types.ErrInvalidNote, "output %d: %s", i, err.Error())
		}

		// Get next deposit index
		depositIndex, err := k.IncrementDepositIndex(ctx, denom)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to increment deposit index for output %d", i)
		}
		outputIndices[i] = depositIndex

		// Create output deposit
		deposit := &types.PrivateDeposit{
			Denom:           denom,
			Index:           depositIndex,
			Commitment:      output.Commitment,
			OneTimeAddress:  output.OneTimeAddress,
			EncryptedNote:   output.EncryptedNote,
			Nullifier:       nil, // Not set until spent
			CreatedAtHeight: ctx.BlockHeight(),
			TxHash:          fmt.Sprintf("%X", ctx.TxBytes()),
		}

		if err := k.SetDeposit(ctx, deposit); err != nil {
			return nil, errors.Wrapf(err, "failed to store output deposit %d", i)
		}
	}

	// Emit event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypePrivateTransfer,
			sdk.NewAttribute(types.AttributeKeyDenom, denom),
			sdk.NewAttribute(types.AttributeKeyInputCount, fmt.Sprintf("%d", len(msg.Inputs))),
			sdk.NewAttribute(types.AttributeKeyOutputCount, fmt.Sprintf("%d", len(msg.Outputs))),
			sdk.NewAttribute(types.AttributeKeyBlockHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	})

	k.Logger(ctx).Info("private transfer completed",
		"denom", denom,
		"inputs", len(msg.Inputs),
		"outputs", len(msg.Outputs),
	)

	return &types.MsgPrivateTransferResponse{
		OutputIndices: outputIndices,
		MerkleRoot:    nil, // Phase 2 only
	}, nil
}

// Unshield implements the MsgServer.Unshield method.
// It moves coins from the privacy pool back to a public balance.
// Phase 1: Uses deposit index (visible) and signature for authorization.
// Phase 2: Uses zk-SNARK proof to hide which deposit is being spent.
func (k msgServer) Unshield(goCtx context.Context, msg *types.MsgUnshield) (*types.MsgUnshieldResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Get and validate parameters
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get params")
	}

	if !params.Enabled {
		return nil, types.ErrModuleDisabled
	}

	// Validate recipient address
	recipient, err := sdk.AccAddressFromBech32(msg.Recipient)
	if err != nil {
		return nil, errors.Wrap(err, "invalid recipient address")
	}

	// Validate denomination
	denom := msg.Denom
	allowed := false
	for _, allowedDenom := range params.AllowedDenoms {
		if denom == allowedDenom {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.Wrapf(types.ErrDenomNotAllowed, "denomination %s is not allowed", denom)
	}

	// Validate amount
	amount, ok := math.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, errors.Wrap(types.ErrInvalidAmount, "amount must be a positive integer")
	}

	// Validate nullifier
	if len(msg.Nullifier) == 0 {
		return nil, types.ErrInvalidNullifier
	}

	// Check if nullifier already used
	used, err := k.CheckNullifierUsed(ctx, msg.Nullifier)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check nullifier")
	}
	if used {
		return nil, types.ErrNullifierAlreadyUsed
	}

	// Validate commitment
	if err := validateECPoint(&msg.Commitment.Commitment); err != nil {
		return nil, errors.Wrap(types.ErrInvalidCommitment, err.Error())
	}

	// Phase 1: Verify deposit exists and signature
	if params.Phase == "phase1" {
		deposit, err := k.GetDeposit(ctx, denom, msg.DepositIndex)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get deposit")
		}
		if deposit == nil {
			return nil, errors.Wrapf(types.ErrDepositNotFound, "deposit %d not found", msg.DepositIndex)
		}

		// Validate signature
		if len(msg.Signature) == 0 {
			return nil, errors.Wrap(types.ErrInvalidSignature, "signature required in Phase 1")
		}

		// Verify signature over (nullifier || recipient || amount)
		if err := k.VerifyUnshieldSignature(deposit, msg.Nullifier, msg.Recipient, msg.Amount, msg.Signature); err != nil {
			return nil, errors.Wrapf(types.ErrInvalidSignature, "signature verification failed: %v", err)
		}

		// Update the deposit to mark it as spent with the nullifier
		deposit.Nullifier = msg.Nullifier
		if err := k.SetDeposit(ctx, deposit); err != nil {
			return nil, errors.Wrap(err, "failed to update deposit with nullifier")
		}
	}

	// Phase 2: Verify zk-SNARK proof
	if params.Phase == "phase2" {
		if msg.ZkProof == nil || len(msg.ZkProof.Proof) == 0 {
			return nil, errors.Wrap(types.ErrInvalidZKProof, "zk proof required in Phase 2")
		}
		// TODO: Implement zk-SNARK verification
		// The proof should verify:
		// - Deposit exists in Merkle tree
		// - Nullifier correctly derived from deposit
		// - Amount in public input matches commitment
		// - Recipient has authority to spend
	}

	// Mark nullifier as used
	usedNullifier := &types.UsedNullifier{
		Nullifier:     msg.Nullifier,
		SpentAtHeight: ctx.BlockHeight(),
		SpentTxHash:   fmt.Sprintf("%X", ctx.TxBytes()),
		Denom:         denom,
	}
	if err := k.SetNullifierUsed(ctx, usedNullifier); err != nil {
		return nil, errors.Wrap(err, "failed to mark nullifier as used")
	}

	// Mint coins to recipient
	coin := sdk.NewCoin(denom, amount)
	coinsToMint := sdk.NewCoins(coin)

	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coinsToMint); err != nil {
		return nil, errors.Wrap(err, "failed to mint coins")
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coinsToMint); err != nil {
		return nil, errors.Wrap(err, "failed to send coins to recipient")
	}

	// Emit event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeUnshield,
			sdk.NewAttribute(types.AttributeKeyRecipient, msg.Recipient),
			sdk.NewAttribute(types.AttributeKeyDenom, denom),
			sdk.NewAttribute(types.AttributeKeyAmount, amount.String()),
			sdk.NewAttribute(types.AttributeKeyDepositIndex, fmt.Sprintf("%d", msg.DepositIndex)),
			sdk.NewAttribute(types.AttributeKeyBlockHeight, fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	})

	k.Logger(ctx).Info("unshielded coins from privacy pool",
		"recipient", msg.Recipient,
		"amount", coin.String(),
		"deposit_index", msg.DepositIndex,
	)

	return &types.MsgUnshieldResponse{
		Amount: coin,
	}, nil
}

// UpdateParams implements the MsgServer.UpdateParams method.
// It updates the privacy module parameters. Only the governance module can call this.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.authority != msg.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.authority, msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate params
	if err := msg.Params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid params")
	}

	// Set params
	if err := k.SetParams(ctx, msg.Params); err != nil {
		return nil, errors.Wrap(err, "failed to set params")
	}

	// Emit event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeUpdateParams,
			sdk.NewAttribute(types.AttributeKeyAuthority, msg.Authority),
		),
	})

	k.Logger(ctx).Info("updated privacy module parameters", "authority", msg.Authority)

	return &types.MsgUpdateParamsResponse{}, nil
}

// validateECPoint validates that an elliptic curve point is well-formed
func validateECPoint(point *types.ECPoint) error {
	if point == nil {
		return fmt.Errorf("point is nil")
	}
	if len(point.X) != 32 {
		return fmt.Errorf("x coordinate must be 32 bytes, got %d", len(point.X))
	}
	if len(point.Y) != 32 {
		return fmt.Errorf("y coordinate must be 32 bytes, got %d", len(point.Y))
	}
	// TODO: In a production implementation, we should verify the point is on the secp256k1 curve
	// and not the point at infinity. This requires:
	// 1. Parsing X and Y as field elements
	// 2. Verifying Y^2 = X^3 + 7 (mod p) where p is the secp256k1 field prime
	// 3. Checking (X, Y) != (0, 0)
	return nil
}

package cli

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/client/utils"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/crypto"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

// GetTxCmd returns the transaction commands for the privacy module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetTxShieldCmd(),
		GetTxPrivateTransferCmd(),
		GetTxUnshieldCmd(),
	)

	return cmd
}

// GetTxShieldCmd returns the command to shield coins into the privacy pool
func GetTxShieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shield [amount] [recipient-view-pubkey] [recipient-spend-pubkey]",
		Short: "Shield coins into the privacy pool",
		Long: `Shield (deposit) coins from your public balance into the privacy pool.
This creates a new private deposit that can only be spent by the recipient.

The recipient public keys should be provided as hex-encoded compressed secp256k1 points (33 bytes each).
For self-shielding, use your own view and spend public keys.`,
		Example: fmt.Sprintf(`
# Shield 1000ulight to yourself
%s tx privacy shield 1000ulight 02abc123... 03def456... --from mykey

# Shield to another recipient
%s tx privacy shield 500ulight 02pubkey1... 03pubkey2... --from sender
`, version.AppName, version.AppName),
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse amount
			amount, err := sdk.ParseCoinNormalized(args[0])
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}

			// Parse recipient view public key
			viewPubKeyBytes, err := hex.DecodeString(args[1])
			if err != nil {
				return fmt.Errorf("invalid view public key hex: %w", err)
			}
			if len(viewPubKeyBytes) != 33 {
				return fmt.Errorf("view public key must be 33 bytes (compressed), got %d", len(viewPubKeyBytes))
			}

			// Parse recipient spend public key
			spendPubKeyBytes, err := hex.DecodeString(args[2])
			if err != nil {
				return fmt.Errorf("invalid spend public key hex: %w", err)
			}
			if len(spendPubKeyBytes) != 33 {
				return fmt.Errorf("spend public key must be 33 bytes (compressed), got %d", len(spendPubKeyBytes))
			}

			// Decompress public keys
			recipientViewPubKey, err := utils.DecompressPubKey(viewPubKeyBytes)
			if err != nil {
				return fmt.Errorf("invalid view public key: %w", err)
			}

			recipientSpendPubKey, err := utils.DecompressPubKey(spendPubKeyBytes)
			if err != nil {
				return fmt.Errorf("invalid spend public key: %w", err)
			}

			// Generate stealth address
			stealthAddr, err := utils.GenerateStealthAddress(recipientViewPubKey, recipientSpendPubKey)
			if err != nil {
				return fmt.Errorf("failed to generate stealth address: %w", err)
			}

			// Create Pedersen commitment
			amountUint := amount.Amount.Uint64()
			commitment, blinding, err := utils.CreateCommitment(amountUint)
			if err != nil {
				return fmt.Errorf("failed to create commitment: %w", err)
			}

			// Encrypt note with amount and blinding factor
			encryptedNote, err := utils.EncryptNote(amountUint, blinding, stealthAddr.SharedSecret)
			if err != nil {
				return fmt.Errorf("failed to encrypt note: %w", err)
			}

			// Convert stealth address to proto format
			oneTimeAddress := types.OneTimeAddress{
				Address: types.ECPoint{
					X: stealthAddr.OneTimeAddress.X.Bytes(),
					Y: stealthAddr.OneTimeAddress.Y.Bytes(),
				},
				TxPublicKey: types.ECPoint{
					X: stealthAddr.TxPublicKey.X.Bytes(),
					Y: stealthAddr.TxPublicKey.Y.Bytes(),
				},
			}

			// Convert commitment to proto format
			pedersenCommitment := types.PedersenCommitment{
				Commitment: types.ECPoint{
					X: commitment.X.Bytes(),
					Y: commitment.Y.Bytes(),
				},
			}

			// Convert encrypted note to proto format
			note := types.Note{
				EncryptedData: encryptedNote.Ciphertext,
				Nonce:         encryptedNote.Nonce,
				EphemeralKey: types.ECPoint{
					X: encryptedNote.EphemeralKey.X.Bytes(),
					Y: encryptedNote.EphemeralKey.Y.Bytes(),
				},
			}

			// Create message
			msg := &types.MsgShield{
				Sender:         clientCtx.GetFromAddress().String(),
				Amount:         amount,
				OneTimeAddress: oneTimeAddress,
				Commitment:     pedersenCommitment,
				EncryptedNote:  note,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// GetTxPrivateTransferCmd returns the command to transfer within the privacy pool
func GetTxPrivateTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [denom] [input-deposit-index] [output-amount1,recipient-view-key1,recipient-spend-key1] [...]",
		Short: "Transfer coins within the privacy pool (Phase 1)",
		Long: `Transfer coins from your private deposits to new private deposits.

Phase 1: This command requires specifying deposit indices (visible on-chain).
Each output is specified as: amount,view-pubkey,spend-pubkey

Example output format: 1000,02abc...,03def...

The sum of output amounts must equal the input deposit amount.`,
		Example: fmt.Sprintf(`
# Transfer 1000ulight from deposit 5 to two recipients (600 + 400 = 1000)
%s tx privacy transfer ulight 5 \
  600,02view1...,03spend1... \
  400,02view2...,03spend2... \
  --from mykey --view-key <hex> --spend-key <hex>

# Split deposit into change output (send 700 to Bob, 300 back to yourself)
%s tx privacy transfer ulight 5 \
  700,02bob_view...,03bob_spend... \
  300,02my_view...,03my_spend... \
  --from mykey --view-key <hex> --spend-key <hex>
`, version.AppName, version.AppName),
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse arguments
			denom := args[0]
			inputDepositIndex, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid input deposit index: %w", err)
			}

			// Get private keys from flags
			viewKeyHex, err := cmd.Flags().GetString("view-key")
			if err != nil || viewKeyHex == "" {
				return fmt.Errorf("view-key flag is required")
			}

			spendKeyHex, err := cmd.Flags().GetString("spend-key")
			if err != nil || spendKeyHex == "" {
				return fmt.Errorf("spend-key flag is required")
			}

			// Parse private keys
			viewPrivKey, spendPrivKey, err := utils.ParsePrivateKeys(viewKeyHex, spendKeyHex)
			if err != nil {
				return fmt.Errorf("failed to parse private keys: %w", err)
			}

			// Compute public keys
			_, spendPubKey, err := utils.ComputePublicKeys(viewPrivKey, spendPrivKey)
			if err != nil {
				return fmt.Errorf("failed to compute public keys: %w", err)
			}

			// Query the input deposit from the chain
			queryClient := types.NewQueryClient(clientCtx)
			depositRes, err := queryClient.Deposit(cmd.Context(), &types.QueryDepositRequest{
				Denom: denom,
				Index: inputDepositIndex,
			})
			if err != nil {
				return fmt.Errorf("failed to query input deposit: %w", err)
			}

			inputDeposit := depositRes.Deposit

			// Convert input deposit to crypto types for scanning
			inputOneTimeAddr, err := protoPointToCrypto(&inputDeposit.OneTimeAddress.Address)
			if err != nil {
				return fmt.Errorf("invalid input one-time address: %w", err)
			}

			inputTxPubKey, err := protoPointToCrypto(&inputDeposit.OneTimeAddress.TxPublicKey)
			if err != nil {
				return fmt.Errorf("invalid input tx public key: %w", err)
			}

			inputCommitment, err := protoPointToCrypto(&inputDeposit.Commitment.Commitment)
			if err != nil {
				return fmt.Errorf("invalid input commitment: %w", err)
			}

			// Scan the input deposit to verify ownership and decrypt
			ownedInput, err := utils.ScanDeposit(
				denom,
				inputDepositIndex,
				inputOneTimeAddr,
				inputTxPubKey,
				inputCommitment,
				inputDeposit.EncryptedNote.EncryptedData,
				inputDeposit.EncryptedNote.Nonce,
				inputDeposit.CreatedAtHeight,
				inputDeposit.TxHash,
				viewPrivKey,
				spendPubKey,
				spendPrivKey,
			)
			if err != nil {
				return fmt.Errorf("failed to scan input deposit: %w", err)
			}

			if ownedInput == nil {
				return fmt.Errorf("input deposit %d does not belong to you", inputDepositIndex)
			}

			// Parse output specifications (amount,view-pubkey,spend-pubkey)
			outputSpecs := args[2:]
			if len(outputSpecs) == 0 {
				return fmt.Errorf("at least one output is required")
			}

			outputs := make([]types.TransferOutput, 0, len(outputSpecs))
			totalOutputAmount := uint64(0)

			for i, spec := range outputSpecs {
				output, amount, err := parseTransferOutput(spec, denom, i)
				if err != nil {
					return fmt.Errorf("invalid output %d: %w", i, err)
				}
				outputs = append(outputs, output)
				totalOutputAmount += amount
			}

			// Verify balance: input amount must equal sum of output amounts
			if ownedInput.Amount != totalOutputAmount {
				return fmt.Errorf("balance mismatch: input amount is %d but outputs sum to %d", ownedInput.Amount, totalOutputAmount)
			}

			// Generate nullifier and signature for input
			inputNullifier, inputSignature, err := utils.PreparePrivateTransferInput(ownedInput)
			if err != nil {
				return fmt.Errorf("failed to prepare input: %w", err)
			}

			// Create input
			input := types.TransferInput{
				DepositIndex: inputDepositIndex,
				Nullifier:    inputNullifier,
				Signature:    inputSignature,
			}

			// Create balance commitment (should be zero since input = sum(outputs))
			// For Phase 1, we create a zero commitment: 0*H + 0*G
			balanceCommitment := types.PedersenCommitment{
				Commitment: types.ECPoint{
					X: make([]byte, 32), // Zero point - this is a simplification
					Y: make([]byte, 32), // In production, use proper zero/identity handling
				},
			}

			// Create the private transfer message
			msg := &types.MsgPrivateTransfer{
				Sender:            clientCtx.GetFromAddress().String(),
				Denom:             denom,
				Inputs:            []types.TransferInput{input},
				Outputs:           outputs,
				BalanceCommitment: balanceCommitment,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("view-key", "", "Your view private key (hex) - required")
	cmd.Flags().String("spend-key", "", "Your spend private key (hex) - required")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// GetTxUnshieldCmd returns the command to unshield coins from the privacy pool
func GetTxUnshieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unshield [recipient] [denom] [amount] [deposit-index]",
		Short: "Unshield coins from the privacy pool to a public address (Phase 1)",
		Long: `Unshield (withdraw) coins from the privacy pool back to a public address.

Phase 1: This command requires specifying the deposit index (visible on-chain).
You must provide your view and spend private keys to generate the necessary proofs.`,
		Example: fmt.Sprintf(`
# Unshield 1000ulight from deposit 5 to a public address
%s tx privacy unshield hikari1... ulight 1000 5 \
  --from mykey --view-key <hex> --spend-key <hex>
`, version.AppName),
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse arguments
			recipientAddr := args[0]
			denom := args[1]
			amount := args[2]
			depositIndex, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid deposit index: %w", err)
			}

			// Validate recipient address
			if _, err := sdk.AccAddressFromBech32(recipientAddr); err != nil {
				return fmt.Errorf("invalid recipient address: %w", err)
			}

			// Get private keys from flags
			viewKeyHex, err := cmd.Flags().GetString("view-key")
			if err != nil || viewKeyHex == "" {
				return fmt.Errorf("view-key flag is required")
			}

			spendKeyHex, err := cmd.Flags().GetString("spend-key")
			if err != nil || spendKeyHex == "" {
				return fmt.Errorf("spend-key flag is required")
			}

			// Parse private keys
			viewPrivKey, spendPrivKey, err := utils.ParsePrivateKeys(viewKeyHex, spendKeyHex)
			if err != nil {
				return fmt.Errorf("failed to parse private keys: %w", err)
			}

			// Compute spend public key (view public key not needed for unshield)
			_, spendPubKey, err := utils.ComputePublicKeys(viewPrivKey, spendPrivKey)
			if err != nil {
				return fmt.Errorf("failed to compute public keys: %w", err)
			}

			// Query the deposit from the chain
			queryClient := types.NewQueryClient(clientCtx)
			depositRes, err := queryClient.Deposit(cmd.Context(), &types.QueryDepositRequest{
				Denom: denom,
				Index: depositIndex,
			})
			if err != nil {
				return fmt.Errorf("failed to query deposit: %w", err)
			}

			deposit := depositRes.Deposit

			// Convert deposit to crypto types for scanning
			oneTimeAddr, err := protoPointToCrypto(&deposit.OneTimeAddress.Address)
			if err != nil {
				return fmt.Errorf("invalid one-time address: %w", err)
			}

			txPubKey, err := protoPointToCrypto(&deposit.OneTimeAddress.TxPublicKey)
			if err != nil {
				return fmt.Errorf("invalid tx public key: %w", err)
			}

			commitment, err := protoPointToCrypto(&deposit.Commitment.Commitment)
			if err != nil {
				return fmt.Errorf("invalid commitment: %w", err)
			}

			// Scan the deposit to check if it's mine and decrypt it
			ownedDeposit, err := utils.ScanDeposit(
				denom,
				depositIndex,
				oneTimeAddr,
				txPubKey,
				commitment,
				deposit.EncryptedNote.EncryptedData,
				deposit.EncryptedNote.Nonce,
				deposit.CreatedAtHeight,
				deposit.TxHash,
				viewPrivKey,
				spendPubKey,
				spendPrivKey,
			)
			if err != nil {
				return fmt.Errorf("failed to scan deposit: %w", err)
			}

			if ownedDeposit == nil {
				return fmt.Errorf("deposit %d does not belong to you", depositIndex)
			}

			// Prepare the unshield transaction
			nullifierBytes, signature, err := utils.PrepareUnshield(ownedDeposit, recipientAddr, amount)
			if err != nil {
				return fmt.Errorf("failed to prepare unshield: %w", err)
			}

			// Convert commitment to proto format
			commitmentProto := types.PedersenCommitment{
				Commitment: types.ECPoint{
					X: ownedDeposit.Commitment.X.Bytes(),
					Y: ownedDeposit.Commitment.Y.Bytes(),
				},
			}

			// Create the unshield message
			msg := &types.MsgUnshield{
				Recipient:    recipientAddr,
				Denom:        denom,
				Amount:       amount,
				DepositIndex: depositIndex,
				Nullifier:    nullifierBytes,
				Commitment:   commitmentProto,
				Signature:    signature,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("view-key", "", "Your view private key (hex) - required")
	cmd.Flags().String("spend-key", "", "Your spend private key (hex) - required")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// protoPointToCrypto converts a protobuf ECPoint to a crypto.ECPoint
func protoPointToCrypto(point *types.ECPoint) (*crypto.ECPoint, error) {
	if point == nil {
		return nil, fmt.Errorf("point is nil")
	}
	if len(point.X) != 32 || len(point.Y) != 32 {
		return nil, fmt.Errorf("invalid point coordinates: X=%d bytes, Y=%d bytes", len(point.X), len(point.Y))
	}

	x := new(big.Int).SetBytes(point.X)
	y := new(big.Int).SetBytes(point.Y)

	ecPoint := crypto.NewECPoint(x, y)
	if !ecPoint.IsOnCurve() {
		return nil, fmt.Errorf("point is not on curve")
	}

	return ecPoint, nil
}

// parseTransferOutput parses a transfer output specification string
// Format: "amount,view-pubkey-hex,spend-pubkey-hex"
// Example: "1000,02abc123...,03def456..."
// Returns: (TransferOutput, amount, error)
func parseTransferOutput(spec string, denom string, _ int) (types.TransferOutput, uint64, error) {
	parts := splitOutputSpec(spec)
	if len(parts) != 3 {
		return types.TransferOutput{}, 0, fmt.Errorf("output must have format 'amount,view-pubkey,spend-pubkey', got %d parts", len(parts))
	}

	// Parse amount
	amount, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("invalid amount: %w", err)
	}
	if amount == 0 {
		return types.TransferOutput{}, 0, fmt.Errorf("amount must be positive")
	}

	// Parse recipient view public key
	viewPubKeyBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("invalid view public key hex: %w", err)
	}
	if len(viewPubKeyBytes) != 33 {
		return types.TransferOutput{}, 0, fmt.Errorf("view public key must be 33 bytes (compressed), got %d", len(viewPubKeyBytes))
	}

	// Parse recipient spend public key
	spendPubKeyBytes, err := hex.DecodeString(parts[2])
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("invalid spend public key hex: %w", err)
	}
	if len(spendPubKeyBytes) != 33 {
		return types.TransferOutput{}, 0, fmt.Errorf("spend public key must be 33 bytes (compressed), got %d", len(spendPubKeyBytes))
	}

	// Decompress public keys
	recipientViewPubKey, err := utils.DecompressPubKey(viewPubKeyBytes)
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("invalid view public key: %w", err)
	}

	recipientSpendPubKey, err := utils.DecompressPubKey(spendPubKeyBytes)
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("invalid spend public key: %w", err)
	}

	// Generate stealth address for this output
	stealthAddr, err := utils.GenerateStealthAddress(recipientViewPubKey, recipientSpendPubKey)
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("failed to generate stealth address: %w", err)
	}

	// Create Pedersen commitment for this output amount
	commitment, blinding, err := utils.CreateCommitment(amount)
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("failed to create commitment: %w", err)
	}

	// Encrypt note with amount and blinding factor
	encryptedNote, err := utils.EncryptNote(amount, blinding, stealthAddr.SharedSecret)
	if err != nil {
		return types.TransferOutput{}, 0, fmt.Errorf("failed to encrypt note: %w", err)
	}

	// Build the TransferOutput
	output := types.TransferOutput{
		Denom: denom,
		Commitment: types.PedersenCommitment{
			Commitment: types.ECPoint{
				X: commitment.X.Bytes(),
				Y: commitment.Y.Bytes(),
			},
		},
		OneTimeAddress: types.OneTimeAddress{
			Address: types.ECPoint{
				X: stealthAddr.OneTimeAddress.X.Bytes(),
				Y: stealthAddr.OneTimeAddress.Y.Bytes(),
			},
			TxPublicKey: types.ECPoint{
				X: stealthAddr.TxPublicKey.X.Bytes(),
				Y: stealthAddr.TxPublicKey.Y.Bytes(),
			},
		},
		EncryptedNote: types.Note{
			EncryptedData: encryptedNote.Ciphertext,
			Nonce:         encryptedNote.Nonce,
			EphemeralKey: types.ECPoint{
				X: encryptedNote.EphemeralKey.X.Bytes(),
				Y: encryptedNote.EphemeralKey.Y.Bytes(),
			},
		},
	}

	return output, amount, nil
}

// splitOutputSpec splits an output specification by commas, handling potential commas in keys
func splitOutputSpec(spec string) []string {
	// Simple split by comma - assumes no commas in the hex strings (which is correct)
	parts := make([]string, 0, 3)
	start := 0
	commaCount := 0

	for i, c := range spec {
		if c == ',' {
			parts = append(parts, spec[start:i])
			start = i + 1
			commaCount++
			if commaCount >= 2 {
				// Last part is everything after the second comma
				parts = append(parts, spec[start:])
				break
			}
		}
	}

	// If we didn't find 2 commas, add the last part
	switch commaCount {
	case 1:
		parts = append(parts, spec[start:])
	case 0:
		parts = append(parts, spec)
	}

	return parts
}

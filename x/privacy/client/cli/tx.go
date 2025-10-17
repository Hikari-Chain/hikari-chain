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

Example output format: 1000,02abc...,03def...`,
		Example: fmt.Sprintf(`
# Transfer from deposit 5 to create two outputs
%s tx privacy transfer ulight 5 \
  1000,02view1...,03spend1... \
  500,02view2...,03spend2... \
  --from mykey --view-key <hex> --spend-key <hex>
`, version.AppName),
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement full private transfer logic
			// For now, return an error indicating this is not yet implemented
			return fmt.Errorf("private transfer requires scanning deposits - full implementation pending")
		},
	}

	cmd.Flags().String("view-key", "", "Your view private key (hex)")
	cmd.Flags().String("spend-key", "", "Your spend private key (hex)")
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

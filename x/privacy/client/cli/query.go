package cli

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/version"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/client/utils"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/crypto"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

// GetQueryCmd returns the cli query commands for the privacy module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetQueryParamsCmd(),
		GetQueryDepositsCmd(),
		GetQueryDepositCmd(),
		GetQueryStatsCmd(),
		GetQueryNullifierUsedCmd(),
		GetQueryDepositsByRangeCmd(),
		GetQueryScanCmd(),
	)

	return cmd
}

// GetQueryParamsCmd returns the command to query privacy module parameters
func GetQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the privacy module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetQueryDepositsCmd returns the command to query all deposits for a denomination
func GetQueryDepositsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deposits [denom]",
		Short: "Query all private deposits for a denomination",
		Long: `Query all private deposits for the specified denomination.
This returns the on-chain deposits with their commitments, stealth addresses, and encrypted notes.

Use pagination flags to control the results.`,
		Example: fmt.Sprintf(`
# Query all ulight deposits
%s query privacy deposits ulight

# Query with pagination
%s query privacy deposits ulight --page 2 --limit 50
`, version.AppName, version.AppName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Deposits(context.Background(), &types.QueryDepositsRequest{
				Denom:      denom,
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddPaginationFlagsToCmd(cmd, "deposits")
	return cmd
}

// GetQueryDepositCmd returns the command to query a specific deposit
func GetQueryDepositCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deposit [denom] [index]",
		Short: "Query a specific private deposit by index",
		Example: fmt.Sprintf(`
# Query deposit 42 for ulight
%s query privacy deposit ulight 42
`, version.AppName),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]
			index, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid deposit index: %w", err)
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Deposit(context.Background(), &types.QueryDepositRequest{
				Denom: denom,
				Index: index,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetQueryStatsCmd returns the command to query privacy pool statistics
func GetQueryStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Query privacy pool statistics",
		Long: `Query statistics about the privacy pool, including:
- Total deposits per denomination
- Total shielded value
- Number of nullifiers used
- Number of transfers`,
		Example: fmt.Sprintf(`
# Query privacy pool stats
%s query privacy stats
`, version.AppName),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Stats(context.Background(), &types.QueryStatsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetQueryNullifierUsedCmd returns the command to check if a nullifier is used
func GetQueryNullifierUsedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nullifier-used [nullifier-hex]",
		Short: "Check if a nullifier has been used (spent)",
		Long: `Check if a nullifier has been used to prevent double-spending.
The nullifier should be provided as a hex-encoded byte string.`,
		Example: fmt.Sprintf(`
# Check if a nullifier is used
%s query privacy nullifier-used 0123456789abcdef...
`, version.AppName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			nullifierHex := args[0]

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.IsNullifierUsed(context.Background(), &types.QueryIsNullifierUsedRequest{
				Nullifier: nullifierHex,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetQueryDepositsByRangeCmd returns the command to query deposits by index range
func GetQueryDepositsByRangeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deposits-by-range [denom] [start-index] [end-index]",
		Short: "Query deposits within an index range",
		Long: `Query all deposits for a denomination between start and end indices.
Useful for scanning deposits efficiently.`,
		Example: fmt.Sprintf(`
# Query deposits from index 100 to 200
%s query privacy deposits-by-range ulight 100 200
`, version.AppName),
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]

			startIndex, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid start index: %w", err)
			}

			endIndex, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid end index: %w", err)
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.DepositsByRange(context.Background(), &types.QueryDepositsByRangeRequest{
				Denom:      denom,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetQueryScanCmd returns the command to scan for owned deposits
func GetQueryScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [denom]",
		Short: "Scan the privacy pool for deposits you own",
		Long: `Scan all deposits in the privacy pool for the specified denomination
and display the ones that belong to you (that you can spend).

This command requires your view and spend private keys to identify and decrypt
deposits. It will display:
- Deposit indices
- Amounts (decrypted)
- Block heights
- Total balance

The scan process may take time for large numbers of deposits.`,
		Example: fmt.Sprintf(`
# Scan for all your ulight deposits
%s query privacy scan ulight --view-key <hex> --spend-key <hex>

# Scan a specific range for faster results
%s query privacy scan ulight --view-key <hex> --spend-key <hex> --start-index 0 --end-index 100
`, version.AppName, version.AppName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]

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

			// Get start and end indices from flags (optional)
			startIndex, _ := cmd.Flags().GetUint64("start-index")
			endIndex, _ := cmd.Flags().GetUint64("end-index")

			queryClient := types.NewQueryClient(clientCtx)

			// If no end index specified, get total deposit count
			if endIndex == 0 {
				statsRes, err := queryClient.Stats(cmd.Context(), &types.QueryStatsRequest{})
				if err != nil {
					return fmt.Errorf("failed to query stats: %w", err)
				}

				// Find the count for this denomination
				for _, stat := range statsRes.DenomStats {
					if stat.Denom == denom {
						if stat.TotalDeposits > 0 {
							endIndex = stat.TotalDeposits - 1 // Convert count to last index
						}
						break
					}
				}

				if endIndex == 0 && startIndex == 0 {
					fmt.Println("No deposits found for", denom)
					return nil
				}
			}

			fmt.Printf("Scanning deposits from index %d to %d for %s...\n", startIndex, endIndex, denom)

			// Query deposits in range
			depositsRes, err := queryClient.DepositsByRange(cmd.Context(), &types.QueryDepositsByRangeRequest{
				Denom:      denom,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			})
			if err != nil {
				return fmt.Errorf("failed to query deposits: %w", err)
			}

			// Scan each deposit
			ownedDeposits := make([]*depositInfo, 0)
			totalBalance := uint64(0)
			scannedCount := 0

			for _, deposit := range depositsRes.Deposits {
				scannedCount++
				if scannedCount%100 == 0 {
					fmt.Printf("Scanned %d deposits...\n", scannedCount)
				}

				// Convert deposit to crypto types
				oneTimeAddr, err := protoPointToCryptoQuery(&deposit.OneTimeAddress.Address)
				if err != nil {
					continue // Skip invalid deposits
				}

				txPubKey, err := protoPointToCryptoQuery(&deposit.OneTimeAddress.TxPublicKey)
				if err != nil {
					continue
				}

				commitment, err := protoPointToCryptoQuery(&deposit.Commitment.Commitment)
				if err != nil {
					continue
				}

				// Try to scan this deposit
				ownedDeposit, err := utils.ScanDeposit(
					denom,
					deposit.Index,
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
					continue // Failed to decrypt, not ours
				}

				if ownedDeposit != nil {
					// This is our deposit!
					ownedDeposits = append(ownedDeposits, &depositInfo{
						Index:       deposit.Index,
						Amount:      ownedDeposit.Amount,
						BlockHeight: deposit.CreatedAtHeight,
						TxHash:      deposit.TxHash,
						Nullifier:   deposit.Nullifier,
					})
					totalBalance += ownedDeposit.Amount
				}
			}

			fmt.Printf("\nScanning complete. Found %d owned deposits out of %d total.\n\n", len(ownedDeposits), scannedCount)

			// Display results
			if len(ownedDeposits) == 0 {
				fmt.Println("No deposits found that belong to you.")
				return nil
			}

			fmt.Println("Your Deposits:")
			fmt.Println("==============")
			for _, info := range ownedDeposits {
				status := "unspent"
				if len(info.Nullifier) > 0 {
					status = "spent"
				}
				fmt.Printf("\nIndex:       %d\n", info.Index)
				fmt.Printf("Amount:      %d %s\n", info.Amount, denom)
				fmt.Printf("Status:      %s\n", status)
				fmt.Printf("Block:       %d\n", info.BlockHeight)
				fmt.Printf("Tx Hash:     %s\n", info.TxHash)
			}

			fmt.Printf("\n==============\n")
			fmt.Printf("Total Balance: %d %s\n", totalBalance, denom)
			fmt.Printf("Spendable:     %d deposits\n", countUnspent(ownedDeposits))

			return nil
		},
	}

	cmd.Flags().String("view-key", "", "Your view private key (hex) - required")
	cmd.Flags().String("spend-key", "", "Your spend private key (hex) - required")
	cmd.Flags().Uint64("start-index", 0, "Start scanning from this deposit index (optional)")
	cmd.Flags().Uint64("end-index", 0, "Stop scanning at this deposit index (optional, defaults to last deposit)")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// depositInfo holds information about an owned deposit
type depositInfo struct {
	Index       uint64
	Amount      uint64
	BlockHeight int64
	TxHash      string
	Nullifier   []byte
}

// countUnspent counts the number of unspent deposits
func countUnspent(deposits []*depositInfo) int {
	count := 0
	for _, d := range deposits {
		if len(d.Nullifier) == 0 {
			count++
		}
	}
	return count
}

// protoPointToCryptoQuery converts a protobuf ECPoint to a crypto.ECPoint
func protoPointToCryptoQuery(point *types.ECPoint) (*crypto.ECPoint, error) {
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

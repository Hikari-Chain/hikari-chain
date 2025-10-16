package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/version"

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

package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/store/prefix"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

var _ types.QueryServer = Keeper{}

// Params returns the current privacy module parameters.
func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	params, err := k.GetParams(goCtx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get params")
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// Deposit returns a specific private deposit by denomination and index.
func (k Keeper) Deposit(goCtx context.Context, req *types.QueryDepositRequest) (*types.QueryDepositResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denomination cannot be empty")
	}

	deposit, err := k.GetDeposit(goCtx, req.Denom, req.Index)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get deposit: %v", err))
	}

	if deposit == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("deposit %d not found for denom %s", req.Index, req.Denom))
	}

	return &types.QueryDepositResponse{Deposit: *deposit}, nil
}

// Deposits returns all deposits for a specific denomination with pagination.
func (k Keeper) Deposits(goCtx context.Context, req *types.QueryDepositsRequest) (*types.QueryDepositsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denomination cannot be empty")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	store := ctx.KVStore(k.storeKey)

	// Create prefix for this denomination's deposits
	denomPrefix := append(types.DepositKeyPrefix, []byte(req.Denom)...)
	denomPrefix = append(denomPrefix, 0x00) // separator
	depositStore := prefix.NewStore(store, denomPrefix)

	var deposits []types.PrivateDeposit
	pageRes, err := query.Paginate(depositStore, req.Pagination, func(key []byte, value []byte) error {
		var deposit types.PrivateDeposit
		if err := k.cdc.Unmarshal(value, &deposit); err != nil {
			return err
		}
		deposits = append(deposits, deposit)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to paginate deposits: %v", err))
	}

	return &types.QueryDepositsResponse{
		Deposits:   deposits,
		Pagination: pageRes,
	}, nil
}

// AllDeposits returns deposits across all denominations with pagination.
func (k Keeper) AllDeposits(goCtx context.Context, req *types.QueryAllDepositsRequest) (*types.QueryAllDepositsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	store := ctx.KVStore(k.storeKey)
	depositStore := prefix.NewStore(store, types.DepositKeyPrefix)

	var deposits []types.PrivateDeposit
	pageRes, err := query.Paginate(depositStore, req.Pagination, func(key []byte, value []byte) error {
		var deposit types.PrivateDeposit
		if err := k.cdc.Unmarshal(value, &deposit); err != nil {
			return err
		}
		deposits = append(deposits, deposit)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to paginate deposits: %v", err))
	}

	return &types.QueryAllDepositsResponse{
		Deposits:   deposits,
		Pagination: pageRes,
	}, nil
}

// NextDepositIndex returns the next available deposit index for a denomination.
func (k Keeper) NextDepositIndex(goCtx context.Context, req *types.QueryNextDepositIndexRequest) (*types.QueryNextDepositIndexResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denomination cannot be empty")
	}

	nextIndex, err := k.GetNextDepositIndex(goCtx, req.Denom)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get next deposit index: %v", err))
	}

	return &types.QueryNextDepositIndexResponse{
		NextIndex:     nextIndex,
		TotalDeposits: nextIndex, // nextIndex equals total count since we start from 0
	}, nil
}

// IsNullifierUsed checks if a nullifier has been used (query handler).
func (k Keeper) IsNullifierUsed(goCtx context.Context, req *types.QueryIsNullifierUsedRequest) (*types.QueryIsNullifierUsedResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Nullifier == "" {
		return nil, status.Error(codes.InvalidArgument, "nullifier cannot be empty")
	}

	// Decode hex-encoded nullifier
	nullifierBytes, err := hex.DecodeString(req.Nullifier)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid nullifier hex encoding: %v", err))
	}

	used, err := k.CheckNullifierUsed(goCtx, nullifierBytes)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check nullifier: %v", err))
	}

	response := &types.QueryIsNullifierUsedResponse{
		Used: used,
	}

	// If used, get additional metadata
	if used {
		usedNullifier, err := k.GetNullifier(goCtx, nullifierBytes)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get nullifier metadata: %v", err))
		}
		if usedNullifier != nil {
			response.SpentAtHeight = usedNullifier.SpentAtHeight
			response.SpentTxHash = usedNullifier.SpentTxHash
		}
	}

	return response, nil
}

// MerkleRoot returns the current Merkle tree root for a denomination (Phase 2).
func (k Keeper) MerkleRoot(goCtx context.Context, req *types.QueryMerkleRootRequest) (*types.QueryMerkleRootResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denomination cannot be empty")
	}

	params, err := k.GetParams(goCtx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get params")
	}

	if params.Phase != "phase2" {
		return nil, status.Error(codes.FailedPrecondition, "merkle tree queries only available in Phase 2")
	}

	// TODO: Implement Merkle tree root retrieval for Phase 2
	// For now, return empty response
	return &types.QueryMerkleRootResponse{
		Root:      []byte{},
		Depth:     params.MerkleTreeDepth,
		LeafCount: 0,
	}, status.Error(codes.Unimplemented, "merkle tree not implemented yet (Phase 2)")
}

// MerklePath returns the Merkle path for a specific leaf (Phase 2).
func (k Keeper) MerklePath(goCtx context.Context, req *types.QueryMerklePathRequest) (*types.QueryMerklePathResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denomination cannot be empty")
	}

	params, err := k.GetParams(goCtx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get params")
	}

	if params.Phase != "phase2" {
		return nil, status.Error(codes.FailedPrecondition, "merkle path queries only available in Phase 2")
	}

	// TODO: Implement Merkle path generation for Phase 2
	return nil, status.Error(codes.Unimplemented, "merkle path not implemented yet (Phase 2)")
}

// DepositsByRange returns deposits within a specific index range.
func (k Keeper) DepositsByRange(goCtx context.Context, req *types.QueryDepositsByRangeRequest) (*types.QueryDepositsByRangeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denomination cannot be empty")
	}

	if req.StartIndex >= req.EndIndex {
		return nil, status.Error(codes.InvalidArgument, "start_index must be less than end_index")
	}

	// Cap the range to prevent abuse (max 1000 deposits per query)
	const maxRangeSize = 1000
	rangeSize := req.EndIndex - req.StartIndex
	actualEndIndex := req.EndIndex

	if rangeSize > maxRangeSize {
		actualEndIndex = req.StartIndex + maxRangeSize
	}

	var deposits []types.PrivateDeposit

	// Iterate through the range and fetch deposits
	for i := req.StartIndex; i < actualEndIndex; i++ {
		deposit, err := k.GetDeposit(goCtx, req.Denom, i)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get deposit %d: %v", i, err))
		}

		// If deposit doesn't exist, we've reached the end
		if deposit == nil {
			break
		}

		deposits = append(deposits, *deposit)
	}

	return &types.QueryDepositsByRangeResponse{
		Deposits:   deposits,
		StartIndex: req.StartIndex,
		EndIndex:   req.StartIndex + uint64(len(deposits)),
	}, nil
}

// Stats returns statistics about the privacy pool.
func (k Keeper) Stats(goCtx context.Context, req *types.QueryStatsRequest) (*types.QueryStatsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params, err := k.GetParams(goCtx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get params")
	}

	// Collect statistics per denomination
	denomStats := make([]types.DenomStats, 0)
	totalDeposits := uint64(0)
	totalSpent := uint64(0)

	for _, denom := range params.AllowedDenoms {
		nextIndex, err := k.GetNextDepositIndex(goCtx, denom)
		if err != nil {
			// Skip denoms that have no deposits yet
			continue
		}

		if nextIndex == 0 {
			continue
		}

		// Count spent deposits by iterating through nullifiers
		// This is expensive - in production, we'd maintain counters
		spentCount := uint64(0)
		store := ctx.KVStore(k.storeKey)
		nullifierStore := prefix.NewStore(store, types.NullifierKeyPrefix)
		iterator := nullifierStore.Iterator(nil, nil)
		defer iterator.Close()

		for ; iterator.Valid(); iterator.Next() {
			var usedNullifier types.UsedNullifier
			if err := k.cdc.Unmarshal(iterator.Value(), &usedNullifier); err != nil {
				continue
			}
			if usedNullifier.Denom == denom {
				spentCount++
			}
		}

		activeCount := nextIndex - spentCount

		denomStats = append(denomStats, types.DenomStats{
			Denom:            denom,
			TotalDeposits:    nextIndex,
			ActiveDeposits:   activeCount,
			TotalValueLocked: "0", // Cannot determine from commitments
			MerkleRoot:       nil, // Phase 2 only
		})

		totalDeposits += nextIndex
		totalSpent += spentCount
	}

	return &types.QueryStatsResponse{
		TotalDeposits:  totalDeposits,
		TotalSpent:     totalSpent,
		ActiveDeposits: totalDeposits - totalSpent,
		DenomStats:     denomStats,
		Phase:          params.Phase,
	}, nil
}

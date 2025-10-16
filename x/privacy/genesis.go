package privacy

import (
	"context"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/keeper"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx context.Context, k keeper.Keeper, data types.GenesisState) error {
	// Set parameters
	if err := k.SetParams(ctx, data.Params); err != nil {
		return err
	}

	// Initialize deposits
	for _, deposit := range data.Deposits {
		if err := k.SetDeposit(ctx, &deposit); err != nil {
			return err
		}
	}

	// Initialize next deposit indices
	for denom, index := range data.NextDepositIndices {
		if err := k.SetNextDepositIndex(ctx, denom, index); err != nil {
			return err
		}
	}

	// Initialize used nullifiers
	for _, nullifier := range data.UsedNullifiers {
		if err := k.SetNullifierUsed(ctx, &nullifier); err != nil {
			return err
		}
	}

	// TODO: Initialize Merkle trees for Phase 2
	// This will be implemented when we add Phase 2 functionality

	return nil
}

// ExportGenesis returns the module's exported genesis.
func ExportGenesis(ctx context.Context, k keeper.Keeper) (*types.GenesisState, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: Export all deposits, nullifiers, and Merkle trees
	// For now, return minimal genesis state
	return &types.GenesisState{
		Params:             params,
		Deposits:           []types.PrivateDeposit{},
		NextDepositIndices: make(map[string]uint64),
		UsedNullifiers:     []types.UsedNullifier{},
		MerkleTrees:        []types.DenomMerkleTree{},
	}, nil
}

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *types.GenesisState {
	return &types.GenesisState{
		Params:             types.DefaultParams(),
		Deposits:           []types.PrivateDeposit{},
		NextDepositIndices: make(map[string]uint64),
		UsedNullifiers:     []types.UsedNullifier{},
		MerkleTrees:        []types.DenomMerkleTree{},
	}
}

// ValidateGenesis validates the genesis state
func ValidateGenesis(data *types.GenesisState) error {
	if err := data.Params.Validate(); err != nil {
		return err
	}

	// Validate deposits
	seenIndices := make(map[string]map[uint64]bool)
	for _, deposit := range data.Deposits {
		if deposit.Denom == "" {
			return types.ErrInvalidDenom
		}
		if _, exists := seenIndices[deposit.Denom]; !exists {
			seenIndices[deposit.Denom] = make(map[uint64]bool)
		}
		if seenIndices[deposit.Denom][deposit.Index] {
			return types.ErrDuplicateDepositIndex
		}
		seenIndices[deposit.Denom][deposit.Index] = true
	}

	// Validate nullifiers
	seenNullifiers := make(map[string]bool)
	for _, nullifier := range data.UsedNullifiers {
		if len(nullifier.Nullifier) == 0 {
			return types.ErrInvalidNullifier
		}
		key := string(nullifier.Nullifier)
		if seenNullifiers[key] {
			return types.ErrDuplicateNullifier
		}
		seenNullifiers[key] = true
	}

	return nil
}

package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

type Keeper struct {
	cdc       codec.BinaryCodec
	storeKey  storetypes.StoreKey
	authority string

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	authority string,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) *Keeper {
	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		authority:     authority,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// SetParams sets the module parameters.
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	store := k.storeService(ctx)
	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}
	store.Set(types.ParamsKey, bz)
	return nil
}

// GetParams gets the module parameters.
func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	store := k.storeService(ctx)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return types.Params{}, nil
	}

	var params types.Params
	if err := k.cdc.Unmarshal(bz, &params); err != nil {
		return types.Params{}, err
	}
	return params, nil
}

// storeService returns a KVStore from the context
func (k Keeper) storeService(ctx context.Context) storetypes.KVStore {
	return sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
}

// GetNextDepositIndex returns the next available deposit index for a denomination
func (k Keeper) GetNextDepositIndex(ctx context.Context, denom string) (uint64, error) {
	store := k.storeService(ctx)
	key := types.NextDepositIndexKey(denom)
	bz := store.Get(key)
	if bz == nil {
		return 0, nil
	}
	return sdk.BigEndianToUint64(bz), nil
}

// SetNextDepositIndex sets the next deposit index for a denomination
func (k Keeper) SetNextDepositIndex(ctx context.Context, denom string, index uint64) error {
	store := k.storeService(ctx)
	key := types.NextDepositIndexKey(denom)
	bz := sdk.Uint64ToBigEndian(index)
	store.Set(key, bz)
	return nil
}

// IncrementDepositIndex increments and returns the next deposit index for a denomination
func (k Keeper) IncrementDepositIndex(ctx context.Context, denom string) (uint64, error) {
	currentIndex, err := k.GetNextDepositIndex(ctx, denom)
	if err != nil {
		return 0, err
	}
	nextIndex := currentIndex + 1
	if err := k.SetNextDepositIndex(ctx, denom, nextIndex); err != nil {
		return 0, err
	}
	return currentIndex, nil
}

// SetDeposit stores a private deposit
func (k Keeper) SetDeposit(ctx context.Context, deposit *types.PrivateDeposit) error {
	store := k.storeService(ctx)
	key := types.DepositKey(deposit.Denom, deposit.Index)
	bz, err := k.cdc.Marshal(deposit)
	if err != nil {
		return err
	}
	store.Set(key, bz)
	return nil
}

// GetDeposit retrieves a private deposit by denomination and index
func (k Keeper) GetDeposit(ctx context.Context, denom string, index uint64) (*types.PrivateDeposit, error) {
	store := k.storeService(ctx)
	key := types.DepositKey(denom, index)
	bz := store.Get(key)
	if bz == nil {
		return nil, nil
	}

	var deposit types.PrivateDeposit
	if err := k.cdc.Unmarshal(bz, &deposit); err != nil {
		return nil, err
	}
	return &deposit, nil
}

// IsNullifierUsed checks if a nullifier has been used
func (k Keeper) IsNullifierUsed(ctx context.Context, nullifier []byte) (bool, error) {
	store := k.storeService(ctx)
	key := types.NullifierKey(nullifier)
	bz := store.Get(key)
	return bz != nil, nil
}

// SetNullifierUsed marks a nullifier as used
func (k Keeper) SetNullifierUsed(ctx context.Context, nullifier *types.UsedNullifier) error {
	store := k.storeService(ctx)
	key := types.NullifierKey(nullifier.Nullifier)
	bz, err := k.cdc.Marshal(nullifier)
	if err != nil {
		return err
	}
	store.Set(key, bz)
	return nil
}

// GetNullifier retrieves nullifier metadata
func (k Keeper) GetNullifier(ctx context.Context, nullifier []byte) (*types.UsedNullifier, error) {
	store := k.storeService(ctx)
	key := types.NullifierKey(nullifier)
	bz := store.Get(key)
	if bz == nil {
		return nil, nil
	}

	var usedNullifier types.UsedNullifier
	if err := k.cdc.Unmarshal(bz, &usedNullifier); err != nil {
		return nil, err
	}
	return &usedNullifier, nil
}
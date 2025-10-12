package post

import (
	"context"

	"github.com/Hikari-Chain/hikari-chain/x/dynamicfee/types"
)

type DynamicfeeKeeper interface {
	GetState(ctx context.Context) (types.State, error)
	GetParams(ctx context.Context) (types.Params, error)
	GetMaxBlockGas(ctx context.Context, params types.Params) uint64
	SetState(ctx context.Context, state types.State) error
	GetEnabledHeight(ctx context.Context) (int64, error)
}

package photon_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Hikari-Chain/hikari-chain/x/photon"
	"github.com/Hikari-Chain/hikari-chain/x/photon/testutil"
	"github.com/Hikari-Chain/hikari-chain/x/photon/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}
	k, _, ctx := testutil.SetupPhotonKeeper(t)

	photon.InitGenesis(ctx, *k, genesisState)
	got := photon.ExportGenesis(ctx, *k)

	require.NotNil(t, got)
	require.Equal(t, genesisState, *got)
}

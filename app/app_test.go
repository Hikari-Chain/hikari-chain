package atomone_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	atomone "github.com/Hikari-Chain/hikari-chain/app"
	atomonehelpers "github.com/Hikari-Chain/hikari-chain/app/helpers"
	govtypes "github.com/Hikari-Chain/hikari-chain/x/gov/types"
)

func TestAtomOneApp_BlockedModuleAccountAddrs(t *testing.T) {
	app := atomone.NewAtomOneApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		atomone.EmptyAppOptions{},
	)

	moduleAccountAddresses := app.ModuleAccountAddrs()
	blockedAddrs := app.BlockedModuleAccountAddrs(moduleAccountAddresses)

	require.NotContains(t, blockedAddrs, authtypes.NewModuleAddress(govtypes.ModuleName).String())
}

func TestAtomOneApp_Export(t *testing.T) {
	app := atomonehelpers.Setup(t)
	_, err := app.ExportAppStateAndValidators(true, []string{}, []string{})
	require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
}

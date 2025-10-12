package v3

import (
	storetypes "cosmossdk.io/store/types"

	"github.com/Hikari-Chain/hikari-chain/app/upgrades"
	dynamicfeetypes "github.com/Hikari-Chain/hikari-chain/x/dynamicfee/types"
)

const (
	UpgradeName = "v3"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: storetypes.StoreUpgrades{
		Added: []string{
			// new module added in v3
			dynamicfeetypes.ModuleName,
		},
	},
}

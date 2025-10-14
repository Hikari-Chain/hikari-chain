package v4

import (
	store "cosmossdk.io/store/types"

	"github.com/Hikari-Chain/hikari-chain/app/upgrades"
	coredaostypes "github.com/Hikari-Chain/hikari-chain/x/coredaos/types"
)

const (
	UpgradeName = "v4"

	capabilityStoreKey = "capability"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			// new module added in v4
			coredaostypes.ModuleName,
		},
		Deleted: []string{
			capabilityStoreKey,
		},
	},
}

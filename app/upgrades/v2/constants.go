package v2

import (
	store "cosmossdk.io/store/types"

	"github.com/Hikari-Chain/hikari-chain/app/upgrades"
	photontypes "github.com/Hikari-Chain/hikari-chain/x/photon/types"
)

const (
	UpgradeName = "v2"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			// new module added in v2
			photontypes.ModuleName,
		},
		Deleted: []string{
			"crisis",
		},
	},
}

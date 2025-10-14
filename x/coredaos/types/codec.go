package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgAnnotateProposal{}, "hikari/v1/MsgAnnotateProposal")
	legacy.RegisterAminoMsg(cdc, &MsgEndorseProposal{}, "hikari/v1/MsgEndorseProposal")
	legacy.RegisterAminoMsg(cdc, &MsgExtendVotingPeriod{}, "hikari/v1/MsgExtendVotingPeriod")
	legacy.RegisterAminoMsg(cdc, &MsgVetoProposal{}, "hikari/v1/MsgVetoProposal")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "hikari/x/coredaos/v1/MsgUpdateParams")
	cdc.RegisterConcrete(&Params{}, "hikari/coredaos/v1/Params", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgAnnotateProposal{}, &MsgEndorseProposal{}, &MsgExtendVotingPeriod{}, &MsgVetoProposal{}, &MsgUpdateParams{},
	)
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

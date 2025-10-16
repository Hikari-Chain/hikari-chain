package types

// Privacy module event types
const (
	EventTypeShield          = "shield"
	EventTypePrivateTransfer = "private_transfer"
	EventTypeUnshield        = "unshield"
	EventTypeUpdateParams    = "update_params"

	AttributeKeySender       = "sender"
	AttributeKeyRecipient    = "recipient"
	AttributeKeyDenom        = "denom"
	AttributeKeyAmount       = "amount"
	AttributeKeyDepositIndex = "deposit_index"
	AttributeKeyInputCount   = "input_count"
	AttributeKeyOutputCount  = "output_count"
	AttributeKeyBlockHeight  = "block_height"
	AttributeKeyAuthority    = "authority"
)

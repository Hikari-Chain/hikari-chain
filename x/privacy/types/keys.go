package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "privacy"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_privacy"
)

// Store key prefixes
var (
	// ParamsKey is the prefix for params key
	ParamsKey = []byte{0x01}

	// DepositKeyPrefix is the prefix for storing private deposits
	// Key: DepositKeyPrefix | denom | index (8 bytes big-endian)
	DepositKeyPrefix = []byte{0x02}

	// NextDepositIndexKeyPrefix is the prefix for storing next deposit index per denom
	// Key: NextDepositIndexKeyPrefix | denom
	NextDepositIndexKeyPrefix = []byte{0x03}

	// NullifierKeyPrefix is the prefix for storing used nullifiers
	// Key: NullifierKeyPrefix | nullifier (32 bytes)
	NullifierKeyPrefix = []byte{0x04}

	// MerkleRootKeyPrefix is the prefix for storing Merkle tree roots per denom (Phase 2)
	// Key: MerkleRootKeyPrefix | denom
	MerkleRootKeyPrefix = []byte{0x05}

	// MerkleNodeKeyPrefix is the prefix for storing Merkle tree nodes (Phase 2)
	// Key: MerkleNodeKeyPrefix | denom | level (1 byte) | index (4 bytes)
	MerkleNodeKeyPrefix = []byte{0x06}
)

// DepositKey returns the store key for a specific deposit
func DepositKey(denom string, index uint64) []byte {
	denomBytes := []byte(denom)
	indexBytes := sdk.Uint64ToBigEndian(index)
	key := make([]byte, len(DepositKeyPrefix)+len(denomBytes)+1+len(indexBytes))
	copy(key, DepositKeyPrefix)
	copy(key[len(DepositKeyPrefix):], denomBytes)
	key[len(DepositKeyPrefix)+len(denomBytes)] = 0x00 // separator
	copy(key[len(DepositKeyPrefix)+len(denomBytes)+1:], indexBytes)
	return key
}

// NextDepositIndexKey returns the store key for next deposit index
func NextDepositIndexKey(denom string) []byte {
	denomBytes := []byte(denom)
	key := make([]byte, len(NextDepositIndexKeyPrefix)+len(denomBytes))
	copy(key, NextDepositIndexKeyPrefix)
	copy(key[len(NextDepositIndexKeyPrefix):], denomBytes)
	return key
}

// NullifierKey returns the store key for a nullifier
func NullifierKey(nullifier []byte) []byte {
	key := make([]byte, len(NullifierKeyPrefix)+len(nullifier))
	copy(key, NullifierKeyPrefix)
	copy(key[len(NullifierKeyPrefix):], nullifier)
	return key
}

// MerkleRootKey returns the store key for a Merkle tree root
func MerkleRootKey(denom string) []byte {
	denomBytes := []byte(denom)
	key := make([]byte, len(MerkleRootKeyPrefix)+len(denomBytes))
	copy(key, MerkleRootKeyPrefix)
	copy(key[len(MerkleRootKeyPrefix):], denomBytes)
	return key
}

// MerkleNodeKey returns the store key for a Merkle tree node
func MerkleNodeKey(denom string, level uint32, index uint32) []byte {
	denomBytes := []byte(denom)
	levelBytes := []byte{byte(level)}
	indexBytes := sdk.Uint64ToBigEndian(uint64(index))[:4] // only use first 4 bytes
	key := make([]byte, len(MerkleNodeKeyPrefix)+len(denomBytes)+1+len(levelBytes)+len(indexBytes))
	copy(key, MerkleNodeKeyPrefix)
	copy(key[len(MerkleNodeKeyPrefix):], denomBytes)
	key[len(MerkleNodeKeyPrefix)+len(denomBytes)] = 0x00 // separator
	copy(key[len(MerkleNodeKeyPrefix)+len(denomBytes)+1:], levelBytes)
	copy(key[len(MerkleNodeKeyPrefix)+len(denomBytes)+1+len(levelBytes):], indexBytes)
	return key
}
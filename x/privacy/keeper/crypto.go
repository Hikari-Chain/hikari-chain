package keeper

import (
	"fmt"
	"math/big"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/crypto"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

// VerifyNullifierSignature verifies a signature for a private transfer input
// This proves ownership of the one-time private key without revealing it
func (k Keeper) VerifyNullifierSignature(
	deposit *types.PrivateDeposit,
	nullifier []byte,
	signature []byte,
) error {
	if deposit == nil {
		return fmt.Errorf("deposit is nil")
	}
	if len(nullifier) == 0 {
		return fmt.Errorf("nullifier is empty")
	}
	if len(signature) != 64 {
		return fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	// Convert one-time address to crypto.ECPoint
	oneTimeAddr := convertToECPoint(&deposit.OneTimeAddress.Address)
	if oneTimeAddr == nil {
		return fmt.Errorf("invalid one-time address")
	}

	// Convert nullifier bytes to crypto.Nullifier
	cryptoNullifier, err := crypto.NullifierFromBytes(nullifier)
	if err != nil {
		return fmt.Errorf("failed to parse nullifier: %w", err)
	}

	// Verify the signature
	if !crypto.VerifyNullifierSignature(oneTimeAddr, cryptoNullifier, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// VerifyUnshieldSignature verifies a signature for an unshield request
// Message format: nullifier || recipient_address || amount
func (k Keeper) VerifyUnshieldSignature(
	deposit *types.PrivateDeposit,
	nullifier []byte,
	recipientAddr string,
	amount string,
	signature []byte,
) error {
	if deposit == nil {
		return fmt.Errorf("deposit is nil")
	}
	if len(nullifier) == 0 {
		return fmt.Errorf("nullifier is empty")
	}
	if recipientAddr == "" {
		return fmt.Errorf("recipient address is empty")
	}
	if amount == "" {
		return fmt.Errorf("amount is empty")
	}
	if len(signature) != 64 {
		return fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	// Convert one-time address to crypto.ECPoint
	oneTimeAddr := convertToECPoint(&deposit.OneTimeAddress.Address)
	if oneTimeAddr == nil {
		return fmt.Errorf("invalid one-time address")
	}

	// Convert nullifier bytes to crypto.Nullifier
	cryptoNullifier, err := crypto.NullifierFromBytes(nullifier)
	if err != nil {
		return fmt.Errorf("failed to parse nullifier: %w", err)
	}

	// Verify the signature
	if !crypto.VerifyUnshieldSignature(oneTimeAddr, cryptoNullifier, recipientAddr, amount, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// convertToECPoint converts a types.ECPoint to crypto.ECPoint
func convertToECPoint(point *types.ECPoint) *crypto.ECPoint {
	if point == nil || len(point.X) != 32 || len(point.Y) != 32 {
		return nil
	}

	x := new(big.Int).SetBytes(point.X)
	y := new(big.Int).SetBytes(point.Y)

	return crypto.NewECPoint(x, y)
}

// ValidateECPointOnCurve validates that an EC point is on the secp256k1 curve
func (k Keeper) ValidateECPointOnCurve(point *types.ECPoint) error {
	cryptoPoint := convertToECPoint(point)
	if cryptoPoint == nil {
		return fmt.Errorf("failed to convert point")
	}

	if !cryptoPoint.IsOnCurve() {
		return fmt.Errorf("point is not on secp256k1 curve")
	}

	if cryptoPoint.IsIdentity() {
		return fmt.Errorf("point is identity element")
	}

	return nil
}

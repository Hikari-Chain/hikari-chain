package crypto

import (
	"fmt"
	"math/big"
)

// Nullifier represents a key image used to prevent double-spends
type Nullifier struct {
	Point *ECPoint
}

// GenerateNullifier generates a nullifier (key image) for Phase 1
// I = oneTimePrivKey * Hp(oneTimeAddr)
func GenerateNullifier(oneTimePrivKey *big.Int, oneTimeAddr *ECPoint) (*Nullifier, error) {
	if oneTimePrivKey == nil {
		return nil, fmt.Errorf("one-time private key is nil")
	}
	if oneTimeAddr == nil {
		return nil, fmt.Errorf("one-time address is nil")
	}

	// Hash one-time address to point
	hp := HashToPoint(oneTimeAddr.Bytes())

	// Multiply by private key
	nullifierPoint := ScalarMult(oneTimePrivKey, hp)
	if nullifierPoint == nil {
		return nil, fmt.Errorf("failed to compute nullifier point")
	}

	return &Nullifier{Point: nullifierPoint}, nil
}

// GenerateNullifierZK generates a nullifier for Phase 2 (ZK-SNARK)
// N = Hash(secret || commitment)
func GenerateNullifierZK(secret, commitment []byte) ([]byte, error) {
	if len(secret) == 0 || len(commitment) == 0 {
		return nil, fmt.Errorf("secret or commitment is empty")
	}

	// Concatenate and hash
	data := append(secret, commitment...)
	return Hash256(data), nil
}

// Bytes returns the compressed byte representation of the nullifier
func (n *Nullifier) Bytes() []byte {
	if n == nil || n.Point == nil {
		return nil
	}
	return n.Point.Compressed()
}

// Verify verifies that the nullifier is valid
func (n *Nullifier) Verify() error {
	if n == nil || n.Point == nil {
		return fmt.Errorf("nullifier is nil")
	}

	if !n.Point.IsOnCurve() {
		return fmt.Errorf("nullifier point is not on curve")
	}

	if n.Point.IsIdentity() {
		return fmt.Errorf("nullifier point is identity element")
	}

	return nil
}

// Equal checks if two nullifiers are equal
func (n *Nullifier) Equal(other *Nullifier) bool {
	if n == nil && other == nil {
		return true
	}
	if n == nil || other == nil {
		return false
	}
	return n.Point.Equal(other.Point)
}

// NullifierFromBytes creates a nullifier from compressed bytes
func NullifierFromBytes(data []byte) (*Nullifier, error) {
	if len(data) != 33 {
		return nil, fmt.Errorf("invalid nullifier size: expected 33 bytes, got %d", len(data))
	}

	point := DecompressPoint(data)
	if point == nil {
		return nil, fmt.Errorf("failed to decompress nullifier point")
	}

	nullifier := &Nullifier{Point: point}
	if err := nullifier.Verify(); err != nil {
		return nil, fmt.Errorf("invalid nullifier: %w", err)
	}

	return nullifier, nil
}

// VerifyNullifierLinkage verifies that a nullifier was generated from the correct one-time address
// This proves knowledge of the one-time private key without revealing it
func VerifyNullifierLinkage(
	nullifier *Nullifier,
	oneTimeAddr *ECPoint,
	oneTimePrivKey *big.Int,
) bool {
	// Regenerate the nullifier and check if it matches
	expectedNullifier, err := GenerateNullifier(oneTimePrivKey, oneTimeAddr)
	if err != nil {
		return false
	}

	return nullifier.Equal(expectedNullifier)
}

// ComputeNullifierHash computes a 32-byte hash of the nullifier for storage
// This is used as the key in the nullifier set to prevent double-spends
func ComputeNullifierHash(nullifier *Nullifier) []byte {
	if nullifier == nil {
		return nil
	}
	return Hash256(nullifier.Bytes())
}

// ComputeNullifierHashFromBytes computes nullifier hash from raw bytes
func ComputeNullifierHashFromBytes(nullifierBytes []byte) []byte {
	return Hash256(nullifierBytes)
}
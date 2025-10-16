package crypto

import (
	"crypto/sha256"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
)

// Curve returns the secp256k1 elliptic curve
func Curve() *btcec.KoblitzCurve {
	return btcec.S256()
}

// G returns the generator point of secp256k1
func G() (*big.Int, *big.Int) {
	curve := Curve()
	return curve.Gx, curve.Gy
}

// ECPoint represents an elliptic curve point
type ECPoint struct {
	X *big.Int
	Y *big.Int
}

// NewECPoint creates a new ECPoint
func NewECPoint(x, y *big.Int) *ECPoint {
	return &ECPoint{X: x, Y: y}
}

// Bytes returns the uncompressed byte representation (65 bytes: 0x04 || X || Y)
func (p *ECPoint) Bytes() []byte {
	if p == nil || p.X == nil || p.Y == nil {
		return nil
	}
	b := make([]byte, 65)
	b[0] = 0x04
	copy(b[1:33], p.X.Bytes())
	copy(b[33:65], p.Y.Bytes())
	return b
}

// Compressed returns the compressed byte representation (33 bytes: 0x02/0x03 || X)
func (p *ECPoint) Compressed() []byte {
	if p == nil || p.X == nil || p.Y == nil {
		return nil
	}
	b := make([]byte, 33)
	// 0x02 if Y is even, 0x03 if Y is odd
	if p.Y.Bit(0) == 0 {
		b[0] = 0x02
	} else {
		b[0] = 0x03
	}
	xBytes := p.X.Bytes()
	copy(b[33-len(xBytes):], xBytes)
	return b
}

// IsOnCurve checks if the point is on the secp256k1 curve
func (p *ECPoint) IsOnCurve() bool {
	if p == nil || p.X == nil || p.Y == nil {
		return false
	}
	return Curve().IsOnCurve(p.X, p.Y)
}

// IsIdentity checks if the point is the identity element (point at infinity)
func (p *ECPoint) IsIdentity() bool {
	if p == nil {
		return true
	}
	return p.X == nil || p.Y == nil
}

// Equal checks if two points are equal
func (p *ECPoint) Equal(other *ECPoint) bool {
	if p == nil && other == nil {
		return true
	}
	if p == nil || other == nil {
		return false
	}
	return p.X.Cmp(other.X) == 0 && p.Y.Cmp(other.Y) == 0
}

// ScalarMult performs scalar multiplication: k * P
func ScalarMult(k *big.Int, p *ECPoint) *ECPoint {
	if p == nil || k == nil {
		return nil
	}
	curve := Curve()
	x, y := curve.ScalarMult(p.X, p.Y, k.Bytes())
	return NewECPoint(x, y)
}

// ScalarBaseMult performs scalar multiplication with the generator: k * G
func ScalarBaseMult(k *big.Int) *ECPoint {
	if k == nil {
		return nil
	}
	curve := Curve()
	x, y := curve.ScalarBaseMult(k.Bytes())
	return NewECPoint(x, y)
}

// PointAdd adds two points: P + Q
func PointAdd(p, q *ECPoint) *ECPoint {
	if p == nil || q == nil {
		return nil
	}
	curve := Curve()
	x, y := curve.Add(p.X, p.Y, q.X, q.Y)
	return NewECPoint(x, y)
}

// DecompressPoint decompresses a compressed point (33 bytes)
func DecompressPoint(compressed []byte) *ECPoint {
	if len(compressed) != 33 {
		return nil
	}

	curve := Curve()
	x := new(big.Int).SetBytes(compressed[1:33])

	// Calculate y² = x³ + 7 (secp256k1 equation)
	y2 := new(big.Int).Mul(x, x)
	y2.Mul(y2, x)
	y2.Add(y2, big.NewInt(7))
	y2.Mod(y2, curve.P)

	// Calculate y = sqrt(y²) mod p
	y := new(big.Int).ModSqrt(y2, curve.P)
	if y == nil {
		return nil
	}

	// Choose correct y based on parity
	if compressed[0] == 0x02 {
		// Y should be even
		if y.Bit(0) == 1 {
			y.Sub(curve.P, y)
		}
	} else if compressed[0] == 0x03 {
		// Y should be odd
		if y.Bit(0) == 0 {
			y.Sub(curve.P, y)
		}
	} else {
		return nil
	}

	return NewECPoint(x, y)
}

// Hash256 performs SHA-256 hash
func Hash256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// HashToScalar hashes data and reduces it to a scalar modulo curve order
func HashToScalar(data []byte) *big.Int {
	hash := Hash256(data)
	scalar := new(big.Int).SetBytes(hash)
	scalar.Mod(scalar, Curve().N)
	return scalar
}

// HashToPoint hashes data to a point on the curve
// Uses try-and-increment method
func HashToPoint(data []byte) *ECPoint {
	curve := Curve()
	hash := Hash256(data)
	x := new(big.Int).SetBytes(hash)

	// Try to find a valid point
	for i := 0; i < 256; i++ {
		// Calculate y² = x³ + 7
		y2 := new(big.Int).Mul(x, x)
		y2.Mul(y2, x)
		y2.Add(y2, big.NewInt(7))
		y2.Mod(y2, curve.P)

		// Try to compute square root
		y := new(big.Int).ModSqrt(y2, curve.P)
		if y != nil {
			// Found valid point
			return NewECPoint(x, y)
		}

		// Try next x
		x.Add(x, big.NewInt(1))
		x.Mod(x, curve.P)
	}

	// This should never happen
	panic("failed to hash to point after 256 attempts")
}

// DeriveH derives the second generator point H for Pedersen commitments
// Uses nothing-up-my-sleeve construction
func DeriveH() *ECPoint {
	// Use a constant string to derive H deterministically
	data := []byte("Hikari Chain Privacy Module - H Generator Point")
	return HashToPoint(data)
}

// H is the cached second generator point
var cachedH *ECPoint

// H returns the second generator point for Pedersen commitments
func H() *ECPoint {
	if cachedH == nil {
		cachedH = DeriveH()
	}
	return cachedH
}
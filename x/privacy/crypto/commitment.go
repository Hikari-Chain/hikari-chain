package crypto

import (
	"fmt"
	"math/big"
)

// Commitment represents a Pedersen commitment
type Commitment struct {
	Point *ECPoint
}

// CreateCommitment creates a Pedersen commitment: C = amount*H + blinding*G
func CreateCommitment(amount uint64, blinding *big.Int) (*Commitment, error) {
	if blinding == nil {
		return nil, fmt.Errorf("blinding factor is nil")
	}

	// amount*H
	amountBig := new(big.Int).SetUint64(amount)
	amountH := ScalarMult(amountBig, H())
	if amountH == nil {
		return nil, fmt.Errorf("failed to compute amount*H")
	}

	// blinding*G
	gx, gy := G()
	blindingG := ScalarMult(blinding, NewECPoint(gx, gy))
	if blindingG == nil {
		return nil, fmt.Errorf("failed to compute blinding*G")
	}

	// Add the points
	commitmentPoint := PointAdd(amountH, blindingG)
	if commitmentPoint == nil {
		return nil, fmt.Errorf("failed to add commitment points")
	}

	return &Commitment{Point: commitmentPoint}, nil
}

// GenerateBlinding generates a random blinding factor
func GenerateBlinding() (*big.Int, error) {
	return GenerateRandomScalar()
}

// Verify verifies that the commitment is valid (point is on curve)
func (c *Commitment) Verify() error {
	if c == nil || c.Point == nil {
		return fmt.Errorf("commitment is nil")
	}

	if !c.Point.IsOnCurve() {
		return fmt.Errorf("commitment point is not on curve")
	}

	if c.Point.IsIdentity() {
		return fmt.Errorf("commitment point is identity element")
	}

	return nil
}

// Bytes returns the compressed byte representation of the commitment
func (c *Commitment) Bytes() []byte {
	if c == nil || c.Point == nil {
		return nil
	}
	return c.Point.Compressed()
}

// Equal checks if two commitments are equal
func (c *Commitment) Equal(other *Commitment) bool {
	if c == nil && other == nil {
		return true
	}
	if c == nil || other == nil {
		return false
	}
	return c.Point.Equal(other.Point)
}

// Add adds two commitments: C1 + C2
func (c *Commitment) Add(other *Commitment) *Commitment {
	if c == nil || other == nil {
		return nil
	}
	sum := PointAdd(c.Point, other.Point)
	return &Commitment{Point: sum}
}

// Sub subtracts two commitments: C1 - C2
func (c *Commitment) Sub(other *Commitment) *Commitment {
	if c == nil || other == nil {
		return nil
	}

	// Negate other.Point by negating Y coordinate
	negOther := NewECPoint(
		new(big.Int).Set(other.Point.X),
		new(big.Int).Sub(Curve().P, other.Point.Y),
	)

	diff := PointAdd(c.Point, negOther)
	return &Commitment{Point: diff}
}

// VerifyCommitmentBalance verifies that input commitment equals sum of output commitments
// C_in = C_out1 + C_out2 + ... + C_outN
func VerifyCommitmentBalance(input *Commitment, outputs []*Commitment) bool {
	if input == nil || len(outputs) == 0 {
		return false
	}

	// Sum all output commitments
	sum := outputs[0]
	for i := 1; i < len(outputs); i++ {
		sum = sum.Add(outputs[i])
		if sum == nil {
			return false
		}
	}

	// Check if sum equals input
	return input.Equal(sum)
}

// VerifyCommitmentBalanceWithFee verifies commitment balance with fee
// C_in = C_out1 + C_out2 + ... + C_outN + fee*H
func VerifyCommitmentBalanceWithFee(input *Commitment, outputs []*Commitment, fee uint64) bool {
	if input == nil || len(outputs) == 0 {
		return false
	}

	// Create fee commitment (fee*H with zero blinding)
	feeCommitment, err := CreateCommitment(fee, big.NewInt(0))
	if err != nil {
		return false
	}

	// Sum all output commitments
	sum := outputs[0]
	for i := 1; i < len(outputs); i++ {
		sum = sum.Add(outputs[i])
		if sum == nil {
			return false
		}
	}

	// Add fee commitment
	sum = sum.Add(feeCommitment)
	if sum == nil {
		return false
	}

	// Check if sum equals input
	return input.Equal(sum)
}

// CommitmentFromBytes creates a commitment from compressed bytes
func CommitmentFromBytes(data []byte) (*Commitment, error) {
	if len(data) != 33 {
		return nil, fmt.Errorf("invalid commitment size: expected 33 bytes, got %d", len(data))
	}

	point := DecompressPoint(data)
	if point == nil {
		return nil, fmt.Errorf("failed to decompress commitment point")
	}

	commitment := &Commitment{Point: point}
	if err := commitment.Verify(); err != nil {
		return nil, fmt.Errorf("invalid commitment: %w", err)
	}

	return commitment, nil
}

// CreateZeroCommitment creates a commitment to zero with the given blinding factor
// Useful for change outputs in private transfers
func CreateZeroCommitment(blinding *big.Int) (*Commitment, error) {
	return CreateCommitment(0, blinding)
}

// VerifyBlindingSum verifies that the sum of blinding factors is correct
// This is used client-side to ensure commitment balance
// blinding_in = blinding_out1 + blinding_out2 + ... + blinding_outN (mod n)
func VerifyBlindingSum(inputBlinding *big.Int, outputBlindings []*big.Int) bool {
	if inputBlinding == nil || len(outputBlindings) == 0 {
		return false
	}

	// Sum output blindings
	sum := new(big.Int).Set(outputBlindings[0])
	for i := 1; i < len(outputBlindings); i++ {
		sum.Add(sum, outputBlindings[i])
		sum.Mod(sum, Curve().N)
	}

	// Check if sum equals input blinding
	return sum.Cmp(inputBlinding) == 0
}
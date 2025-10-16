package crypto

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// StealthKeyPair represents a dual-key stealth address key pair
type StealthKeyPair struct {
	ViewPrivateKey  *big.Int
	ViewPublicKey   *ECPoint
	SpendPrivateKey *big.Int
	SpendPublicKey  *ECPoint
}

// GenerateStealthKeyPair generates a new stealth address key pair
func GenerateStealthKeyPair() (*StealthKeyPair, error) {
	// Generate view key pair
	viewPrivKey, err := GenerateRandomScalar()
	if err != nil {
		return nil, fmt.Errorf("failed to generate view private key: %w", err)
	}
	viewPubKey := ScalarBaseMult(viewPrivKey)

	// Generate spend key pair
	spendPrivKey, err := GenerateRandomScalar()
	if err != nil {
		return nil, fmt.Errorf("failed to generate spend private key: %w", err)
	}
	spendPubKey := ScalarBaseMult(spendPrivKey)

	return &StealthKeyPair{
		ViewPrivateKey:  viewPrivKey,
		ViewPublicKey:   viewPubKey,
		SpendPrivateKey: spendPrivKey,
		SpendPublicKey:  spendPubKey,
	}, nil
}

// StealthAddress represents a one-time stealth address
type StealthAddress struct {
	PublicKey   *ECPoint // One-time public key P
	TxPublicKey *ECPoint // Transaction public key R
}

// GenerateStealthAddress generates a one-time stealth address for the recipient
// Returns: (stealth address, shared secret, random scalar r)
func GenerateStealthAddress(recipientViewPubKey, recipientSpendPubKey *ECPoint) (*StealthAddress, []byte, *big.Int, error) {
	// 1. Generate random scalar r
	r, err := GenerateRandomScalar()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate random scalar: %w", err)
	}

	// 2. Compute transaction public key R = r*G
	txPubKey := ScalarBaseMult(r)

	// 3. Compute shared secret
	// sharedSecret = Hash(r * recipientViewPubKey)
	temp := ScalarMult(r, recipientViewPubKey)
	if temp == nil {
		return nil, nil, nil, fmt.Errorf("failed to compute shared secret point")
	}
	sharedSecret := Hash256(temp.Bytes())

	// 4. Derive one-time public key
	// P = Hash(sharedSecret)*G + recipientSpendPubKey
	hs := HashToScalar(sharedSecret)
	hsG := ScalarBaseMult(hs)
	oneTimeAddr := PointAdd(hsG, recipientSpendPubKey)

	if oneTimeAddr == nil {
		return nil, nil, nil, fmt.Errorf("failed to compute one-time address")
	}

	return &StealthAddress{
		PublicKey:   oneTimeAddr,
		TxPublicKey: txPubKey,
	}, sharedSecret, r, nil
}

// CheckIfMine checks if a stealth address belongs to the recipient
// Returns: (isMine, one-time private key if mine)
func CheckIfMine(
	oneTimeAddr, txPubKey *ECPoint,
	myViewPrivKey *big.Int,
	mySpendPubKey *ECPoint,
	mySpendPrivKey *big.Int,
) (bool, *big.Int) {
	// 1. Compute shared secret
	// sharedSecret = Hash(viewPrivKey * txPubKey)
	temp := ScalarMult(myViewPrivKey, txPubKey)
	if temp == nil {
		return false, nil
	}
	sharedSecret := Hash256(temp.Bytes())

	// 2. Derive expected one-time public key
	hs := HashToScalar(sharedSecret)
	hsG := ScalarBaseMult(hs)
	expectedAddr := PointAdd(hsG, mySpendPubKey)

	// 3. Check if it matches
	if expectedAddr.Equal(oneTimeAddr) {
		// This is mine! Compute private key
		// oneTimePrivKey = Hash(sharedSecret) + spendPrivKey (mod n)
		oneTimePrivKey := new(big.Int).Add(hs, mySpendPrivKey)
		oneTimePrivKey.Mod(oneTimePrivKey, Curve().N)
		return true, oneTimePrivKey
	}

	return false, nil
}

// ComputeSharedSecret computes the ECDH shared secret
// For sender: sharedSecret = r * recipientViewPubKey
// For recipient: sharedSecret = viewPrivKey * txPubKey
func ComputeSharedSecret(privKey *big.Int, pubKey *ECPoint) []byte {
	temp := ScalarMult(privKey, pubKey)
	if temp == nil {
		return nil
	}
	return Hash256(temp.Bytes())
}

// DeriveOneTimePrivateKey derives the one-time private key from the shared secret
// oneTimePrivKey = Hash(sharedSecret) + spendPrivKey (mod n)
func DeriveOneTimePrivateKey(sharedSecret []byte, spendPrivKey *big.Int) *big.Int {
	hs := HashToScalar(sharedSecret)
	oneTimePrivKey := new(big.Int).Add(hs, spendPrivKey)
	oneTimePrivKey.Mod(oneTimePrivKey, Curve().N)
	return oneTimePrivKey
}

// GenerateRandomScalar generates a random scalar in [1, n-1]
func GenerateRandomScalar() (*big.Int, error) {
	n := Curve().N
	for {
		// Generate 32 random bytes
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}

		// Convert to big.Int
		k := new(big.Int).SetBytes(b)

		// Ensure k is in [1, n-1]
		if k.Cmp(big.NewInt(0)) > 0 && k.Cmp(n) < 0 {
			return k, nil
		}
	}
}

// ValidateStealthAddress validates a stealth address
func ValidateStealthAddress(addr *StealthAddress) error {
	if addr == nil {
		return fmt.Errorf("stealth address is nil")
	}

	// Check public key is on curve
	if addr.PublicKey == nil || !addr.PublicKey.IsOnCurve() {
		return fmt.Errorf("public key is not on curve")
	}

	// Check transaction public key is on curve
	if addr.TxPublicKey == nil || !addr.TxPublicKey.IsOnCurve() {
		return fmt.Errorf("transaction public key is not on curve")
	}

	// Check not identity element
	if addr.PublicKey.IsIdentity() {
		return fmt.Errorf("public key is identity element")
	}
	if addr.TxPublicKey.IsIdentity() {
		return fmt.Errorf("transaction public key is identity element")
	}

	return nil
}
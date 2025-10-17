package crypto

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
)

// Signature represents an ECDSA signature
type Signature struct {
	R *big.Int
	S *big.Int
}

// SignMessage signs a message using ECDSA with the private key
// Returns: signature bytes (64 bytes: R || S)
func SignMessage(privKey *big.Int, message []byte) ([]byte, error) {
	if privKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	if len(message) == 0 {
		return nil, fmt.Errorf("message is empty")
	}

	// Ensure private key is 32 bytes
	privKeyBytes := make([]byte, 32)
	privKeyB := privKey.Bytes()
	copy(privKeyBytes[32-len(privKeyB):], privKeyB)

	// Convert to btcec private key
	btcPrivKey, _ := btcec.PrivKeyFromBytes(privKeyBytes)

	// Hash the message
	msgHash := Hash256(message)

	// Sign the hash
	sig := btcecdsa.Sign(btcPrivKey, msgHash)

	// Serialize to compact format (64 bytes: R || S)
	sigBytes := make([]byte, 64)

	// Get R and S as byte arrays (they are 32 bytes each)
	r := sig.R()
	s := sig.S()
	rBytes := r.Bytes()
	sBytes := s.Bytes()

	// Copy R and S (they are already 32 bytes)
	copy(sigBytes[0:32], rBytes[:])
	copy(sigBytes[32:64], sBytes[:])

	return sigBytes, nil
}

// VerifySignature verifies an ECDSA signature
// signature: 64 bytes (R || S)
// pubKey: EC point (X, Y)
// message: message that was signed
func VerifySignature(pubKey *ECPoint, message []byte, signature []byte) bool {
	if pubKey == nil || len(message) == 0 || len(signature) != 64 {
		return false
	}

	// Parse R and S from signature
	var r, s btcec.ModNScalar
	if overflow := r.SetByteSlice(signature[0:32]); overflow {
		return false
	}
	if overflow := s.SetByteSlice(signature[32:64]); overflow {
		return false
	}

	// Create btcec signature
	sig := btcecdsa.NewSignature(&r, &s)

	// Convert ECPoint to btcec.PublicKey
	pubKeyBytes := pubKey.Compressed()
	btcPubKey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return false
	}

	// Hash the message
	msgHash := Hash256(message)

	// Verify the signature
	return sig.Verify(msgHash, btcPubKey)
}

// SignNullifier signs a nullifier for Phase 1 private transfer
// This proves ownership of the one-time private key without revealing it
// Message format: nullifier_bytes
func SignNullifier(oneTimePrivKey *big.Int, nullifier *Nullifier) ([]byte, error) {
	if oneTimePrivKey == nil {
		return nil, fmt.Errorf("one-time private key is nil")
	}
	if nullifier == nil {
		return nil, fmt.Errorf("nullifier is nil")
	}

	// Message to sign is the nullifier itself
	message := nullifier.Bytes()
	if len(message) == 0 {
		return nil, fmt.Errorf("nullifier bytes are empty")
	}

	return SignMessage(oneTimePrivKey, message)
}

// VerifyNullifierSignature verifies a nullifier signature for Phase 1
// This verifies that the signer knows the one-time private key for the given address
func VerifyNullifierSignature(
	oneTimeAddr *ECPoint,
	nullifier *Nullifier,
	signature []byte,
) bool {
	if oneTimeAddr == nil || nullifier == nil || len(signature) != 64 {
		return false
	}

	// Message is the nullifier bytes
	message := nullifier.Bytes()
	if len(message) == 0 {
		return false
	}

	return VerifySignature(oneTimeAddr, message, signature)
}

// SignUnshield signs an unshield request for Phase 1
// Message format: nullifier || recipient_address || amount
func SignUnshield(
	oneTimePrivKey *big.Int,
	nullifier *Nullifier,
	recipientAddr string,
	amount string,
) ([]byte, error) {
	if oneTimePrivKey == nil {
		return nil, fmt.Errorf("one-time private key is nil")
	}
	if nullifier == nil {
		return nil, fmt.Errorf("nullifier is nil")
	}
	if recipientAddr == "" {
		return nil, fmt.Errorf("recipient address is empty")
	}
	if amount == "" {
		return nil, fmt.Errorf("amount is empty")
	}

	// Construct message: nullifier || recipient || amount
	message := append(nullifier.Bytes(), []byte(recipientAddr)...)
	message = append(message, []byte(amount)...)

	return SignMessage(oneTimePrivKey, message)
}

// VerifyUnshieldSignature verifies an unshield signature for Phase 1
func VerifyUnshieldSignature(
	oneTimeAddr *ECPoint,
	nullifier *Nullifier,
	recipientAddr string,
	amount string,
	signature []byte,
) bool {
	if oneTimeAddr == nil || nullifier == nil || len(signature) != 64 {
		return false
	}
	if recipientAddr == "" || amount == "" {
		return false
	}

	// Reconstruct message
	message := append(nullifier.Bytes(), []byte(recipientAddr)...)
	message = append(message, []byte(amount)...)

	return VerifySignature(oneTimeAddr, message, signature)
}

// ParseSignature parses a signature from bytes
func ParseSignature(sigBytes []byte) (*Signature, error) {
	if len(sigBytes) != 64 {
		return nil, fmt.Errorf("invalid signature length: expected 64 bytes, got %d", len(sigBytes))
	}

	r := new(big.Int).SetBytes(sigBytes[0:32])
	s := new(big.Int).SetBytes(sigBytes[32:64])

	return &Signature{R: r, S: s}, nil
}

// Bytes returns the byte representation of the signature (64 bytes)
func (s *Signature) Bytes() []byte {
	if s == nil || s.R == nil || s.S == nil {
		return nil
	}

	sigBytes := make([]byte, 64)
	rBytes := s.R.Bytes()
	sBytes := s.S.Bytes()

	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)

	return sigBytes
}

// Verify verifies the signature against a public key and message
func (s *Signature) Verify(pubKey *ECPoint, message []byte) bool {
	if s == nil {
		return false
	}
	return VerifySignature(pubKey, message, s.Bytes())
}

// ConvertPrivKeyToECDSA converts a big.Int private key to ecdsa.PrivateKey
// This is useful for compatibility with other crypto libraries
func ConvertPrivKeyToECDSA(privKey *big.Int) *ecdsa.PrivateKey {
	if privKey == nil {
		return nil
	}

	// Ensure private key is 32 bytes
	privKeyBytes := make([]byte, 32)
	privKeyB := privKey.Bytes()
	copy(privKeyBytes[32-len(privKeyB):], privKeyB)

	btcPrivKey, _ := btcec.PrivKeyFromBytes(privKeyBytes)
	return btcPrivKey.ToECDSA()
}

// ConvertPubKeyToECDSA converts an ECPoint to ecdsa.PublicKey
func ConvertPubKeyToECDSA(pubKey *ECPoint) *ecdsa.PublicKey {
	if pubKey == nil || pubKey.X == nil || pubKey.Y == nil {
		return nil
	}

	return &ecdsa.PublicKey{
		Curve: Curve(),
		X:     pubKey.X,
		Y:     pubKey.Y,
	}
}

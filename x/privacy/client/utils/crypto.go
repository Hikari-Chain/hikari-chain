package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/crypto"
)

// StealthAddressResult contains the result of generating a stealth address
type StealthAddressResult struct {
	OneTimeAddress *crypto.ECPoint
	TxPublicKey    *crypto.ECPoint
	SharedSecret   []byte
	RandomR        *big.Int
}

// GenerateStealthAddress generates a one-time stealth address for a recipient
func GenerateStealthAddress(recipientViewPubKey, recipientSpendPubKey *crypto.ECPoint) (*StealthAddressResult, error) {
	stealthAddr, sharedSecret, r, err := crypto.GenerateStealthAddress(recipientViewPubKey, recipientSpendPubKey)
	if err != nil {
		return nil, err
	}

	return &StealthAddressResult{
		OneTimeAddress: stealthAddr.PublicKey,
		TxPublicKey:    stealthAddr.TxPublicKey,
		SharedSecret:   sharedSecret,
		RandomR:        r,
	}, nil
}

// CreateCommitment creates a Pedersen commitment to an amount
// Returns: (commitment point, blinding factor, error)
func CreateCommitment(amount uint64) (*crypto.ECPoint, *big.Int, error) {
	blinding, err := crypto.GenerateBlinding()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate blinding: %w", err)
	}

	commitment, err := crypto.CreateCommitment(amount, blinding)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create commitment: %w", err)
	}

	return commitment.Point, blinding, nil
}

// EncryptedNote contains an encrypted note with ephemeral key
type EncryptedNote struct {
	Ciphertext   []byte
	Nonce        []byte
	EphemeralKey *crypto.ECPoint
}

// EncryptNote encrypts a note containing amount and blinding factor
// The note is encrypted using AES-GCM with a key derived from the shared secret
func EncryptNote(amount uint64, blinding *big.Int, sharedSecret []byte) (*EncryptedNote, error) {
	// Derive encryption key from shared secret
	encryptionKey := crypto.Hash256(append(sharedSecret, []byte("note_encryption")...))[:32]

	// Create plaintext: amount (8 bytes) || blinding (32 bytes)
	plaintext := make([]byte, 40)
	binary.LittleEndian.PutUint64(plaintext[0:8], amount)
	blindingBytes := blinding.Bytes()
	copy(plaintext[40-len(blindingBytes):], blindingBytes)

	// Generate random nonce (12 bytes for AES-GCM)
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	// Generate ephemeral key for the note (same as transaction public key in practice)
	// For now, we'll use a deterministic derivation from shared secret
	ephemeralScalar := crypto.HashToScalar(append(sharedSecret, []byte("ephemeral_key")...))
	ephemeralKey := crypto.ScalarBaseMult(ephemeralScalar)

	return &EncryptedNote{
		Ciphertext:   ciphertext,
		Nonce:        nonce,
		EphemeralKey: ephemeralKey,
	}, nil
}

// DecryptNote decrypts a note to recover amount and blinding factor
func DecryptNote(ciphertext, nonce []byte, sharedSecret []byte) (uint64, *big.Int, error) {
	// Derive encryption key from shared secret
	encryptionKey := crypto.Hash256(append(sharedSecret, []byte("note_encryption")...))[:32]

	// Create AES-GCM cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	if len(plaintext) != 40 {
		return 0, nil, fmt.Errorf("invalid plaintext length: expected 40, got %d", len(plaintext))
	}

	// Parse amount and blinding
	amount := binary.LittleEndian.Uint64(plaintext[0:8])
	blinding := new(big.Int).SetBytes(plaintext[8:40])

	return amount, blinding, nil
}

// GenerateNullifier generates a nullifier (key image) from a one-time private key
func GenerateNullifier(oneTimePrivKey *big.Int, oneTimeAddr *crypto.ECPoint) ([]byte, error) {
	nullifier, err := crypto.GenerateNullifier(oneTimePrivKey, oneTimeAddr)
	if err != nil {
		return nil, err
	}
	return nullifier.Bytes(), nil
}

// DecompressPubKey decompresses a compressed secp256k1 public key
func DecompressPubKey(compressed []byte) (*crypto.ECPoint, error) {
	point := crypto.DecompressPoint(compressed)
	if point == nil {
		return nil, fmt.Errorf("failed to decompress public key")
	}
	return point, nil
}

// ParsePrivateKeyHex parses a hex-encoded private key
func ParsePrivateKeyHex(hexKey string) (*big.Int, error) {
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("private key must be 32 bytes (64 hex chars), got %d", len(hexKey))
	}

	privKey := new(big.Int)
	privKey.SetString(hexKey, 16)

	// Validate it's in valid range [1, n-1]
	n := crypto.Curve().N
	if privKey.Cmp(big.NewInt(0)) <= 0 || privKey.Cmp(n) >= 0 {
		return nil, fmt.Errorf("private key out of valid range")
	}

	return privKey, nil
}

// CheckIfDepositIsMine checks if a deposit belongs to the user
// Returns: (isMine, oneTimePrivateKey if mine, error)
func CheckIfDepositIsMine(
	oneTimeAddr, txPubKey *crypto.ECPoint,
	myViewPrivKey *big.Int,
	mySpendPubKey *crypto.ECPoint,
	mySpendPrivKey *big.Int,
) (bool, *big.Int, error) {
	isMine, oneTimePrivKey := crypto.CheckIfMine(
		oneTimeAddr,
		txPubKey,
		myViewPrivKey,
		mySpendPubKey,
		mySpendPrivKey,
	)

	return isMine, oneTimePrivKey, nil
}

// SignNullifier creates an ECDSA signature over the nullifier using the one-time private key
// This proves ownership of the deposit in Phase 1
func SignNullifier(nullifier []byte, oneTimePrivKey *big.Int) ([]byte, error) {
	// TODO: Implement proper ECDSA signature
	// For Phase 1, we need to sign the nullifier with the one-time private key
	// This proves we know the private key corresponding to the one-time address
	// without revealing it.
	//
	// Placeholder: In a real implementation, use:
	// - crypto/ecdsa or btcec package
	// - Sign Hash(nullifier) with oneTimePrivKey
	// - Return DER-encoded signature

	// For now, return a placeholder signature
	return crypto.Hash256(append(nullifier, oneTimePrivKey.Bytes()...)), nil
}

// GenerateKeyPair generates a new stealth address key pair
func GenerateKeyPair() (*crypto.StealthKeyPair, error) {
	return crypto.GenerateStealthKeyPair()
}

// ExportPublicKeys exports public keys as hex-encoded compressed points
func ExportPublicKeys(keyPair *crypto.StealthKeyPair) (viewPubHex, spendPubHex string) {
	viewPubHex = fmt.Sprintf("%x", keyPair.ViewPublicKey.Compressed())
	spendPubHex = fmt.Sprintf("%x", keyPair.SpendPublicKey.Compressed())
	return
}

// ExportPrivateKeys exports private keys as hex-encoded scalars
func ExportPrivateKeys(keyPair *crypto.StealthKeyPair) (viewPrivHex, spendPrivHex string) {
	viewPrivBytes := keyPair.ViewPrivateKey.Bytes()
	viewPriv32 := make([]byte, 32)
	copy(viewPriv32[32-len(viewPrivBytes):], viewPrivBytes)
	viewPrivHex = fmt.Sprintf("%x", viewPriv32)

	spendPrivBytes := keyPair.SpendPrivateKey.Bytes()
	spendPriv32 := make([]byte, 32)
	copy(spendPriv32[32-len(spendPrivBytes):], spendPrivBytes)
	spendPrivHex = fmt.Sprintf("%x", spendPriv32)

	return
}

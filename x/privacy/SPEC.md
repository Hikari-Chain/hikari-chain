# Privacy Module Technical Specification

This document provides detailed technical specifications for implementing the privacy module. It complements the [README.md](README.md) with implementation-specific details for developers.

## Table of Contents

- [Cryptographic Primitives](#cryptographic-primitives)
  - [Elliptic Curve Configuration](#elliptic-curve-configuration)
  - [Stealth Address Generation](#stealth-address-generation)
  - [Pedersen Commitments](#pedersen-commitments)
  - [Nullifier Generation](#nullifier-generation)
- [State Management](#state-management)
  - [Storage Keys](#storage-keys)
  - [Data Structures](#data-structures)
  - [Indexing Strategy](#indexing-strategy)
- [Message Validation](#message-validation)
- [Keeper Methods](#keeper-methods)
- [Client Library](#client-library)
- [Security Considerations](#security-considerations)
- [Testing Requirements](#testing-requirements)
- [Performance Targets](#performance-targets)

## Cryptographic Primitives

### Elliptic Curve Configuration

**Curve**: secp256k1 (same as Bitcoin/Ethereum)

```go
import (
    "crypto/elliptic"
    "github.com/btcsuite/btcd/btcec/v2"
)

// Use secp256k1 curve
curve := btcec.S256()

// Curve parameters
// Order (n): FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
// Generator (G): (x, y) coordinates
```

**Why secp256k1?**
- Well-tested and battle-hardened (Bitcoin, Ethereum)
- Efficient implementations available
- Compatible with Cosmos SDK key types
- Good performance for ECDH operations

### Stealth Address Generation

**Algorithm**: Dual-Key Stealth Address Protocol (Monero-style)

#### Key Generation

User generates two key pairs:

```go
// View key pair (for scanning)
viewPrivateKey := generateRandomScalar()  // 32 bytes
viewPublicKey := viewPrivateKey * G       // EC point

// Spend key pair (for spending)
spendPrivateKey := generateRandomScalar() // 32 bytes
spendPublicKey := spendPrivateKey * G     // EC point
```

#### Sending to Stealth Address

```go
// Sender (Alice) sending to recipient (Bob)
func GenerateStealthAddress(
    recipientViewPubKey,
    recipientSpendPubKey ECPoint,
) (oneTimeAddr ECPoint, txPubKey ECPoint, sharedSecret []byte) {

    // 1. Generate random scalar r
    r := generateRandomScalar() // 32 bytes

    // 2. Compute transaction public key R = r*G
    txPubKey = scalarMult(r, G)

    // 3. Compute shared secret
    // sharedSecret = Hash(r * recipientViewPubKey)
    temp := scalarMult(r, recipientViewPubKey)
    sharedSecret = SHA256(encodePoint(temp))

    // 4. Derive one-time public key
    // P = Hash(sharedSecret)*G + recipientSpendPubKey
    hs := new(big.Int).SetBytes(sharedSecret)
    hs.Mod(hs, curve.Params().N) // Reduce modulo curve order

    hsG := scalarMult(hs, G)
    oneTimeAddr = pointAdd(hsG, recipientSpendPubKey)

    return oneTimeAddr, txPubKey, sharedSecret
}
```

#### Scanning for Stealth Addresses

```go
// Recipient (Bob) scanning for his addresses
func CheckIfMine(
    oneTimeAddr, txPubKey ECPoint,
    myViewPrivKey, mySpendPubKey ECPoint,
) (bool, *big.Int) {

    // 1. Compute shared secret
    // sharedSecret = Hash(viewPrivKey * txPubKey)
    temp := scalarMult(myViewPrivKey, txPubKey)
    sharedSecret := SHA256(encodePoint(temp))

    // 2. Derive expected one-time public key
    hs := new(big.Int).SetBytes(sharedSecret)
    hs.Mod(hs, curve.Params().N)

    hsG := scalarMult(hs, G)
    expectedAddr := pointAdd(hsG, mySpendPubKey)

    // 3. Check if it matches
    if pointsEqual(expectedAddr, oneTimeAddr) {
        // This is mine! Compute private key
        // oneTimePrivKey = Hash(sharedSecret) + spendPrivKey
        oneTimePrivKey := new(big.Int).Add(hs, mySpendPrivKey)
        oneTimePrivKey.Mod(oneTimePrivKey, curve.Params().N)
        return true, oneTimePrivKey
    }

    return false, nil
}
```

#### Key Encoding

```go
// ECPoint encoding for storage and transmission
type ECPoint struct {
    X []byte // 32 bytes
    Y []byte // 32 bytes
}

// Compressed encoding (33 bytes)
func compressPoint(p ECPoint) []byte {
    // Format: [0x02/0x03][X-coordinate]
    // 0x02 if Y is even, 0x03 if Y is odd
    compressed := make([]byte, 33)
    if p.Y[31]&1 == 0 {
        compressed[0] = 0x02
    } else {
        compressed[0] = 0x03
    }
    copy(compressed[1:], p.X)
    return compressed
}
```

### Pedersen Commitments

**Scheme**: Elliptic Curve Pedersen Commitments

#### Generator Point Setup

```go
// G: Standard secp256k1 generator (well-known)
G := curve.Params().Gx, curve.Params().Gy

// H: Second generator (nothing-up-my-sleeve)
// Derive from hash of G
H := deriveH()

func deriveH() ECPoint {
    // Hash "nothing-up-my-sleeve" constant
    data := []byte("Hikari Chain Privacy Module - H Generator Point")
    hash := SHA256(data)

    // Find point on curve (try incrementing until valid)
    x := new(big.Int).SetBytes(hash)
    for {
        y := solveForY(x)
        if y != nil {
            // Found valid point
            return ECPoint{X: x, Y: y}
        }
        x.Add(x, big.NewInt(1))
        x.Mod(x, curve.Params().P)
    }
}
```

#### Creating Commitments

```go
// Commit to an amount
func CreateCommitment(amount uint64, blinding *big.Int) ECPoint {
    // C = amount*H + blinding*G

    // amount*H
    amountBig := new(big.Int).SetUint64(amount)
    amountH := scalarMult(amountBig, H)

    // blinding*G
    blindingG := scalarMult(blinding, G)

    // Add the points
    commitment := pointAdd(amountH, blindingG)

    return commitment
}

// Generate random blinding factor
func generateBlinding() *big.Int {
    // Random scalar in [1, n-1]
    for {
        bytes := make([]byte, 32)
        rand.Read(bytes)

        r := new(big.Int).SetBytes(bytes)
        if r.Cmp(big.NewInt(0)) > 0 && r.Cmp(curve.Params().N) < 0 {
            return r
        }
    }
}
```

#### Verifying Commitment Balance

```go
// Verify: C_in = C_out1 + C_out2 + ... + C_outN
func VerifyCommitmentBalance(
    inputCommitment ECPoint,
    outputCommitments []ECPoint,
) bool {

    // Sum all output commitments
    sum := outputCommitments[0]
    for i := 1; i < len(outputCommitments); i++ {
        sum = pointAdd(sum, outputCommitments[i])
    }

    // Check if sum equals input
    return pointsEqual(inputCommitment, sum)
}
```

### Nullifier Generation

#### Phase 1: Key Image (Monero-style)

```go
// Generate nullifier (key image) for a note
func GenerateNullifier(oneTimePrivKey *big.Int, oneTimeAddr ECPoint) ECPoint {
    // I = oneTimePrivKey * Hp(oneTimeAddr)

    // 1. Hash one-time address to point
    hp := hashToPoint(oneTimeAddr)

    // 2. Multiply by private key
    nullifier := scalarMult(oneTimePrivKey, hp)

    return nullifier
}

// Hash-to-point function
func hashToPoint(p ECPoint) ECPoint {
    // Hash the point
    data := append(p.X, p.Y...)
    hash := SHA256(data)

    // Find point on curve
    x := new(big.Int).SetBytes(hash)
    for {
        y := solveForY(x)
        if y != nil {
            return ECPoint{X: x, Y: y}
        }
        x.Add(x, big.NewInt(1))
        x.Mod(x, curve.Params().P)
    }
}

// Verify nullifier is unique (prevents double-spend)
func VerifyNullifier(
    nullifier ECPoint,
    signature []byte,
    message []byte,
) bool {
    // Verify ECDSA signature with one-time public key
    // This proves knowledge of private key without revealing which note
    return ecdsa.Verify(oneTimeAddr, message, signature)
}
```

#### Phase 2: ZK-SNARK Nullifier

```go
// Generate nullifier for ZK proof
func GenerateNullifierZK(secret, commitment []byte) []byte {
    // N = Hash(secret || commitment)
    data := append(secret, commitment...)
    return SHA256(data) // 32 bytes
}
```

## State Management

### Storage Keys

```go
// Key prefixes
const (
    // Phase 1 & 2 common
    ParamsKey               = []byte{0x01}
    NullifierPrefix         = []byte{0x02}

    // Phase 1 specific
    DepositPrefix           = []byte{0x10}
    DepositCountPrefix      = []byte{0x11}
    DepositBlockIndexPrefix = []byte{0x12}

    // Phase 2 specific
    TreeRootPrefix          = []byte{0x20}
    TreeLeafPrefix          = []byte{0x21}
    TreeNodePrefix          = []byte{0x22}
    TreeNextIndexPrefix     = []byte{0x23}
    VerificationKeyPrefix   = []byte{0x24}
)

// Key construction functions

// Deposit key: 0x10 || len(denom) || denom || index
func DepositKey(denom string, index uint64) []byte {
    denomBytes := []byte(denom)
    denomLen := byte(len(denomBytes))

    indexBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(indexBytes, index)

    return append(
        append(
            append(DepositPrefix, denomLen),
            denomBytes...,
        ),
        indexBytes...,
    )
}

// Nullifier key: 0x02 || nullifier_hash
func NullifierKey(nullifier []byte) []byte {
    return append(NullifierPrefix, nullifier...)
}

// Tree root key: 0x20 || len(denom) || denom
func TreeRootKey(denom string) []byte {
    denomBytes := []byte(denom)
    denomLen := byte(len(denomBytes))

    return append(
        append(TreeRootPrefix, denomLen),
        denomBytes...,
    )
}

// Tree leaf key: 0x21 || len(denom) || denom || index
func TreeLeafKey(denom string, index uint64) []byte {
    denomBytes := []byte(denom)
    denomLen := byte(len(denomBytes))

    indexBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(indexBytes, index)

    return append(
        append(
            append(TreeLeafPrefix, denomLen),
            denomBytes...,
        ),
        indexBytes...,
    )
}

// Tree node key: 0x22 || len(denom) || denom || level || index
func TreeNodeKey(denom string, level uint32, index uint64) []byte {
    denomBytes := []byte(denom)
    denomLen := byte(len(denomBytes))

    levelBytes := make([]byte, 4)
    binary.BigEndian.PutUint32(levelBytes, level)

    indexBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(indexBytes, index)

    return append(
        append(
            append(
                append(TreeNodePrefix, denomLen),
                denomBytes...,
            ),
            levelBytes...,
        ),
        indexBytes...,
    )
}
```

### Data Structures

```go
// Protobuf message structures (see proto definitions)

// Note: Internal representation (not stored on-chain)
type Note struct {
    Amount           uint64
    Secret           []byte  // 32 bytes
    Blinding         []byte  // 32 bytes
    RecipientViewKey ECPoint
    RecipientSpendKey ECPoint
    Denom            string
}

// PrivateDeposit: On-chain storage (Phase 1)
type PrivateDeposit struct {
    OneTimeAddress  OneTimeAddress      // Stealth address
    Commitment      PedersenCommitment  // Amount commitment
    EncryptedAmount []byte              // AES-GCM encrypted
    BlockHeight     uint64              // For scanning
    Denom           string              // Asset denomination
}

// Commitment: Merkle tree storage (Phase 2)
type Commitment struct {
    Value []byte  // 32 bytes (compressed EC point)
}
```

### Indexing Strategy

#### Phase 1: Block Height Index

```go
// Index deposits by block height for efficient scanning
func IndexDepositByBlock(
    store sdk.KVStore,
    denom string,
    blockHeight uint64,
    depositIndex uint64,
) {
    // Key: 0x12 || len(denom) || denom || block_height || deposit_index
    key := DepositBlockIndexKey(denom, blockHeight, depositIndex)

    // Value: nil (existence check only)
    store.Set(key, []byte{})
}

// Query deposits by block range
func GetDepositsByBlockRange(
    store sdk.KVStore,
    denom string,
    fromBlock, toBlock uint64,
) []uint64 {
    indices := []uint64{}

    for height := fromBlock; height <= toBlock; height++ {
        // Iterate deposits at this height
        prefix := DepositBlockIndexPrefix(denom, height)
        iterator := sdk.KVStorePrefixIterator(store, prefix)
        defer iterator.Close()

        for ; iterator.Valid(); iterator.Next() {
            index := ExtractIndexFromKey(iterator.Key())
            indices = append(indices, index)
        }
    }

    return indices
}
```

#### Phase 2: Merkle Tree Management

```go
// Merkle tree depth (32 levels = 2^32 capacity)
const TreeDepth = 32

// Update Merkle tree when adding commitment
func UpdateMerkleTree(
    store sdk.KVStore,
    denom string,
    commitment []byte,
) (uint64, []byte) {

    // Get next index
    index := GetNextIndex(store, denom)

    // Store leaf
    leafKey := TreeLeafKey(denom, index)
    store.Set(leafKey, commitment)

    // Update tree nodes from leaf to root
    currentHash := commitment
    currentIndex := index

    for level := 0; level < TreeDepth; level++ {
        // Get sibling
        siblingIndex := currentIndex ^ 1  // XOR with 1 flips last bit
        siblingKey := TreeNodeKey(denom, uint32(level), siblingIndex)
        sibling := store.Get(siblingKey)

        if len(sibling) == 0 {
            // Sibling doesn't exist, use zero hash
            sibling = make([]byte, 32)
        }

        // Hash with sibling
        if currentIndex%2 == 0 {
            // Current is left child
            currentHash = HashPair(currentHash, sibling)
        } else {
            // Current is right child
            currentHash = HashPair(sibling, currentHash)
        }

        // Store parent node
        parentIndex := currentIndex / 2
        parentKey := TreeNodeKey(denom, uint32(level+1), parentIndex)
        store.Set(parentKey, currentHash)

        currentIndex = parentIndex
    }

    // Update root
    rootKey := TreeRootKey(denom)
    store.Set(rootKey, currentHash)

    // Increment next index
    SetNextIndex(store, denom, index+1)

    return index, currentHash
}

// Hash pair of nodes
func HashPair(left, right []byte) []byte {
    data := append(left, right...)
    hash := SHA256(data)
    return hash
}

// Get Merkle proof for leaf
func GetMerkleProof(
    store sdk.KVStore,
    denom string,
    leafIndex uint64,
) [][]byte {

    proof := make([][]byte, TreeDepth)
    currentIndex := leafIndex

    for level := 0; level < TreeDepth; level++ {
        // Get sibling
        siblingIndex := currentIndex ^ 1
        siblingKey := TreeNodeKey(denom, uint32(level), siblingIndex)
        sibling := store.Get(siblingKey)

        if len(sibling) == 0 {
            sibling = make([]byte, 32)
        }

        proof[level] = sibling
        currentIndex = currentIndex / 2
    }

    return proof
}
```

## Message Validation

### MsgShield Validation

```go
func (k Keeper) ValidateShield(ctx sdk.Context, msg *MsgShield) error {
    // 1. Check denomination is allowed
    params := k.GetParams(ctx)
    if !contains(params.AllowedDenoms, msg.Amount.Denom) {
        return sdkerrors.Wrapf(
            ErrDenomNotAllowed,
            "denomination %s not in allowed list",
            msg.Amount.Denom,
        )
    }

    // 2. Check minimum amount
    minAmount := params.MinShieldAmounts[msg.Amount.Denom]
    if msg.Amount.Amount.LT(minAmount) {
        return sdkerrors.Wrapf(
            ErrBelowMinimum,
            "amount %s below minimum %s",
            msg.Amount.Amount,
            minAmount,
        )
    }

    // 3. Validate stealth address
    if !k.ValidateStealthAddress(msg.StealthAddress) {
        return ErrInvalidStealthAddress
    }

    // 4. Validate commitment (points on curve)
    if !k.ValidateCommitment(msg.Commitment) {
        return ErrInvalidCommitment
    }

    // 5. Check sender has balance
    sender := sdk.MustAccAddressFromBech32(msg.Sender)
    if !k.bankKeeper.HasBalance(ctx, sender, msg.Amount) {
        return sdkerrors.ErrInsufficientFunds
    }

    return nil
}

func (k Keeper) ValidateStealthAddress(addr OneTimeAddress) bool {
    // Check public key is on curve
    if !isOnCurve(addr.PublicKey) {
        return false
    }

    // Check transaction public key is on curve
    if !isOnCurve(addr.TxPublicKey) {
        return false
    }

    // Check not identity element
    if isIdentity(addr.PublicKey) || isIdentity(addr.TxPublicKey) {
        return false
    }

    return true
}
```

### MsgPrivateTransfer Validation

```go
// Phase 1 validation
func (k Keeper) ValidatePrivateTransferPhase1(
    ctx sdk.Context,
    msg *MsgPrivateTransfer,
) error {

    // 1. Check input deposit exists
    deposit, found := k.GetDeposit(ctx, msg.Denom, msg.InputDepositIndex)
    if !found {
        return ErrDepositNotFound
    }

    // 2. Check nullifier not used
    if k.IsNullifierUsed(ctx, msg.Nullifier) {
        return ErrDoubleSpend
    }

    // 3. Verify spend signature
    if !k.VerifySpendSignature(msg, deposit) {
        return ErrInvalidSignature
    }

    // 4. Verify commitment balance
    if !k.VerifyCommitmentBalance(
        deposit.Commitment,
        extractCommitments(msg.Outputs),
    ) {
        return ErrInvalidCommitmentBalance
    }

    // 5. Check all outputs have same denom
    for _, output := range msg.Outputs {
        if output.Denom != deposit.Denom {
            return ErrDenomMismatch
        }
    }

    // 6. Check max outputs limit
    params := k.GetParams(ctx)
    if len(msg.Outputs) > int(params.MaxOutputs) {
        return ErrTooManyOutputs
    }

    return nil
}

// Phase 2 validation
func (k Keeper) ValidatePrivateTransferPhase2(
    ctx sdk.Context,
    msg *MsgPrivateTransfer,
) error {

    // 1. Check merkle root matches current state
    currentRoot := k.GetMerkleRoot(ctx, msg.Denom)
    if !bytes.Equal(currentRoot, msg.MerkleRoot) {
        return ErrInvalidMerkleRoot
    }

    // 2. Check nullifiers not used
    for _, nullifier := range msg.Nullifiers {
        if k.IsNullifierUsed(ctx, nullifier) {
            return ErrDoubleSpend
        }
    }

    // 3. Verify ZK-SNARK proof
    if !k.VerifyZKProof(ctx, msg) {
        return ErrInvalidProof
    }

    // 4. Check max outputs limit
    params := k.GetParams(ctx)
    if len(msg.OutputCommitments) > int(params.MaxOutputs) {
        return ErrTooManyOutputs
    }

    return nil
}
```

## Keeper Methods

### Core Keeper Interface

```go
type Keeper interface {
    // Parameters
    GetParams(ctx sdk.Context) Params
    SetParams(ctx sdk.Context, params Params)

    // Deposits (Phase 1)
    GetDeposit(ctx sdk.Context, denom string, index uint64) (PrivateDeposit, bool)
    SetDeposit(ctx sdk.Context, denom string, index uint64, deposit PrivateDeposit)
    GetNextDepositIndex(ctx sdk.Context, denom string) uint64
    GetDepositsByBlockRange(ctx sdk.Context, denom string, from, to uint64) []PrivateDeposit

    // Merkle Tree (Phase 2)
    GetMerkleRoot(ctx sdk.Context, denom string) []byte
    GetMerkleProof(ctx sdk.Context, denom string, index uint64) [][]byte
    AddCommitmentToTree(ctx sdk.Context, denom string, commitment []byte) (uint64, []byte)

    // Nullifiers (Both phases)
    IsNullifierUsed(ctx sdk.Context, nullifier []byte) bool
    MarkNullifierUsed(ctx sdk.Context, nullifier []byte)

    // Cryptography
    ValidateStealthAddress(addr OneTimeAddress) bool
    ValidateCommitment(commitment PedersenCommitment) bool
    VerifySpendSignature(msg interface{}, deposit PrivateDeposit) bool
    VerifyCommitmentBalance(input PedersenCommitment, outputs []PedersenCommitment) bool
    VerifyZKProof(ctx sdk.Context, msg *MsgPrivateTransfer) bool  // Phase 2
}
```

### Implementation Examples

```go
// Get deposit
func (k Keeper) GetDeposit(
    ctx sdk.Context,
    denom string,
    index uint64,
) (PrivateDeposit, bool) {

    store := ctx.KVStore(k.storeKey)
    key := DepositKey(denom, index)

    bz := store.Get(key)
    if bz == nil {
        return PrivateDeposit{}, false
    }

    var deposit PrivateDeposit
    k.cdc.MustUnmarshal(bz, &deposit)

    return deposit, true
}

// Mark nullifier as used
func (k Keeper) MarkNullifierUsed(ctx sdk.Context, nullifier []byte) {
    store := ctx.KVStore(k.storeKey)
    key := NullifierKey(nullifier)
    store.Set(key, []byte{0x01})
}

// Is nullifier used
func (k Keeper) IsNullifierUsed(ctx sdk.Context, nullifier []byte) bool {
    store := ctx.KVStore(k.storeKey)
    key := NullifierKey(nullifier)
    return store.Has(key)
}
```

## Client Library

### Note Management

```go
type NoteManager struct {
    viewPrivateKey  *big.Int
    viewPublicKey   ECPoint
    spendPrivateKey *big.Int
    spendPublicKey  ECPoint

    notes []Note
}

// Scan chain for notes
func (nm *NoteManager) Scan(
    ctx context.Context,
    client QueryClient,
    denom string,
    fromBlock, toBlock uint64,
) error {

    // Query deposits in block range
    deposits, err := client.GetDeposits(ctx, denom, fromBlock, toBlock)
    if err != nil {
        return err
    }

    // Try to decrypt each deposit
    for _, deposit := range deposits {
        note := nm.tryDecrypt(deposit)
        if note != nil {
            nm.notes = append(nm.notes, *note)
        }
    }

    return nil
}

// Try to decrypt a deposit
func (nm *NoteManager) tryDecrypt(deposit PrivateDeposit) *Note {
    // Check if this is my stealth address
    isMine, oneTimePrivKey := CheckIfMine(
        deposit.OneTimeAddress.PublicKey,
        deposit.OneTimeAddress.TxPublicKey,
        nm.viewPrivateKey,
        nm.spendPublicKey,
    )

    if !isMine {
        return nil
    }

    // Decrypt amount
    sharedSecret := ComputeSharedSecret(
        nm.viewPrivateKey,
        deposit.OneTimeAddress.TxPublicKey,
    )

    plaintext := DecryptAESGCM(deposit.EncryptedAmount, sharedSecret)

    // Parse note
    note := ParseNote(plaintext)
    note.OneTimePrivateKey = oneTimePrivKey

    return note
}

// Get balance
func (nm *NoteManager) GetBalance(denom string) uint64 {
    balance := uint64(0)
    for _, note := range nm.notes {
        if note.Denom == denom && !note.Spent {
            balance += note.Amount
        }
    }
    return balance
}
```

### Encryption/Decryption

```go
// Encrypt note data with ECDH shared secret
func EncryptNote(note Note, sharedSecret []byte) []byte {
    // Serialize note
    plaintext := SerializeNote(note)

    // Derive encryption key from shared secret
    encKey := DeriveKey(sharedSecret, "encryption")

    // Encrypt with AES-256-GCM
    ciphertext := EncryptAESGCM(plaintext, encKey)

    return ciphertext
}

func EncryptAESGCM(plaintext, key []byte) []byte {
    block, err := aes.NewCipher(key)
    if err != nil {
        panic(err)
    }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        panic(err)
    }

    // Generate random nonce
    nonce := make([]byte, aesGCM.NonceSize())
    rand.Read(nonce)

    // Encrypt and authenticate
    ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

    return ciphertext
}

func DecryptAESGCM(ciphertext, key []byte) []byte {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil
    }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return nil
    }

    nonceSize := aesGCM.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

    plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil
    }

    return plaintext
}
```

## Security Considerations

### Random Number Generation

```go
import "crypto/rand"

// ALWAYS use crypto/rand for cryptographic randomness
func generateRandomBytes(n int) []byte {
    bytes := make([]byte, n)
    _, err := rand.Read(bytes)
    if err != nil {
        panic("crypto/rand failed: " + err.Error())
    }
    return bytes
}

// NEVER use math/rand for cryptographic purposes
```

### Key Storage

```go
// Keys should NEVER be stored unencrypted
// Use keyring with password protection

type SecureKeyStorage interface {
    // Store encrypted key
    StoreKey(name string, key []byte, password string) error

    // Load and decrypt key
    LoadKey(name string, password string) ([]byte, error)

    // Delete key
    DeleteKey(name string) error
}
```

### Constant-Time Operations

```go
import "crypto/subtle"

// Use constant-time comparison for secrets
func compareSecrets(a, b []byte) bool {
    return subtle.ConstantTimeCompare(a, b) == 1
}
```

## Testing Requirements

### Unit Tests

1. **Cryptographic Primitives**:
   - Stealth address generation and detection
   - Pedersen commitment creation and verification
   - Nullifier generation and uniqueness
   - Encryption/decryption

2. **State Management**:
   - Deposit storage and retrieval
   - Nullifier tracking
   - Merkle tree updates (Phase 2)
   - Key construction

3. **Message Validation**:
   - Valid message acceptance
   - Invalid message rejection
   - Edge cases (zero amounts, invalid keys, etc.)

### Integration Tests

1. **Complete Flows**:
   - Shield → Scan → Transfer → Unshield
   - Multiple users interacting
   - Multiple denominations

2. **Error Cases**:
   - Double-spend attempts
   - Invalid proofs
   - Unauthorized denominations

### Test Vectors

Provide test vectors for:
- Stealth address generation
- Commitment creation
- Nullifier generation
- Merkle tree updates

## Performance Targets

### Phase 1

| Operation | Target | Notes |
|-----------|--------|-------|
| Shield | < 500ms | Client-side crypto + on-chain validation |
| Private Transfer | < 1s | Signature generation + validation |
| Unshield | < 500ms | Similar to Shield |
| Scanning (1000 deposits) | < 5s | Client-side, parallelizable |
| Commitment verification | < 10ms | EC point addition |

### Phase 2

| Operation | Target | Notes |
|-----------|--------|-------|
| Shield | < 500ms | Same as Phase 1 |
| Proof Generation | 5-10s | Client-side, can be pre-generated |
| Proof Verification | < 10ms | On-chain, constant time |
| Merkle tree update | < 50ms | Depth-32 tree |
| Scanning (10000 deposits) | < 10s | With bloom filter optimization |

### Gas Costs

| Operation | Target Gas | Notes |
|-----------|------------|-------|
| Shield | < 200K | Deposit creation |
| Private Transfer (Phase 1) | < 300K | Signature + commitment verification |
| Private Transfer (Phase 2) | < 1M | ZK proof verification |
| Unshield | < 200K | Similar to Shield |

---

**Note**: This specification should be updated as implementation progresses and edge cases are discovered.
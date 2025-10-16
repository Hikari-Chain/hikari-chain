---
sidebar_position: 1
---

# `x/privacy`

## Abstract

This module provides privacy-preserving transaction capabilities to Hikari Chain, introduced by [ADR-HIKARI-001](../../docs/architecture/adr-hikari-001-privacy-deposits.md). Users can shield their funds into a private pool, perform anonymous transfers within the pool, and unshield back to public balances. The implementation follows a two-phase approach: Phase 1 (testnet-only) uses stealth addresses and Pedersen commitments for architecture validation, while Phase 2 (mainnet) employs zk-SNARKs for maximum privacy and unlinkability.

## Contents

- [`x/privacy`](#xprivacy)
  - [Abstract](#abstract)
  - [Contents](#contents)
  - [Concepts](#concepts)
    - [Stealth Addresses](#stealth-addresses)
    - [Pedersen Commitments](#pedersen-commitments)
    - [Nullifiers](#nullifiers)
    - [Phase 1 vs Phase 2](#phase-1-vs-phase-2)
  - [State](#state)
    - [Phase 1 State (Testnet)](#phase-1-state-testnet)
    - [Phase 2 State (Mainnet)](#phase-2-state-mainnet)
  - [Messages](#messages)
    - [MsgShield](#msgshield)
    - [MsgPrivateTransfer](#msgprivatetransfer)
    - [MsgUnshield](#msgunshield)
  - [Events](#events)
  - [Queries](#queries)
  - [Parameters](#parameters)
  - [Client](#client)
    - [CLI](#cli)
    - [gRPC](#grpc)
    - [REST](#rest)
  - [Implementation Phases](#implementation-phases)
  - [References](#references)

## Concepts

The privacy module enables users to perform confidential transactions by:
1. **Shielding** coins from their public balance into a private pool
2. **Transferring** privately within the pool without revealing amounts or recipients
3. **Unshielding** coins back to public balances when needed

### Stealth Addresses

Stealth addresses are one-time addresses generated using ECDH (Elliptic Curve Diffie-Hellman) key exchange. When Alice wants to send funds to Bob:

1. Bob publishes a public view key (`V`) and spend key (`S`)
2. Alice generates an ephemeral key pair (`r`, `R = r*G`)
3. Alice computes a shared secret: `Hs(r*V)`
4. Alice derives Bob's one-time address: `P = Hs(r*V)*G + S`
5. Bob can detect this is his by computing: `Hs(v*R)*G + S = P` where `v` is his private view key

This allows Bob to receive funds at a unique address that only he can link to his identity.

### Pedersen Commitments

Pedersen commitments hide transaction amounts while allowing validators to verify balance equations:

```
C = amount*H + blinding*G
```

Where:
- `G` is the standard elliptic curve generator
- `H` is a second generator point (nothing-up-my-sleeve construction)
- `amount` is the value being committed
- `blinding` is a random scalar for hiding

**Properties:**
- **Hiding**: Commitment reveals nothing about the amount
- **Binding**: Cannot change the committed amount later
- **Homomorphic**: `C1 + C2 = (a1 + a2)*H + (b1 + b2)*G`

Validators verify: `C_input = C_output1 + C_output2 + ...` without knowing amounts.

### Nullifiers

Nullifiers prevent double-spending without revealing which note is spent:

**Phase 1 (Simple Key Image):**
```
I = x*Hp(P)
```
Where:
- `x` is the one-time private key
- `Hp` is a hash-to-point function
- `P` is the one-time public key

**Phase 2 (ZK-SNARK Nullifier):**
```
N = Hash(secret, commitment)
```

Each note has a unique nullifier. Once used, it's marked to prevent double-spending.

### Phase 1 vs Phase 2

| Feature | Phase 1 (Testnet) | Phase 2 (Mainnet) |
|---------|-------------------|-------------------|
| **Deployment** | Testnet only | Mainnet production |
| **Proof System** | ECDSA signatures | zk-SNARKs (Groth16/PLONK) |
| **Transaction Graph** | ❌ Visible (deposit indices) | ✅ Hidden (Merkle tree) |
| **Verification Time** | ~100ms | ~5ms (constant) |
| **Anonymity Set** | Limited (timing-based) | Large (all notes in tree) |
| **Privacy Level** | Medium | Maximum |
| **Purpose** | Architecture validation | Production privacy |

**See [ADR-HIKARI-001](../../docs/architecture/adr-hikari-001-privacy-deposits.md) for detailed comparison and [sequence diagrams](../../docs/architecture/diagrams/).**

## State

### Phase 1 State (Testnet)

Phase 1 stores deposits in a simple array structure **per denomination** (separate pools):

**Deposit Storage:**
```
Key:   "deposits/{denom}/{index}"
Value: PrivateDeposit
```

**Nullifier Tracking (global across all denoms):**
```
Key:   "nullifiers/{nullifier_hash}"
Value: bool (true if used)
```

**Deposit Counter per denomination:**
```
Key:   "deposit_count/{denom}"
Value: uint64
```

**Block Index (for efficient scanning):**
```
Key:   "deposits_by_block/{denom}/{block_height}/{index}"
Value: nil (existence check)
```

**Why separate pools per denomination?**
- Cannot mix different denoms in Pedersen commitment arithmetic
- Each denom has independent anonymity set
- Prevents cross-denomination balance correlation

### Phase 2 State (Mainnet)

Phase 2 uses a Merkle tree for unlinkability **per denomination** (separate trees):

**Merkle Tree Root per denomination:**
```
Key:   "tree/{denom}/root"
Value: bytes (32 bytes)
```

**Tree Leaves:**
```
Key:   "tree/{denom}/leaves/{index}"
Value: Commitment (32 bytes)
```

**Tree Nodes:**
```
Key:   "tree/{denom}/nodes/{level}/{index}"
Value: bytes (32 bytes)
```

**Next Index per denomination:**
```
Key:   "tree/{denom}/next_index"
Value: uint64
```

**Nullifier Set (global across all denoms):**
```
Key:   "nullifiers/{nullifier_hash}"
Value: bool (true if used)
```

**Verification Keys (Phase 2 only, shared across all denoms):**
```
Key:   "circuit/vk/transfer"
Value: VerificationKey

Key:   "circuit/vk/unshield"
Value: VerificationKey
```

**Why separate trees per denomination?**
- Each denom needs its own Merkle tree for balance verification
- Cannot mix different denoms in ZK circuit proofs
- Each tree provides independent anonymity set per asset

## Messages

### MsgShield

Deposits coins from public balance to the private pool.

**Protobuf Definition:**
```protobuf
message MsgShield {
  option (cosmos.msg.v1.signer) = "sender";

  string sender = 1;                          // Public address sending funds
  cosmos.base.v1beta1.Coin amount = 2;        // Amount to shield (e.g., "100ulight")

  // Phase 1 & 2:
  OneTimeAddress stealth_address = 3;         // ECDH one-time address
  PedersenCommitment commitment = 4;          // C = amount*H + blinding*G
  bytes encrypted_amount = 5;                 // Encrypted with ECDH shared secret
}
```

**Validation:**
1. Sender must have sufficient balance
2. Stealth address points must be on the elliptic curve
3. Amount must be positive and meet minimum threshold
4. **Denomination must be in `allowed_denoms` parameter** (governance-controlled)
5. Amount must be >= `min_shield_amounts[denom]`

**State Changes:**
- Burns `amount` from sender's public balance
- Adds deposit to denomination-specific pool (Phase 1: array, Phase 2: Merkle tree)
- Emits `EventShield`

**Examples:**
```bash
# Shield LIGHT
hikarid tx privacy shield 100ulight hikari1abc...stealth --from alice

# Shield PHOTON (if enabled via governance)
hikarid tx privacy shield 1000000uphoton hikari1abc...stealth --from alice

# Shield IBC token (if enabled)
hikarid tx privacy shield 500ibc/27394FB... hikari1abc...stealth --from alice
```

### MsgPrivateTransfer

Transfers funds privately within the pool.

**Protobuf Definition (Phase 1):**
```protobuf
message MsgPrivateTransfer {
  option (cosmos.msg.v1.signer) = "sender";

  string sender = 1;                          // Fee payer (not revealed as actual sender)
  uint64 input_deposit_index = 2;             // ⚠️ VISIBLE in Phase 1
  Nullifier nullifier = 3;                    // Key image to prevent double-spend
  repeated PrivateDepositOutput outputs = 4;  // New deposits (recipient + change)
  bytes spend_signature = 5;                  // ECDSA signature proving ownership
}
```

**Protobuf Definition (Phase 2):**
```protobuf
message MsgPrivateTransfer {
  option (cosmos.msg.v1.signer) = "sender";

  string sender = 1;                          // Fee payer only
  bytes merkle_root = 2;                      // Anchor to tree state
  repeated Nullifier nullifiers = 3;          // Can spend multiple inputs
  repeated Commitment output_commitments = 4; // New note commitments
  repeated bytes encrypted_notes = 5;         // Encrypted for recipients
  ZKProof proof = 6;                          // zk-SNARK proving validity
}
```

**Validation (Phase 1):**
1. Input deposit exists and not already spent
2. Nullifier has not been used
3. Spend signature is valid (proves ownership of one-time key)
4. Commitment balance: `C_in = sum(C_out)`

**Validation (Phase 2):**
1. Merkle root matches current tree root
2. Nullifiers have not been used
3. **ZK-SNARK proof verifies**:
   - Notes exist in Merkle tree
   - Nullifiers correctly derived
   - Balance equation holds
   - Output commitments valid

**State Changes:**
- Marks nullifier(s) as used
- Adds output deposits/commitments
- Updates Merkle root (Phase 2)
- Emits `EventPrivateTransfer`

**Example:**
```bash
# Phase 1
hikarid tx privacy transfer 42 hikari1bob...stealth 60ulight --from alice

# Phase 2
hikarid tx privacy transfer hikari1bob...pubkey 60ulight --from alice
```

### MsgUnshield

Withdraws funds from private pool to public balance.

**Protobuf Definition (Phase 1):**
```protobuf
message MsgUnshield {
  option (cosmos.msg.v1.signer) = "recipient";

  string recipient = 1;                       // Public address to receive funds
  uint64 deposit_index = 2;                   // ⚠️ VISIBLE in Phase 1
  Nullifier nullifier = 3;                    // Key image
  bytes spend_signature = 4;                  // Proves ownership
}
```

**Protobuf Definition (Phase 2):**
```protobuf
message MsgUnshield {
  option (cosmos.msg.v1.signer) = "recipient";

  string recipient = 1;                       // Public address to receive funds
  bytes merkle_root = 2;                      // Anchor to tree state
  Nullifier nullifier = 3;                    // Prevents double-spend
  uint64 amount = 4;                          // Amount to unshield (revealed)
  ZKProof proof = 5;                          // Proves note ownership and amount
}
```

**Validation (Phase 1):**
1. Deposit exists and not already spent
2. Nullifier has not been used
3. Spend signature is valid
4. Amount is extracted from encrypted data

**Validation (Phase 2):**
1. Merkle root matches current state
2. Nullifier has not been used
3. **ZK-SNARK proof verifies**:
   - Note exists in tree
   - Amount matches public input
   - Recipient owns the note
   - Nullifier correctly derived

**State Changes:**
- Marks nullifier as used
- Mints `amount` to recipient's public balance
- Emits `EventUnshield`

**Example:**
```bash
hikarid tx privacy unshield 100ulight --from bob
```

## Events

### EventShield
```protobuf
message EventShield {
  string sender = 1;
  string amount = 2;
  uint64 deposit_index = 3;           // Phase 1 only
  bytes commitment = 4;
  uint64 block_height = 5;
}
```

### EventPrivateTransfer
```protobuf
message EventPrivateTransfer {
  uint64 input_deposit_index = 1;    // Phase 1 only
  bytes nullifier = 2;
  repeated uint64 output_indices = 3;
  uint64 block_height = 4;
}
```

### EventUnshield
```protobuf
message EventUnshield {
  string recipient = 1;
  string amount = 2;
  uint64 deposit_index = 3;           // Phase 1 only
  bytes nullifier = 4;
  uint64 block_height = 5;
}
```

## Queries

### Deposits (Phase 1)
```
Query/Deposits
  - from_block: uint64 (optional, default: 0)
  - to_block: uint64 (optional, default: latest)
Returns: []PrivateDeposit
```

### MerkleRoot (Phase 2)
```
Query/MerkleRoot
Returns: bytes (32 bytes)
```

### MerkleProof (Phase 2)
```
Query/MerkleProof
  - leaf_index: uint64
Returns: MerkleProof (path from leaf to root)
```

### NullifierUsed
```
Query/NullifierUsed
  - nullifier: bytes
Returns: bool
```

### Balance (Client-side)
```
Query/Balance
  - view_key: string
  - spend_key: string
Returns: []Note (scanned and decrypted notes)
```

### Params
```
Query/Params
Returns: Params
```

## Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | true | Enable/disable privacy module globally |
| `allowed_denoms` | []string | ["ulight"] | Denominations allowed for private deposits (governance-controlled) |
| `min_shield_amounts` | map[string]string | {"ulight": "1"} | Minimum shield amount per denom |
| `max_outputs` | uint32 | 16 | Maximum outputs per transfer |
| `tree_depth` | uint32 | 32 | Merkle tree depth (Phase 2 only) |

### Governance-Controlled Denominations

The `allowed_denoms` parameter controls which denominations can be shielded:

**Adding a new denomination:**
```bash
# Governance proposal to enable PHOTON privacy
hikarid tx gov submit-proposal param-change proposal.json

# proposal.json
{
  "title": "Enable PHOTON Privacy Deposits",
  "description": "Allow uphoton to be shielded in privacy module",
  "changes": [{
    "subspace": "privacy",
    "key": "allowed_denoms",
    "value": ["ulight", "uphoton"]
  }, {
    "subspace": "privacy",
    "key": "min_shield_amounts",
    "value": {"ulight": "1", "uphoton": "1000000"}
  }]
}
```

**Disabling a denomination:**
```bash
# Remove denomination from allowed list
# Existing deposits remain, but new shields are blocked
```

**IBC Token Support:**
IBC tokens can be enabled using their IBC denom hash:
```json
{
  "allowed_denoms": [
    "ulight",
    "uphoton",
    "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
  ]
}
```

## Client

### CLI

**Shield coins:**
```bash
hikarid tx privacy shield <amount> [stealth-address] [flags]
  --from string    Account to shield from
```

**Private transfer:**
```bash
hikarid tx privacy transfer <recipient-pubkey> <amount> [flags]
  --from string    Account paying fees
```

**Unshield coins:**
```bash
hikarid tx privacy unshield <amount> [flags]
  --from string    Recipient account
```

**Generate keys:**
```bash
hikarid keys privacy generate [flags]
  --name string    Key name
```

**Scan for notes:**
```bash
hikarid query privacy scan [flags]
  --view-key string
  --spend-key string
  --from-block uint64
  --to-block uint64
```

**Check balance:**
```bash
hikarid query privacy balance [flags]
  --view-key string
  --spend-key string
```

### gRPC

- `Query/Deposits`: List deposits in block range (Phase 1)
- `Query/MerkleRoot`: Get current Merkle root (Phase 2)
- `Query/MerkleProof`: Get Merkle proof for leaf (Phase 2)
- `Query/NullifierUsed`: Check if nullifier has been used
- `Query/Params`: Get module parameters

### REST

Endpoints mirror the gRPC queries:

- `/hikari/privacy/v1/deposits?from_block=0&to_block=1000`
- `/hikari/privacy/v1/merkle_root`
- `/hikari/privacy/v1/merkle_proof/{leaf_index}`
- `/hikari/privacy/v1/nullifier_used/{nullifier}`
- `/hikari/privacy/v1/params`

## Implementation Phases

### Phase 1: Testnet Only (4-6 weeks)
- ✅ Stealth addresses with ECDH
- ✅ Pedersen commitments for amount hiding
- ✅ Simple nullifiers (key images)
- ✅ ECDSA signatures for authorization
- ⚠️ Deposit indices visible (transaction graph exposed)
- **Purpose**: Architecture validation, UX testing, no real funds

### Phase 2: Mainnet Production (12-16 weeks)
- ✅ Merkle tree for commitment storage
- ✅ zk-SNARK proofs (Groth16 or PLONK)
- ✅ Full unlinkability (hidden transaction graph)
- ✅ Constant-time verification (~5ms)
- ✅ Large anonymity sets (all notes in tree)
- **Purpose**: Production privacy with real user funds

**See [ADR-HIKARI-001 Implementation Phases](../../docs/architecture/adr-hikari-001-privacy-deposits.md#implementation-phases) for detailed timeline.**

## References

- [ADR-HIKARI-001: Privacy Deposits](../../docs/architecture/adr-hikari-001-privacy-deposits.md) - Complete specification
- [Phase 1 Sequence Diagrams](../../docs/architecture/diagrams/privacy-phase1/) - Testnet flow visualization
- [Phase 2 Sequence Diagrams](../../docs/architecture/diagrams/privacy-phase2/) - Mainnet flow visualization
- [Monero Stealth Addresses](https://www.getmonero.org/resources/moneropedia/stealthaddress.html) - Stealth address reference
- [Zcash Sapling Protocol](https://zips.z.cash/protocol/protocol.pdf) - zk-SNARK inspiration
- [Tornado Cash](https://tornado.cash/) - Similar architecture (Phase 1 style)
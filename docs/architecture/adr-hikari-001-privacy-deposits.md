# ADR-HIKARI-001: Privacy Deposits with Incremental Implementation

## Changelog

- 2025-01-16: Initial draft

## Status

DRAFT Not Implemented

## Abstract

This ADR proposes the introduction of privacy-preserving transaction capabilities to Hikari Chain through a testnet-first development approach. The feature enables users to shield their transactions from public observation while maintaining the security and verifiability of the blockchain.

Phase 1 provides a testnet implementation with stealth addresses and Pedersen commitments for architecture validation (4-6 weeks development, testnet only). Phase 2 delivers the production implementation with zk-SNARKs for full transaction unlinkability (12-16 weeks development, mainnet deployment). Phase 1 serves purely as a proof-of-concept on testnet and will not handle real user funds.

## Context

Hikari Chain is positioned as a privacy-focused sidechain within the AtomOne ecosystem. While the Cosmos SDK and IBC protocol provide excellent interoperability and performance, they lack native privacy features. All transactions on standard Cosmos chains are fully transparent:

- Transaction amounts are visible to everyone
- Sender and recipient addresses are public
- Transaction graphs can be analyzed to trace fund flows
- This transparency conflicts with legitimate privacy needs for individuals and businesses

### Privacy Requirements

Users need privacy for legitimate reasons:
1. **Personal Privacy**: Individuals don't want their financial activities publicly observable
2. **Business Confidentiality**: Companies need to hide sensitive transaction details from competitors
3. **Security**: Public wealth visibility creates security risks and makes users targets
4. **Fungibility**: Privacy is necessary for true fungibility of digital assets

### Technical Challenge

Implementing privacy in a decentralized blockchain requires solving several challenges:
- Hiding transaction details while allowing validators to verify correctness
- Preventing double-spending without revealing which coins are spent
- Maintaining compatibility with existing Cosmos SDK modules and IBC
- Balancing privacy with performance and user experience

### Testnet-First Approach Rationale

Rather than attempting a full zk-SNARK implementation immediately, we propose a testnet-first development approach:

**Phase 1 (Testnet Only - Stealth Privacy)** validates the architecture:
- Proves the core architecture and user experience with test tokens
- Validates demand and collects user feedback
- Tests client-side scanning and key management
- Lower technical risk for initial validation
- Does not require trusted setup ceremony
- **No real user funds - testnet only**

**Phase 2 (Mainnet - Zero-Knowledge Privacy)** delivers production privacy:
- Builds on validated Phase 1 architecture
- Addresses Phase 1 limitations (transaction graph visibility)
- Leverages mature zk-SNARK libraries and tooling
- Benefits from lessons learned in testnet
- **Only version to handle real user funds**

This approach allows us to validate the architecture and UX quickly on testnet before investing in the complex zk-SNARK implementation for mainnet. Users never risk real funds with the weaker Phase 1 privacy model.

## Decision

We will implement privacy deposits through a new `x/privacy` module developed in two phases:

**Phase 1 (Testnet)**: Deploy basic privacy implementation for architecture validation
**Phase 2 (Mainnet)**: Deploy production-ready zk-SNARK implementation

Both phases use the same `x/privacy` module name and share core architecture, but Phase 2 is a complete rewrite with enhanced cryptographic proofs. There is no migration path between phases because Phase 1 never handles real user funds.

### Core Architecture (Both Phases)

```
┌─────────────────────────────────────────────────────────┐
│                    PUBLIC ACCOUNTS                       │
│  hikari1alice: 1000 LIGHT                               │
│  hikari1bob:   500 LIGHT                                │
└────────────────┬────────────────────────────────────────┘
                 │ Shield (MsgShield)
                 ↓
┌─────────────────────────────────────────────────────────┐
│              GLOBAL PRIVATE POOL (State)                │
│                                                          │
│  Deposits (commitments/notes)                           │
│  Nullifiers (spent tracking)                            │
│                                                          │
└────────────────┬────────────────────────────────────────┘
                 │ Unshield (MsgUnshield)
                 ↓
┌─────────────────────────────────────────────────────────┐
│                    PUBLIC ACCOUNTS                       │
│  hikari1alice: 500 LIGHT                                │
│  hikari1bob:   800 LIGHT                                │
└─────────────────────────────────────────────────────────┘
```

### Common Components

Both phases share these fundamental components:

1. **Stealth Addresses**: One-time addresses derived using ECDH for recipient privacy
2. **Pedersen Commitments**: Hide transaction amounts while allowing verification
3. **Nullifiers**: Prevent double-spending without revealing which deposit is spent
4. **Global Pool**: All private deposits in one shared pool for maximum anonymity set

### Three Core Operations

Both phases implement the same user-facing operations:

1. **Shield**: Move coins from public balance to private pool
2. **Private Transfer**: Transfer within private pool (hidden amounts and recipients)
3. **Unshield**: Move coins from private pool back to public balance

### Phase 1: Stealth Address Privacy (Testnet Only)

**Timeline**: 4-6 weeks
**Module**: `x/privacy`
**Privacy Level**: Medium
**Deployment**: Testnet only - for validation and testing

#### What Phase 1 Provides:

✅ **Hidden Recipients**: Stealth addresses via ECDH
✅ **Hidden Amounts**: Pedersen commitments
✅ **No Viewing Keys Required**: Self-scanning via stealth address derivation
✅ **Double-Spend Prevention**: Cryptographic nullifiers

#### What Phase 1 Does NOT Provide:

❌ **Transaction Graph Privacy**: Deposit indices visible, can trace connections
❌ **Hidden Inputs**: Can see which deposit is being spent
❌ **Ring Signatures**: No decoy inputs

#### State Design (Phase 1):

```protobuf
message OneTimeAddress {
  ECPoint public_key = 1;    // P = Hs(r*V)*G + S
  ECPoint tx_public_key = 2; // R = r*G
}

message PedersenCommitment {
  ECPoint point = 1; // C = amount*H + blinding*G
}

message PrivateDeposit {
  OneTimeAddress one_time_address = 1;
  PedersenCommitment commitment = 2;
  bytes encrypted_amount = 3;
  uint64 block_height = 4;
  string denom = 5; // "ulight"
}

message Nullifier {
  ECPoint key_image = 1; // I = x*Hp(P)
}
```

#### Message Types (Phase 1):

```protobuf
// Deposit coins to private pool
message MsgShield {
  string sender = 1;
  cosmos.base.v1beta1.Coin amount = 2;
  OneTimeAddress stealth_address = 3;
  PedersenCommitment commitment = 4;
  bytes encrypted_amount = 5;
}

// Transfer within private pool
message MsgPrivateTransfer {
  string sender = 1;                      // Fee payer
  uint64 input_deposit_index = 2;         // Which deposit to spend
  Nullifier nullifier = 3;                // Prevent double-spend
  repeated PrivateDepositOutput outputs = 4;
  bytes spend_signature = 5;              // Proves ownership
}

// Withdraw from private pool
message MsgUnshield {
  string recipient = 1;
  uint64 deposit_index = 2;
  Nullifier nullifier = 3;
  bytes spend_signature = 4;
}
```

#### Validation Logic (Phase 1):

Validators verify:
1. ✅ Sender has sufficient balance (Shield)
2. ✅ Stealth address is on valid elliptic curve
3. ✅ Nullifier has not been used before
4. ✅ Spend signature is valid (proves ownership)
5. ✅ Commitment balance: `C_in = C_out1 + C_out2 + ...`

**Note**: In Phase 1, deposit indices are visible, so transaction graph analysis is possible.

#### Privacy Analysis (Phase 1):

**What validators/observers see:**
- ❌ Shield: Sender address visible (entry point)
- ⚠️ Transfer: Input deposit index visible (can track which deposit is spent)
- ✅ Transfer: Output amounts hidden in commitments
- ✅ Transfer: Recipient stealth addresses (unlinkable to public addresses)
- ❌ Unshield: Recipient address visible (exit point)

**Attack vectors:**
- Transaction graph analysis (deposit #42 → deposits #100, #101)
- Timing analysis (shield followed immediately by transfer)
- Amount correlation (unique amounts can be tracked)
- Entry/exit point deanonymization

**Anonymity set size**: Limited by timing and amounts

### Phase 2: Zero-Knowledge Privacy (Mainnet Production)

**Timeline**: 12-16 weeks after Phase 1 testnet validation
**Module**: `x/privacy` (rebuilt with zk-SNARK support)
**Privacy Level**: Maximum
**Deployment**: Mainnet - production deployment with real user funds

#### What Phase 2 Adds:

✅ **Hidden Transaction Graph**: No one knows which deposit is spent
✅ **Unlinkable Transactions**: Cannot trace fund flows
✅ **Large Anonymity Sets**: All deposits are potential sources (not just recent ones)
✅ **Constant-Size Proofs**: ~200-300 bytes regardless of complexity
✅ **Efficient Verification**: ~5ms verification time (constant)

#### Key Architectural Changes (Phase 2):

1. **Merkle Tree Instead of Linear Array**:
   ```
   Phase 1: deposits[0], deposits[1], deposits[2], ... (index-based)
   Phase 2: Merkle tree of commitments (membership proofs)
   ```

2. **ZK-SNARK Proofs Replace Simple Signatures**:
   - Prove: "I know a valid note in the tree"
   - Prove: "Balance equation holds"
   - Without revealing: Which note or any amounts

3. **Updated Message Types**:
   ```protobuf
   message MsgPrivateTransfer {
     string sender = 1;                // Fee payer only
     bytes merkle_root = 2;            // Tree state anchor
     repeated Nullifier nullifiers = 3;
     repeated Commitment output_commitments = 4;
     repeated bytes encrypted_notes = 5;
     ZKProof proof = 6;                // THE KEY ADDITION
   }
   ```

#### ZK-SNARK Circuit (Phase 2):

The circuit proves the following statement:

**Public Inputs** (visible to validators):
- Merkle root
- Nullifiers
- Output commitments

**Private Inputs** (known only to prover):
- Note details (amount, secret, recipient)
- Merkle path (proof of inclusion)
- Blinding factors

**Circuit Logic**:
```
1. commitment = Hash(amount, secret, recipient, blinding)
2. VerifyMerkleProof(commitment, merkle_path, merkle_root)
3. nullifier = Hash(secret, commitment)
4. sum(input_amounts) == sum(output_amounts)
5. Output commitments correctly formed
```

#### Technology Stack (Phase 2):

- **Proof System**: Groth16 (requires trusted setup) or PLONK (universal setup)
- **Curve**: BN254 (optimal for zk-SNARKs)
- **Library**: gnark (Go native) or arkworks (Rust, via FFI)
- **Hash Function**: Poseidon (optimized for zk-SNARKs)
- **Trusted Setup**: Multi-party computation ceremony (if Groth16)

#### Performance Characteristics:

**Phase 1**:
- Transaction size: ~1-2 KB
- Verification time: ~100ms (signature verification + commitment arithmetic)
- Proof generation: N/A
- Gas cost: ~200K-300K

**Phase 2**:
- Transaction size: ~500-800 bytes (smaller despite more privacy!)
- Verification time: ~5ms (constant, regardless of anonymity set size)
- Proof generation: ~5-10 seconds (client-side)
- Gas cost: ~500K-1M

#### Privacy Analysis (Phase 2):

**What validators/observers see:**
- ❌ Shield: Sender address visible (entry point only)
- ✅ Transfer: Completely opaque (don't know which deposit spent)
- ✅ Transfer: No transaction graph visible
- ✅ Transfer: Amounts fully hidden
- ❌ Unshield: Recipient address visible (exit point only)

**Attack vectors (significantly reduced)**:
- ✅ Transaction graph analysis: IMPOSSIBLE
- ✅ Amount correlation: IMPOSSIBLE (all in commitments)
- ⚠️ Timing analysis: Difficult but possible
- ⚠️ Entry/exit correlation: Possible (mitigate with mixing)

**Anonymity set size**: All deposits in the tree (potentially millions)

### Transition from Phase 1 to Phase 2

Since Phase 1 is testnet-only and Phase 2 is a fresh mainnet deployment, there is no migration:

1. **Testnet Phase 1**:
   - Deploy and test with community
   - Gather feedback on UX, performance, and architecture
   - Iterate based on learnings
   - All test tokens and deposits are disposable

2. **Mainnet Phase 2 Launch**:
   - Fresh deployment with zk-SNARK implementation
   - No legacy Phase 1 state to migrate
   - Clean genesis state for privacy module
   - Users start fresh with maximum privacy from day one

3. **Development Continuity**:
   - Phase 2 reuses validated architecture from Phase 1
   - Same message types (Shield/Transfer/Unshield)
   - Enhanced with Merkle tree and ZK proofs
   - Client libraries can reuse scanning and key management logic

4. **Testnet → Mainnet Process**:
   - Complete Phase 2 development
   - Deploy to Hikari testnet for final validation (4-8 weeks)
   - Security audits (circuit + implementation)
   - Mainnet launch via governance or genesis

### Module Structure

```
x/privacy/              (Same module for both phases)
├── keeper/
│   ├── keeper.go                 # Main keeper
│   ├── msg_server.go             # Message handlers
│   ├── grpc_query.go             # Queries
│   ├── deposit.go                # Deposit management (Phase 1: array, Phase 2: tree)
│   ├── nullifier.go              # Nullifier tracking
│   ├── crypto.go                 # Stealth addresses, commitments
│   └── proof_verification.go    # Phase 2: ZK proof verification
├── types/
│   ├── keys.go
│   ├── codec.go
│   ├── msgs.go
│   ├── commitment.go
│   ├── proof.go                  # Phase 2: ZK proof types
│   └── errors.go
├── circuit/                       # Phase 2 only
│   ├── circuit.go                # Circuit definition (gnark)
│   ├── setup.go                  # Trusted setup
│   └── verify.go                 # Verification logic
├── client/
│   ├── cli/
│   │   ├── tx.go
│   │   └── query.go
│   └── prover/                    # Phase 2: Client-side proving
│       ├── prover.go
│       └── note_manager.go
└── module.go
```

### CLI Examples

```bash
# Phase 1 & 2: Shield coins
hikarid tx privacy shield 100ulight [recipient-stealth-addr] --from alice

# Phase 1 & 2: Private transfer
hikarid tx privacy transfer [deposit-index] [recipient-pubkey] 50ulight --from alice

# Phase 1 & 2: Unshield coins
hikarid tx privacy unshield [deposit-index] --from bob

# Phase 1 & 2: Scan for deposits
hikarid query privacy scan-deposits [view-key] [spend-key] --from-block=1000

# Phase 1 & 2: Query balance
hikarid query privacy balance [view-key]
```

Note: CLI commands remain similar between phases; the difference is in the underlying proof system.

## Consequences

### Positive

1. **User Privacy**: Users gain financial privacy for legitimate use cases
2. **Competitive Advantage**: Hikari Chain differentiates from other Cosmos chains
3. **Reduced Risk**: Phase 1 testnet validates architecture before major Phase 2 investment
4. **Fast Validation**: Phase 1 proves concept in 4-6 weeks on testnet
5. **User Feedback**: Learn from testnet usage before committing to mainnet development
6. **No Real Funds at Risk**: Phase 1 uses test tokens only
7. **Proven Architecture**: Both Tornado Cash (Phase 1-style) and Zcash (Phase 2-style) prove viability
8. **Cosmos Ecosystem**: Brings privacy to IBC, enabling private cross-chain transfers
9. **No Viewing Keys**: ECDH-based scanning is more user-friendly than Zcash-style viewing keys
10. **Clean Launch**: Mainnet starts with maximum privacy, no legacy weak privacy to support

### Negative

1. **Regulatory Risk**: Privacy features may face regulatory scrutiny
2. **Compliance Complexity**: May need optional transparency features for regulated users
3. **Phase 1 Limitations**: Testnet-only means longer wait for mainnet privacy
4. **Client Complexity**: Users must run scanning to find their deposits
5. **Phase 2 Complexity**: zk-SNARKs are complex, need specialized audits
6. **Trusted Setup Risk**: Phase 2 (if using Groth16) requires trusted ceremony
7. **Performance**: Phase 2 proof generation takes 5-10 seconds client-side
8. **State Growth**: Deposits and nullifiers accumulate forever
9. **Education Required**: Users need to understand privacy limitations and best practices
10. **Development Time**: Total time to mainnet is 16-22 weeks (4-6 testnet + 12-16 mainnet)

### Neutral

1. **Module Size**: Adds ~5,000-10,000 lines of code (Phase 1), ~20,000+ (Phase 2)
2. **Dependencies**: Requires elliptic curve libraries (Phase 1), gnark/arkworks (Phase 2)
3. **Testing Burden**: Cryptographic code requires extensive testing and audits
4. **Documentation**: Significant user education materials needed
5. **IBC Compatibility**: Private IBC transfers require both chains to support privacy
6. **Gas Costs**: Higher than standard transfers (Phase 1: 2-3x, Phase 2: 5-10x)
7. **Trusted Setup Ceremony**: Phase 2 requires community participation (if Groth16)
8. **Alternative**: Could use PLONK instead of Groth16 to avoid trusted setup

## Alternative Designs Considered

### 1. Ring Signatures (Not Chosen)

An intermediate option between Phase 1 and Phase 2 would use ring signatures to hide which deposit is spent among a small set (e.g., 10-100 decoys).

**Pros**:
- No trusted setup
- Better privacy than Phase 1
- Simpler than zk-SNARKs

**Cons**:
- Linear verification cost (O(n) in ring size)
- Larger transaction sizes (~10KB for ring of 100)
- Still linkable with sophisticated analysis
- Not a significant improvement over Phase 1

**Decision**: Skip this intermediate step and go directly from Phase 1 to Phase 2 for maximum privacy.

### 2. Account-Based Privacy (Not Chosen)

Instead of UTXO-style notes, use account-based privacy (like Zexe/Aleo).

**Pros**:
- More familiar to Cosmos developers
- Easier state management

**Cons**:
- More complex circuits
- Harder to reason about privacy guarantees
- Less battle-tested in production

**Decision**: Use UTXO-style notes (proven by Zcash, Tornado Cash).

### 3. Confidential Assets Only (Not Chosen)

Implement only Pedersen commitments for amount hiding without full privacy.

**Pros**:
- Very simple implementation
- No complex cryptography

**Cons**:
- Sender/recipient still visible
- Very weak privacy model
- Not worth the complexity

**Decision**: If we're adding privacy, go for meaningful privacy (stealth addresses minimum).

### 4. Privacy-Focused L2/Rollup (Not Chosen)

Build privacy as a separate L2 instead of integrating into L1.

**Pros**:
- Doesn't bloat L1 state
- Can iterate faster

**Cons**:
- Adds complexity (need bridge)
- Splits liquidity
- Worse UX (need to bridge funds)

**Decision**: Integrate directly into L1 for better UX, following Hikari Chain's vision as a privacy-focused sidechain.

## Implementation Phases

### Phase 1 Implementation (4-6 weeks, Testnet Only)

**Week 1-2: Core Infrastructure**
- Create `x/privacy` module structure
- Define protobuf types (OneTimeAddress, PedersenCommitment, etc.)
- Implement keeper storage functions
- Unit tests for storage

**Week 2-3: Cryptographic Primitives**
- Implement EC point operations
- Pedersen commitments (generate H base point, create/verify)
- Stealth address generation (ECDH)
- Nullifier (key image) generation
- Comprehensive crypto tests

**Week 3-4: Message Handlers**
- Implement MsgShield, MsgPrivateTransfer, MsgUnshield
- Transaction validation
- Integration tests

**Week 4-5: Client Implementation**
- Wallet functionality (key generation, pool scanning)
- CLI commands
- Scanning service
- Client tests

**Week 5-6: Testing & Documentation**
- E2E testing on testnet
- Performance optimization
- Security review
- User documentation
- Module README

**Deliverables**:
- Working Phase 1 privacy module (testnet deployment)
- CLI wallet
- Test coverage >80%
- User documentation
- Testnet user guide
- Feedback collection system
- Architecture validation report

### Phase 2 Implementation (12-16 weeks, Mainnet Deployment)

**Week 1-4: Circuit Development**
- Design circuit architecture
- Implement in gnark
- Optimize constraint count (<100K constraints)
- Circuit testing and benchmarks

**Week 4-5: Trusted Setup**
- Choose proof system (Groth16 vs PLONK)
- Perform trusted setup ceremony (if Groth16)
- Generate and publish proving/verification keys
- Verify ceremony integrity

**Week 5-9: Module Implementation**
- Implement Merkle tree management
- Implement message handlers with ZK proof verification
- Integrate proof verification in keeper
- Fresh genesis state design
- Integration tests

**Week 9-12: Client Implementation**
- Note management system
- Proof generation client
- Background scanner with bloom filters
- Updated CLI tools

**Week 12-14: Testing & Optimization**
- E2E testing with large anonymity sets
- Performance optimization
- Gas cost optimization
- Security audits (circuit + implementation)

**Week 15-16: Deployment**
- Final testnet deployment and testing
- Multi-party computation ceremony (if using Groth16)
- Monitoring setup
- Mainnet launch preparation
- User education materials

**Deliverables**:
- Production-ready zk-SNARK privacy module
- Proving keys and verification keys
- Full-featured client with prover
- Security audit reports (circuit + implementation)
- Mainnet launch documentation
- User guides and best practices

## Security Considerations

### Phase 1 Security (Testnet)

1. **Elliptic Curve**: Use secp256k1 (Bitcoin/Ethereum standard)
2. **Random Number Generation**: Use `crypto/rand` for all randomness
3. **Hash Functions**: SHA-256 for commitments and key derivation
4. **Nullifier Database**: Must be persistent, never deleted
5. **Known Limitations**: Transaction graph visible, entry/exit points visible
6. **Testnet Only**: No real funds at risk, focus on architecture validation

**Validation Goals**:
- Verify cryptographic primitives work correctly
- Test client-side scanning performance
- Validate user experience and key management
- Identify potential issues before Phase 2 development

### Phase 2 Security

1. **Trusted Setup** (if Groth16):
   - Multi-party computation required
   - Need at least one honest participant
   - Alternative: Use PLONK (no trusted setup)

2. **Circuit Soundness**:
   - Formal verification recommended
   - Multiple independent audits required
   - Bug bounty program
   - Open source for community review

3. **Nullifier Collision**:
   - Use cryptographic hash (Poseidon)
   - Collision probability: 2^-256 (negligible)

4. **Merkle Tree Security**:
   - All nodes must compute tree identically
   - Deterministic root computation
   - State sync must include full tree

5. **Proof Verification**:
   - Must verify all public inputs match state
   - Check nullifiers not reused
   - Verify cryptographic proof
   - Charge appropriate gas to prevent DoS

**Security Audits Required**:
- Phase 1: Internal security review only (testnet, no audit needed)
- Phase 2: Full professional audit required:
  - ZK circuit audit (~3-4 weeks)
  - Module implementation audit (~2-3 weeks)
  - Cryptography review (~1-2 weeks)
  - Total: 6-9 weeks before mainnet launch

### Regulatory Considerations

1. **AML/KYC Compliance**:
   - Consider optional view-only audit keys
   - Document compliance capabilities
   - Consult legal counsel before deployment

2. **Auditability**:
   - Entry/exit points still visible (Shield/Unshield)
   - Consider adding optional transparency features
   - Enable regulatory reporting if required

3. **Best Practices**:
   - Clear terms of service
   - User education on legal use
   - Cooperation with law enforcement when legally required
   - Monitor for abuse patterns

## References

### Technical References

- [Zcash Protocol Specification](https://zips.z.cash/protocol/protocol.pdf) - Phase 2 inspiration
- [Tornado Cash Whitepaper](https://tornado.cash/audits/TornadoCash_whitepaper_v1.4.pdf) - Phase 1 inspiration
- [Monero Stealth Addresses](https://www.getmonero.org/resources/moneropedia/stealthaddress.html) - Stealth address implementation
- [Pedersen Commitments](https://crypto.stanford.edu/~dabo/pubs/papers/OnDlogBased.pdf) - Commitment scheme
- [Groth16 ZK-SNARKs](https://eprint.iacr.org/2016/260.pdf) - Phase 2 proof system
- [PLONK Universal SNARKs](https://eprint.iacr.org/2019/953.pdf) - Alternative proof system
- [gnark - Go ZK-SNARK Library](https://github.com/ConsenSys/gnark) - Phase 2 implementation library
- [Poseidon Hash](https://eprint.iacr.org/2019/458.pdf) - ZK-friendly hash function

### Implementation References

- Zcash Sapling (Phase 2 architecture)
- Tornado Cash (Phase 1 architecture)
- Aztec Protocol (account-based ZK privacy)
- Penumbra (Cosmos SDK privacy, similar goals)

### Related ADRs

- ADR-002: Photon Token - Fee token economics that will apply to privacy transactions
- ADR-003: Governance Proposal Deposit Auto Throttler - Governance that will control privacy parameters

### Related Issues

- (To be added when implementation begins)

---

**Note**: This ADR will be updated as implementation progresses and as we learn from Phase 1 deployment.

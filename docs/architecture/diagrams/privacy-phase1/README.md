# Privacy Phase 1 Sequence Diagrams

This directory contains Mermaid sequence diagrams for the Phase 1 (Stealth Address + Commitments) implementation of the privacy module.

## Diagrams

1. **shield-operation.mmd** - Shield operation (deposit to private pool)
2. **private-transfer-operation.mmd** - Private transfer with simple signature
3. **unshield-operation.mmd** - Unshield operation (withdraw to public)
4. **complete-user-journey.mmd** - Complete Alice → Bob transfer showing privacy limitations

## Key Differences from Phase 2

Phase 1 is **testnet-only** and serves as architecture validation before Phase 2 mainnet deployment.

### Privacy Level Comparison

| Feature | Phase 1 | Phase 2 |
|---------|---------|---------|
| **Deposit Index** | ❌ Visible | ✅ Hidden |
| **Transaction Graph** | ❌ Visible (linkable) | ✅ Hidden (unlinkable) |
| **Amounts** | ✅ Hidden (commitments) | ✅ Hidden (commitments) |
| **Recipients** | ✅ Hidden (stealth addr) | ✅ Hidden (stealth addr) |
| **Proof Type** | Simple ECDSA signature | ZK-SNARK |
| **Verification Time** | ~100ms | ~5ms |
| **Anonymity Set** | Limited (timing-based) | Large (all notes in tree) |

### Phase 1 Limitations Highlighted

The diagrams clearly show:
- 🔴 **Red boxes**: Warnings about visible information
- ⚠️ **Yellow boxes**: Privacy limitations
- 🔗 **Transaction graph**: Can trace deposit #42 → #100 → Bob

### Why Testnet Only?

Phase 1's visible transaction graph makes it unsuitable for mainnet privacy. It serves to:
- ✅ Validate core architecture (stealth addresses, commitments, nullifiers)
- ✅ Test client-side scanning and key management
- ✅ Gather user feedback on UX
- ✅ Identify issues before Phase 2 development
- ❌ NOT suitable for protecting real user funds

## Viewing the Diagrams

### Option 1: GitHub (Automatic Rendering)
GitHub automatically renders `.mmd` files. Simply view them in the GitHub web interface.

### Option 2: VS Code
Install the [Mermaid Preview extension](https://marketplace.visualstudio.com/items?itemName=bierner.markdown-mermaid) for VS Code.

### Option 3: Mermaid Live Editor
Copy the diagram content and paste into [Mermaid Live Editor](https://mermaid.live/)

### Option 4: Command Line
```bash
# Install mermaid-cli
npm install -g @mermaid-js/mermaid-cli

# Generate PNG from diagram
mmdc -i shield-operation.mmd -o shield-operation.png

# Generate SVG
mmdc -i shield-operation.mmd -o shield-operation.svg
```

## Related Documentation

See [ADR-HIKARI-001: Privacy Deposits](../../adr-hikari-001-privacy-deposits.md) for complete technical specification.

Compare with [Phase 2 diagrams](../privacy-phase2/) to understand the privacy improvements.

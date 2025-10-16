# Privacy Phase 2 Sequence Diagrams

This directory contains Mermaid sequence diagrams for the Phase 2 (zk-SNARK) implementation of the privacy module.

## Diagrams

1. **shield-operation.mmd** - Shield operation (deposit to private pool)
2. **private-transfer-operation.mmd** - Private transfer with ZK proof
3. **unshield-operation.mmd** - Unshield operation (withdraw to public)
4. **complete-user-journey.mmd** - Complete Alice → Bob transfer example

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

## Diagram Format

All diagrams use Mermaid's `sequenceDiagram` format. Key features:
- **Participants**: Alice, Bob, Client, Validator, x/privacy module
- **Color coding**:
  - Blue boxes: Client-side operations (proof generation)
  - Green boxes: Validator verification
  - Red boxes: Observer limitations
- **Timing notes**: Proof generation (~5-10s), verification (~5ms)

## Related Documentation

See [ADR-HIKARI-001: Privacy Deposits](../../adr-hikari-001-privacy-deposits.md) for complete technical specification.

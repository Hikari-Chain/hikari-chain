# Hikari Chain

Hikari Chain is built as a fork of [AtomOne](https://github.com/atomone-hub/atomone),
which itself is built using the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk) as
a fork of the [Cosmos Hub](https://github.com/cosmos/gaia) at version
[v15.2.0](https://github.com/cosmos/gaia/releases/tag/v15.2.0).

The following modifications were inherited from AtomOne:

1. Removed x/globalfee module and revert to older and simpler fee decorator
2. Removed Packet Forwarding Middleware
3. Removed Interchain Security module
4. Reverted to standard Cosmos SDK v0.47.10 without the Liquid Staking Module (LSM)
5. Changed Bech32 prefixes to `hikari` (see `cmd/hikarid/cmd/config.go`)
6. Removed ability for validators to vote on proposals with delegations, they can
   only use their own stake

## Genesis file

The proposed genesis files for Hikari Chain can be found in the [genesis repo](https://github.com/Hikari-Chain/genesis).

## Public RPC and fullnode endpoints

The public RPC and fullnode endpoints directory can be found in the [config repo](https://github.com/Hikari-Chain/config).

## Reproducible builds

An effort has been made to make it possible to build the exact same binary
locally as the Github Release section. To do this:

- Checkout to the expected released version
- Run `make build` (which will output the binary to the `build` directory) or
`make install`. Note that a fixed version of the `go` binary is required,
follow the command instructions to install this specific version if needed.
- The resulted binary should have the same sha256 hash than the one from the
Github Release section.

## Ledger support

Run `make build/install LEDGER_ENABLED=true` to have ledger support in
`hikarid` binary.

Note that this will disable reproducible builds, as it introduces OS
dependencies.

## Acknowledgements

Hikari Chain is a fork of [AtomOne](https://github.com/atomone-hub/atomone).

Portions of this codebase are copied or adapted from
[atomone-hub/atomone](https://github.com/atomone-hub/atomone),
[cosmos/gaia@v15](https://github.com/cosmos/gaia/tree/v15.0.0),
[cosmos/cosmos-sdk@v47.10](https://github.com/cosmos/cosmos-sdk/tree/v0.47.10)
and [skip-mev/feemarket@v1.1.1](https://github.com/skip-mev/feemarket/tree/v1.1.1).

Their original licenses are included in [ATTRIBUTION](ATTRIBUTION)

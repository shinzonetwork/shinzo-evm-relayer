# Shinzo EVM Relayer

A production-grade relayer that keeps ShinzoHub in sync with events from the Shinzo Outpost contract on Ethereum (or any EVM-compatible chain). It runs two independent pipelines:

- **Payment pipeline** — subscribes to `PaymentCreated` log events and broadcasts `MsgRequestStreamAccess` to ShinzoHub.
- **Attestation pipeline** — polls block `extraData` headers, resolves on-chain attestations, verifies signatures, and broadcasts `MsgIndexerAttestation` to ShinzoHub.

## Requirements

- Go 1.24+
- Running EVM JSON-RPC node (e.g. `http://127.0.0.1:8545`)
- Running ShinzoHub node with:
  - Comet RPC (e.g. `http://127.0.0.1:26657`)
  - gRPC enabled (e.g. `127.0.0.1:9090`)

## Build

```bash
make build
# artifact: ./build/shinzo-evm-relayer
```

## Install

```bash
make install
# installs to $GOBIN or $(go env GOPATH)/bin
```

## Configure

On first run, a default config is written to:
```
~/.shinzo-evm-relayer/config/config.toml
```

Edit it to match your setup:

```toml
[evm]
rpc         = "http://127.0.0.1:8545"
contract    = "0xYourOutpostContractHere"   # Outpost contract (payment pipeline)
start_block = 0

# Payment pipeline
payment_listen_enabled = false

# Attestation pipeline
scan_enabled    = true
scan_batch_size = 2000
confirmations   = 0
issuer          = "0xYourChallengeIssuerHere"
extradata_tag   = "0x5348"          # "SH" — 2-byte header tag in block extraData
source_chain    = "ethereum"
source_chain_id = 1

[cosmos]
grpc       = "127.0.0.1:9090"
rpc        = "http://127.0.0.1:26657"
chain_id   = "9001"
fee_denom  = "ushinzo"
gas_prices = "0.025ushinzo"

[log]
level = "info"   # debug | info | error
```

All values can also be overridden via flags or environment variables prefixed with `SHINZO_` (e.g. `SHINZO_EVM_RPC`).

## Keys

Store the relayer's signing key from a BIP-39 mnemonic file:

```bash
# mnemonic.txt — plain text file with your 24-word mnemonic
shinzo-evm-relayer keys add \
  --name relayer \
  --mnemonic-file ./mnemonic.txt \
  --hd-path "m/44'/60'/0'/0/0"
```

This writes `~/.shinzo-evm-relayer/config/key.json` (mode `0600`) and prints the derived EVM and Shinzo bech32 addresses.

## Run

```bash
shinzo-evm-relayer start

# Override individual settings without editing the config file:
shinzo-evm-relayer start --evm.rpc http://geth:8545 --log.level debug
```

## Data

Scan progress is persisted to `~/.shinzo-evm-relayer/data/scan_cursor.json` so restarts resume from the last scanned block.

## Project Layout

```
cmd/relayer/
  main.go      entry point
  root.go      root cobra command
  start.go     start subcommand — flag binding and pipeline startup
  keys.go      keys subcommand — add / manage signing key

internal/
  config/
    paths.go   home/config/data/logs directory resolution
    config.go  TOML config struct, default generation, loading
  cursor/
    cursor.go  scan-cursor persistence (next_block tracking)
  keys/
    store.go   key.json read/write
    derive.go  BIP-39 → HD → ECDSA key derivation, bech32 encoding
  evm/
    abi.go     embedded Outpost and ChallengeIssuer ABIs
    contract.go on-chain calls: DomainSeparator, FetchAttestationCore, ComputeDigest, VerifySignature
    listener.go PaymentCreated log subscription → PaymentEvent channel
    scanner.go  extraData block polling → AttestationJob channel
  cosmos/
    client.go      key.json → ethsecp256k1 TxSigner
    payment.go     PaymentSender — MsgRequestStreamAccess broadcaster
    attestation.go AttestationSender — MsgIndexerAttestation broadcaster
  app/
    app.go     App struct, NewLogger, Start() — wires both pipelines
```

## Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Compile to `./build/shinzo-evm-relayer` |
| `make install` | Install binary to `$GOBIN` |
| `make clean` | Remove build artifacts |
| `make tidy` | `go mod tidy` |
| `make fmt` | `go fmt ./...` |
| `make vet` | `go vet ./...` |
| `make test` | Run tests |
| `make release` | Cross-compile quick presets |

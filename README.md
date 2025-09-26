# Shinzo EVM Relayer

The Shinzo EVM Relayer is a lightweight service that keeps ShinzoHub in sync with events from the Shinzo Outpost contract on Ethereum (or any EVM-compatible chain).

## Requirements
- Go 1.21+ (1.22+ recommended)
- Running EVM JSON-RPC (e.g. http://127.0.0.1:8545)
- Running ShinzoHub node with:
  - Comet RPC (e.g. http://127.0.0.1:26657)
  - gRPC enabled (e.g. 127.0.0.1:9090)

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

Edit it to set:
```toml
[evm]
rpc         = "http://127.0.0.1:8545"
contract    = "0xYourOutpostContractHere"
start_block = 0

[cosmos]
grpc       = "127.0.0.1:9090"
rpc        = "http://127.0.0.1:26657"
chain_id   = "shinzohub-dev"     # or your chain id
fee_denom  = "ushinzo"
gas_prices = "0.025ushinzo"
```

## Keys

Add a key record:

```bash
# mnemonic.txt contains your 24-word mnemonic
./build/shinzo-evm-relayer keys add   --name relayer   --mnemonic-file ./mnemonic.txt   --hd-path "m/44'/60'/0'/0/0"
```

This writes `~/.shinzo-evm-relayer/config/key.json`.

## Run

```bash
./build/shinzo-evm-relayer start
```

Or if installed:

```bash
shinzo-evm-relayer start
```

## Make Targets

- `make build` – compile to `./build/`
- `make install` – install binary to `$GOBIN`
- `make clean` – remove build artifacts
- `make tidy` – `go mod tidy`
- `make fmt` – `go fmt ./...`
- `make vet` – `go vet ./...`
- `make test` – run tests
- `make release` – cross-compile quick presets

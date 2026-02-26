package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	EVM    EVMConfig    `toml:"evm"`
	Cosmos CosmosConfig `toml:"cosmos"`
	Log    LogConfig    `toml:"log"`
}

type EVMConfig struct {
	RPC        string `toml:"rpc"`
	Contract   string `toml:"contract"`
	StartBlock uint64 `toml:"start_block"`

	PaymentListenEnabled bool `toml:"payment_listen_enabled"`

	ScanEnabled   bool   `toml:"scan_enabled"`
	ScanBatchSize uint64 `toml:"scan_batch_size"`
	Confirmations uint64 `toml:"confirmations"`
	Issuer        string `toml:"issuer"`
	ExtraDataTag  string `toml:"extradata_tag"`
	SourceChain   string `toml:"source_chain"`
	SourceChainID uint64 `toml:"source_chain_id"`
}

type CosmosConfig struct {
	GRPC      string `toml:"grpc"`
	RPC       string `toml:"rpc"`
	ChainID   string `toml:"chain_id"`
	FeeDenom  string `toml:"fee_denom"`
	GasPrices string `toml:"gas_prices"`
}

type LogConfig struct {
	Level string `toml:"level"`
}

const defaultConfig = `
#######################################################################
#                Shinzo EVM Relayer Configuration File
#######################################################################

[evm]
rpc = "http://127.0.0.1:8545"
contract = "0xYourOutpostContractHere"
start_block = 0

payment_listen_enabled = false

scan_enabled = true
scan_batch_size = 2000
confirmations = 0

issuer = "0x56e3552F0b6F5Cb971b4bFE51d572237059ed42A"
extradata_tag = "0x5348"
source_chain = "ethereum"
source_chain_id = 1

[cosmos]
grpc = "127.0.0.1:9090"
rpc = "http://127.0.0.1:26657"
chain_id = "9001"
fee_denom = "ushinzo"
gas_prices = "0.025ushinzo"

[log]
level = "info"
`

func Ensure(paths Paths) (bool, error) {
	if _, err := os.Stat(paths.ConfigFile); os.IsNotExist(err) {
		if err := os.WriteFile(paths.ConfigFile, []byte(defaultConfig), 0o644); err != nil {
			return false, fmt.Errorf("write default config: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func Load(paths Paths) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(paths.ConfigFile, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config.toml: %w", err)
	}
	return cfg, nil
}

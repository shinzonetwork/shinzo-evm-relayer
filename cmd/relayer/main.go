package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/go-bip39"

	"github.com/btcsuite/btcutil/bech32"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"

	"crypto/ecdsa"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	shinzosdk "github.com/shinzonetwork/shinzohub/sdk"
	sourcehubtypes "github.com/shinzonetwork/shinzohub/x/sourcehub/types"
)

type Config struct {
	EVM struct {
		RPC        string `toml:"rpc"`
		Contract   string `toml:"contract"`
		StartBlock uint64 `toml:"start_block"`
	} `toml:"evm"`

	Cosmos struct {
		GRPC      string `toml:"grpc"`
		RPC       string `toml:"rpc"`
		ChainID   string `toml:"chain_id"`
		FeeDenom  string `toml:"fee_denom"`
		GasPrices string `toml:"gas_prices"`
	} `toml:"cosmos"`

	Log struct {
		Level string `toml:"level"`
	} `toml:"log"`
}

const defaultConfigTOML = `
#######################################################################
#                Shinzo EVM Relayer Configuration File
#
# First run generates this file at ~/.shinzo-evm-relayer/config/config.toml
#######################################################################

[evm]
rpc = "http://127.0.0.1:8545"
contract = "0xYourOutpostContractHere"
start_block = 0

[cosmos]
grpc = "127.0.0.1:9090"
rpc = "http://127.0.0.1:26657"
chain_id = "9001"
fee_denom = "ushinzo"
gas_prices = "0.025ushinzo"

[log]
level = "info"
`

type Paths struct {
	HomeDir    string
	ConfigDir  string
	DataDir    string
	LogsDir    string
	ConfigFile string
}

func resolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("failed to get home dir: %v", err)
	}
	root := filepath.Join(home, ".shinzo-evm-relayer")
	return Paths{
		HomeDir:    root,
		ConfigDir:  filepath.Join(root, "config"),
		DataDir:    filepath.Join(root, "data"),
		LogsDir:    filepath.Join(root, "logs"),
		ConfigFile: filepath.Join(root, "config", "config.toml"),
	}, nil
}

func ensureDirs(paths Paths) error {
	for _, d := range []string{paths.HomeDir, paths.ConfigDir, paths.DataDir, paths.LogsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", d, err)
		}
	}
	return nil
}

func ensureConfig(paths Paths) (bool, error) {
	if _, err := os.Stat(paths.ConfigFile); os.IsNotExist(err) {
		if err := os.WriteFile(paths.ConfigFile, []byte(defaultConfigTOML), 0o644); err != nil {
			return false, fmt.Errorf("failed to write default config: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func loadConfig(paths Paths) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(paths.ConfigFile, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config.toml: %w", err)
	}
	return cfg, nil
}

type RelayerApp struct {
	Config Config
	Paths  Paths
	Logger *log.Logger
}

func InitLogger(level string) *log.Logger {
	return log.NewWithOptions(os.Stdout, log.Options{
		ReportTimestamp: true,
		Prefix:          "shinzo-evm-relayer",
		Level:           parseLevel(level),
	})
}

func parseLevel(level string) log.Level {
	switch level {
	case "debug":
		return log.DebugLevel
	case "error":
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}

const envPrefix = "SHINZO"

func bindFlag(cmd *cobra.Command, name string, def interface{}) {
	switch v := def.(type) {
	case string:
		cmd.Flags().String(name, v, fmt.Sprintf("%s config", name))
	case int:
		cmd.Flags().Int(name, v, fmt.Sprintf("%s config", name))
	case uint64:
		cmd.Flags().Uint64(name, v, fmt.Sprintf("%s config", name))
	case bool:
		cmd.Flags().Bool(name, v, fmt.Sprintf("%s config", name))
	default:
		panic(fmt.Sprintf("bindFlag: unsupported default type for %q: %T", name, def))
	}

	_ = viper.BindPFlag(name, cmd.Flags().Lookup(name))
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "shinzo-evm-relayer",
		Short: "Relayer for Shinzo Outpost contract -> ShinzoHub",
	}
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newKeysCmd())
	return rootCmd
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Shinzo EVM Relayer",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := resolvePaths()
			if err != nil {
				return err
			}
			if err := ensureDirs(paths); err != nil {
				return err
			}
			configCreated, err := ensureConfig(paths)
			if err != nil {
				return err
			}
			if configCreated {
				fmt.Println("Created default config at", paths.ConfigFile)
			}
			fileCfg, err := loadConfig(paths)
			if err != nil {
				return err
			}
			cfg := fileCfg

			// overrides (env/flags)
			if v := viper.GetString("evm.rpc"); v != "" {
				cfg.EVM.RPC = v
			}
			if v := viper.GetString("evm.contract"); v != "" {
				cfg.EVM.Contract = v
			}
			if v := viper.GetUint64("evm.start_block"); v != 0 {
				cfg.EVM.StartBlock = v
			}
			if v := viper.GetString("cosmos.grpc"); v != "" {
				cfg.Cosmos.GRPC = v
			}
			if v := viper.GetString("cosmos.rpc"); v != "" {
				cfg.Cosmos.RPC = v
			}
			if v := viper.GetString("cosmos.chain_id"); v != "" {
				cfg.Cosmos.ChainID = v
			}
			if v := viper.GetString("cosmos.fee_denom"); v != "" {
				cfg.Cosmos.FeeDenom = v
			}
			if v := viper.GetString("cosmos.gas_prices"); v != "" {
				cfg.Cosmos.GasPrices = v
			}
			if v := viper.GetString("log.level"); v != "" {
				cfg.Log.Level = v
			}

			logger := InitLogger(cfg.Log.Level)
			app := RelayerApp{Config: cfg, Paths: paths, Logger: logger}
			return app.Start()
		},
	}

	bindFlag(cmd, "evm.rpc", "http://127.0.0.1:8545")
	bindFlag(cmd, "evm.contract", "")
	bindFlag(cmd, "evm.start_block", 0)

	bindFlag(cmd, "cosmos.grpc", "127.0.0.1:9090")
	bindFlag(cmd, "cosmos.rpc", "http://127.0.0.1:26657")
	bindFlag(cmd, "cosmos.chain_id", "9001")
	bindFlag(cmd, "cosmos.fee_denom", "ushinzo")
	bindFlag(cmd, "cosmos.gas_prices", "0.025ushinzo")

	bindFlag(cmd, "log.level", "info")
	return cmd
}

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage relayer keys (Hermes-style)",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "add",
			Short: "Add a new key from a mnemonic file",
			RunE: func(cmd *cobra.Command, args []string) error {
				name, _ := cmd.Flags().GetString("name")
				mnemonicFile, _ := cmd.Flags().GetString("mnemonic-file")
				hdPath, _ := cmd.Flags().GetString("hd-path")

				if name == "" {
					return fmt.Errorf("--name is required")
				}
				if mnemonicFile == "" {
					return fmt.Errorf("--mnemonic-file is required")
				}

				bz, err := os.ReadFile(mnemonicFile)
				if err != nil {
					return fmt.Errorf("failed to read mnemonic file: %w", err)
				}
				mnemonic := strings.TrimSpace(string(bz))
				if !bip39.IsMnemonicValid(mnemonic) {
					return fmt.Errorf("invalid mnemonic")
				}

				paths, _ := resolvePaths()
				if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
					return err
				}
				keyFile := filepath.Join(paths.ConfigDir, "key.json")

				rec := map[string]string{
					"name":     name,
					"mnemonic": mnemonic,
					"hd_path":  hdPath,
				}
				data, _ := json.MarshalIndent(rec, "", "  ")
				if err := os.WriteFile(keyFile, data, 0o600); err != nil {
					return fmt.Errorf("failed to save key: %w", err)
				}

				evm, shinzo, _ := previewAddresses(mnemonic, hdPath)
				fmt.Println("Key saved for relayer:", name)
				fmt.Println("EVM:", evm)
				fmt.Println("SHINZO:", shinzo)
				return nil
			},
		},
	)

	cmd.PersistentFlags().String("name", "", "Name of the key")
	cmd.PersistentFlags().String("mnemonic-file", "", "Path to a file containing mnemonic")
	cmd.PersistentFlags().String("hd-path", "m/44'/60'/0'/0/0", "HD derivation path (Ethermint)")

	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func (app *RelayerApp) Start() error {
	app.Logger.Info("Relayer setup done, starting processes")

	setBech32HRP("shinzo")

	eventCh := make(chan EVMEvent, 100)
	errCh := make(chan error, 2)

	go func() {
		if err := app.runEVMListener(eventCh); err != nil {
			app.Logger.Error("EVM listener failed", "err", err)
			errCh <- fmt.Errorf("EVM listener failed: %w", err)
		}
	}()
	go func() {
		if err := app.runCosmosSender(eventCh); err != nil {
			app.Logger.Error("Cosmos sender failed", "err", err)
			errCh <- fmt.Errorf("Cosmos sender failed: %w", err)
		}
	}()

	return <-errCh
}

type EVMEvent struct {
	TxHash   string
	BlockNum uint64
	Payload  PaymentData
}

type PaymentData struct {
	Resource   uint8
	Identity   string
	StreamId   string
	Expiration *big.Int
}

func (app *RelayerApp) runEVMListener(out chan<- EVMEvent) error {
	app.Logger.Info("Starting EVM listener", "rpc", app.Config.EVM.RPC, "contract", app.Config.EVM.Contract)

	client, err := ethclient.Dial(app.Config.EVM.RPC)
	if err != nil {
		return err
	}
	parsedABI, err := abi.JSON(strings.NewReader(outpostABI))
	if err != nil {
		return err
	}
	eventSig := []byte("PaymentCreated(uint8,string,string,uint256)")
	eventHash := gethcrypto.Keccak256Hash(eventSig)

	contractAddr := common.HexToAddress(app.Config.EVM.Contract)
	start := big.NewInt(int64(app.Config.EVM.StartBlock))

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
		Topics:    [][]common.Hash{{eventHash}},
		FromBlock: start,
	}

	logsCh := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logsCh)
	if err != nil {
		return fmt.Errorf("subscription failed: %w", err)
	}

	app.Logger.Info("Subscribed to PaymentCreated events")

	go func() {
		for {
			select {
			case err := <-sub.Err():
				app.Logger.Error("subscription error", "err", err)
				return
			case vLog := <-logsCh:
				var data PaymentData
				if err := parsedABI.UnpackIntoInterface(&data, "PaymentCreated", vLog.Data); err != nil {
					app.Logger.Error("unpack error", "err", err)
					continue
				}
				out <- EVMEvent{
					TxHash:   vLog.TxHash.Hex(),
					BlockNum: vLog.BlockNumber,
					Payload:  data,
				}
			}
		}
	}()

	return nil
}

func loadRelayerKey(paths Paths) (string, string, string, error) {
	keyFile := filepath.Join(paths.ConfigDir, "key.json")
	bz, err := os.ReadFile(keyFile)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read key file: %w", err)
	}

	var rec struct {
		Name     string `json:"name"`
		Mnemonic string `json:"mnemonic"`
		HdPath   string `json:"hd_path"`
	}
	if err := json.Unmarshal(bz, &rec); err != nil {
		return "", "", "", fmt.Errorf("failed to unmarshal key file: %w", err)
	}
	return rec.Name, rec.Mnemonic, rec.HdPath, nil
}

type privKeySigner struct{ key cryptotypes.PrivKey }

func (s *privKeySigner) GetPrivateKey() cryptotypes.PrivKey { return s.key }
func (s *privKeySigner) GetAccAddress() string {
	return sdk.AccAddress(s.key.PubKey().Address().Bytes()).String()
}

func txSignerFromCosmosKey(pk cryptotypes.PrivKey) shinzosdk.TxSigner { return &privKeySigner{key: pk} }

func (app *RelayerApp) runCosmosSender(in <-chan EVMEvent) error {
	_, mnemonic, hdPath, err := loadRelayerKey(app.Paths)
	if err != nil {
		return fmt.Errorf("could not load relayer key: %w", err)
	}

	priv, evmAddr, err := derivePrivAndEVMAddr(mnemonic, hdPath)
	if err != nil {
		return fmt.Errorf("derive key: %w", err)
	}

	raw := gethcrypto.FromECDSA(priv) // 32-byte secret
	if l := len(raw); l != 32 {
		return fmt.Errorf("unexpected priv key length %d", l)
	}
	cosmosPriv := &ethsecp256k1.PrivKey{Key: raw}
	signer := txSignerFromCosmosKey(cosmosPriv)

	shinzoBech32, err := evmToBech32("shinzo", evmAddr)
	if err != nil {
		return fmt.Errorf("bech32 encode: %w", err)
	}
	app.Logger.Info("Cosmos sender ready", "address", shinzoBech32)

	hubClient, err := shinzosdk.NewClient(
		shinzosdk.WithGRPCAddr(app.Config.Cosmos.GRPC),
		shinzosdk.WithCometRPCAddr(app.Config.Cosmos.RPC),
	)
	if err != nil {
		return fmt.Errorf("init hub client: %w", err)
	}
	defer hubClient.Close()

	txBuilder, err := shinzosdk.NewTxBuilder(
		shinzosdk.WithSDKClient(hubClient),
		shinzosdk.WithChainID(app.Config.Cosmos.ChainID),
		shinzosdk.WithMinGasPrice(app.Config.Cosmos.GasPrices),
	)
	if err != nil {
		return fmt.Errorf("init tx builder: %w", err)
	}

	for ev := range in {
		app.Logger.Info("Processing event", "tx_hash", ev.TxHash, "block", ev.BlockNum)

		var resEnum sourcehubtypes.Resource
		switch ev.Payload.Resource {
		case 0:
			resEnum = sourcehubtypes.Resource_RESOURCE_PRIMITIVE
		case 1:
			resEnum = sourcehubtypes.Resource_RESOURCE_VIEW
		default:
			resEnum = sourcehubtypes.Resource_RESOURCE_VIEW
		}

		exp := uint64(0)
		if ev.Payload.Expiration != nil {
			if ev.Payload.Expiration.IsUint64() {
				exp = ev.Payload.Expiration.Uint64()
			} else {
				exp = math.MaxUint64
			}
		}

		params := shinzosdk.RequestStreamAccessParams{
			Signer:     signer,
			StreamId:   ev.Payload.StreamId,
			Identity:   ev.Payload.Identity,
			Resource:   resEnum,
			Expiration: exp,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		resp, err := shinzosdk.RequestStreamAccess(ctx, hubClient, txBuilder, params)
		cancel()
		if err != nil {
			app.Logger.Error("MsgRequestStreamAccess failed", "err", err, "stream", ev.Payload.StreamId)
			continue
		}

		app.Logger.Info("Stream access requested",
			"height", resp.Height,
			"hash", resp.TxHash,
			"resource", ev.Payload.Resource,
			"did", ev.Payload.Identity,
			"stream", ev.Payload.StreamId,
			"expiration", exp,
		)
	}

	return nil
}

func derivePrivAndEVMAddr(mnemonic, hdPath string) (*ecdsa.PrivateKey, common.Address, error) {
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, common.Address{}, err
	}
	path, err := hdwallet.ParseDerivationPath(hdPath)
	if err != nil {
		return nil, common.Address{}, err
	}
	acct, err := wallet.Derive(path, false)
	if err != nil {
		return nil, common.Address{}, err
	}
	priv, err := wallet.PrivateKey(acct)
	if err != nil {
		return nil, common.Address{}, err
	}
	addr := gethcrypto.PubkeyToAddress(priv.PublicKey)
	return priv, addr, nil
}

func previewAddresses(mnemonic, hdPath string) (string, string, error) {
	_, evm, err := derivePrivAndEVMAddr(mnemonic, hdPath)
	if err != nil {
		return "", "", err
	}
	s, err := evmToBech32("shinzo", evm)
	return evm.Hex(), s, err
}

func setBech32HRP(hrp string) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(hrp, hrp+sdk.PrefixPublic)
	cfg.SetBech32PrefixForValidator(hrp+"valoper", hrp+"valoperpub")
	cfg.SetBech32PrefixForConsensusNode(hrp+"valcons", hrp+"valconspub")
	cfg.Seal()
}

func evmToBech32(hrp string, evm common.Address) (string, error) {
	data5, err := convertBits(evm.Bytes(), 8, 5, true)
	if err != nil {
		return "", err
	}
	return bech32.Encode(hrp, data5)
}

func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	var ret []byte
	var acc uint = 0
	var bits uint = 0
	maxv := (1 << toBits) - 1
	maxAcc := (1 << (fromBits + toBits - 1)) - 1

	for _, value := range data {
		acc = ((acc << fromBits) | uint(value)) & uint(maxAcc)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, byte((acc>>bits)&uint(maxv)))
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(toBits-bits))&uint(maxv)))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&uint(maxv)) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	return ret, nil
}

const outpostABI = `
[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": false, "name": "resource", "type": "uint8"},
      {"indexed": false, "name": "identity", "type": "string"},
      {"indexed": false, "name": "streamId", "type": "string"},
      {"indexed": false, "name": "expiration", "type": "uint256"}
    ],
    "name": "PaymentCreated",
    "type": "event"
  }
]
`

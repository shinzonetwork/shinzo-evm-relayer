package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/shinzonetwork/shinzo-evm-relayer/internal/app"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/config"
)

const envPrefix = "SHINZO"

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Shinzo EVM Relayer",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Resolve()
			if err != nil {
				return err
			}
			if err := config.EnsureDirs(paths); err != nil {
				return err
			}
			created, err := config.Ensure(paths)
			if err != nil {
				return err
			}
			if created {
				fmt.Println("Created default config at", paths.ConfigFile)
			}

			cfg, err := config.Load(paths)
			if err != nil {
				return err
			}

			applyOverrides(cmd, &cfg)

			logger := app.NewLogger(cfg.Log.Level)
			a := &app.App{Config: cfg, Paths: paths, Logger: logger}
			return a.Start()
		},
	}

	bindFlag(cmd, "evm.rpc", "http://127.0.0.1:8545")
	bindFlag(cmd, "evm.contract", "")
	bindFlag(cmd, "evm.start_block", uint64(0))

	bindFlag(cmd, "evm.payment_listen_enabled", false)

	bindFlag(cmd, "evm.scan_enabled", true)
	bindFlag(cmd, "evm.scan_batch_size", uint64(2000))
	bindFlag(cmd, "evm.confirmations", uint64(0))
	bindFlag(cmd, "evm.issuer", "")
	bindFlag(cmd, "evm.extradata_tag", "0x5348")

	bindFlag(cmd, "cosmos.grpc", "127.0.0.1:9090")
	bindFlag(cmd, "cosmos.rpc", "http://127.0.0.1:26657")
	bindFlag(cmd, "cosmos.chain_id", "9001")
	bindFlag(cmd, "cosmos.fee_denom", "ushinzo")
	bindFlag(cmd, "cosmos.gas_prices", "0.025ushinzo")

	bindFlag(cmd, "log.level", "info")

	return cmd
}

func bindFlag(cmd *cobra.Command, name string, def interface{}) {
	switch v := def.(type) {
	case string:
		cmd.Flags().String(name, v, fmt.Sprintf("%s config", name))
	case uint64:
		cmd.Flags().Uint64(name, v, fmt.Sprintf("%s config", name))
	case bool:
		cmd.Flags().Bool(name, v, fmt.Sprintf("%s config", name))
	default:
		panic(fmt.Sprintf("bindFlag: unsupported type for %q: %T", name, def))
	}
	_ = viper.BindPFlag(name, cmd.Flags().Lookup(name))
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

func applyOverrides(cmd *cobra.Command, cfg *config.Config) {
	if v := viper.GetString("evm.rpc"); v != "" {
		cfg.EVM.RPC = v
	}
	if v := viper.GetString("evm.contract"); v != "" {
		cfg.EVM.Contract = v
	}
	if v := viper.GetUint64("evm.start_block"); v != 0 {
		cfg.EVM.StartBlock = v
	}
	if cmd.Flags().Changed("evm.payment_listen_enabled") {
		cfg.EVM.PaymentListenEnabled = viper.GetBool("evm.payment_listen_enabled")
	}
	if cmd.Flags().Changed("evm.scan_enabled") {
		cfg.EVM.ScanEnabled = viper.GetBool("evm.scan_enabled")
	}
	if v := viper.GetUint64("evm.scan_batch_size"); v != 0 {
		cfg.EVM.ScanBatchSize = v
	}
	if cmd.Flags().Changed("evm.confirmations") {
		cfg.EVM.Confirmations = viper.GetUint64("evm.confirmations")
	}
	if v := viper.GetString("evm.issuer"); v != "" {
		cfg.EVM.Issuer = v
	}
	if v := viper.GetString("evm.extradata_tag"); v != "" {
		cfg.EVM.ExtraDataTag = v
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
}

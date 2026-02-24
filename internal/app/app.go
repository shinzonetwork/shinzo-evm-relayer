package app

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/shinzonetwork/shinzo-evm-relayer/internal/config"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/cosmos"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/evm"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/keys"
)

// App is the top-level application object that owns the relayer pipelines.
type App struct {
	Config config.Config
	Paths  config.Paths
	Logger *log.Logger
}

// NewLogger creates a charmbracelet logger configured for the given level.
func NewLogger(level string) *log.Logger {
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

// Start runs the configured pipelines and blocks until one of them returns an
// error (or the process is killed).
func (a *App) Start() error {
	a.Logger.Info("Relayer setup done, starting processes")
	keys.SetBech32HRP("shinzo")

	errCh := make(chan error, 4)

	if a.Config.EVM.PaymentListenEnabled {
		eventCh := make(chan evm.PaymentEvent, 100)

		go func() {
			l := evm.NewListener(evm.ListenerConfig{
				RPC:          a.Config.EVM.RPC,
				ContractAddr: a.Config.EVM.Contract,
				StartBlock:   a.Config.EVM.StartBlock,
				Logger:       a.Logger,
			})
			if err := l.Run(eventCh); err != nil {
				a.Logger.Error("EVM PaymentCreated listener failed", "err", err)
				errCh <- err
			}
		}()

		go func() {
			s := cosmos.NewPaymentSender(cosmos.PaymentSenderConfig{
				GRPCAddr:  a.Config.Cosmos.GRPC,
				RPCAddr:   a.Config.Cosmos.RPC,
				ChainID:   a.Config.Cosmos.ChainID,
				GasPrices: a.Config.Cosmos.GasPrices,
				ConfigDir: a.Paths.ConfigDir,
				Logger:    a.Logger,
			})
			if err := s.Run(eventCh); err != nil {
				a.Logger.Error("Cosmos payment sender failed", "err", err)
				errCh <- err
			}
		}()

		a.Logger.Info("PaymentCreated pipeline enabled")
	} else {
		a.Logger.Info("PaymentCreated pipeline disabled")
	}

	if a.Config.EVM.ScanEnabled {
		attCh := make(chan evm.AttestationJob, 100)

		go func() {
			sc := evm.NewScanner(evm.ScannerConfig{
				RPC:           a.Config.EVM.RPC,
				IssuerAddr:    a.Config.EVM.Issuer,
				ExtraDataTag:  a.Config.EVM.ExtraDataTag,
				BatchSize:     a.Config.EVM.ScanBatchSize,
				Confirmations: a.Config.EVM.Confirmations,
				StartBlock:    a.Config.EVM.StartBlock,
				DataDir:       a.Paths.DataDir,
				Logger:        a.Logger,
			})
			if err := sc.Run(attCh); err != nil {
				a.Logger.Error("EVM extraData scanner failed", "err", err)
				errCh <- err
			}
		}()

		go func() {
			s := cosmos.NewAttestationSender(cosmos.AttestationSenderConfig{
				GRPCAddr:      a.Config.Cosmos.GRPC,
				RPCAddr:       a.Config.Cosmos.RPC,
				ChainID:       a.Config.Cosmos.ChainID,
				GasPrices:     a.Config.Cosmos.GasPrices,
				ConfigDir:     a.Paths.ConfigDir,
				SourceChain:   a.Config.EVM.SourceChain,
				SourceChainID: a.Config.EVM.SourceChainID,
				Logger:        a.Logger,
			})
			if err := s.Run(attCh); err != nil {
				a.Logger.Error("Cosmos attestation sender failed", "err", err)
				errCh <- err
			}
		}()

		a.Logger.Info("extraData scanning pipeline enabled")
	} else {
		a.Logger.Info("extraData scanning pipeline disabled")
	}

	if !a.Config.EVM.PaymentListenEnabled && !a.Config.EVM.ScanEnabled {
		return fmt.Errorf("nothing to run: enable evm.payment_listen_enabled and/or evm.scan_enabled")
	}

	return <-errCh
}

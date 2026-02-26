package cosmos

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/log"

	shinzosdk "github.com/shinzonetwork/shinzohub/sdk"
	sourcehubtypes "github.com/shinzonetwork/shinzohub/x/sourcehub/types"

	"github.com/shinzonetwork/shinzo-evm-relayer/internal/evm"
)

type PaymentSenderConfig struct {
	GRPCAddr  string
	RPCAddr   string
	ChainID   string
	GasPrices string
	ConfigDir string
	Logger    *log.Logger
}

type PaymentSender struct {
	cfg PaymentSenderConfig
}

func NewPaymentSender(cfg PaymentSenderConfig) *PaymentSender {
	return &PaymentSender{cfg: cfg}
}

func (s *PaymentSender) Run(in <-chan evm.PaymentEvent) error {
	signer, bech32Addr, err := buildSigner(s.cfg.ConfigDir)
	if err != nil {
		return err
	}
	s.cfg.Logger.Info("Cosmos payment sender ready", "address", bech32Addr)

	hubClient, err := shinzosdk.NewClient(
		shinzosdk.WithGRPCAddr(s.cfg.GRPCAddr),
		shinzosdk.WithCometRPCAddr(s.cfg.RPCAddr),
	)
	if err != nil {
		return fmt.Errorf("init hub client: %w", err)
	}
	defer hubClient.Close()

	txBuilder, err := shinzosdk.NewTxBuilder(
		shinzosdk.WithSDKClient(hubClient),
		shinzosdk.WithChainID(s.cfg.ChainID),
		shinzosdk.WithMinGasPrice(s.cfg.GasPrices),
	)
	if err != nil {
		return fmt.Errorf("init tx builder: %w", err)
	}

	for ev := range in {
		s.cfg.Logger.Info("Processing PaymentCreated",
			"tx_hash", ev.TxHash,
			"block", ev.BlockNum,
		)

		resEnum := resourceEnum(ev.Payload.Resource)

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
			s.cfg.Logger.Error("MsgRequestStreamAccess failed",
				"err", err, "stream", ev.Payload.StreamId)
			continue
		}

		s.cfg.Logger.Info("Stream access requested",
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

func resourceEnum(r uint8) sourcehubtypes.Resource {
	switch r {
	case 0:
		return sourcehubtypes.Resource_RESOURCE_PRIMITIVE
	default:
		return sourcehubtypes.Resource_RESOURCE_VIEW
	}
}

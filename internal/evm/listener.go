package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/charmbracelet/log"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type PaymentData struct {
	Resource   uint8
	Identity   string
	StreamId   string
	Expiration *big.Int
}

type PaymentEvent struct {
	TxHash   string
	BlockNum uint64
	Payload  PaymentData
}

type ListenerConfig struct {
	RPC          string
	ContractAddr string
	StartBlock   uint64
	Logger       *log.Logger
}

type Listener struct {
	cfg ListenerConfig
}

func NewListener(cfg ListenerConfig) *Listener {
	return &Listener{cfg: cfg}
}

func (l *Listener) Run(out chan<- PaymentEvent) error {
	if l.cfg.ContractAddr == "" {
		return fmt.Errorf("evm.contract is required when payment_listen_enabled=true")
	}

	l.cfg.Logger.Info("Starting PaymentCreated listener",
		"rpc", l.cfg.RPC,
		"contract", l.cfg.ContractAddr,
	)

	client, err := ethclient.Dial(l.cfg.RPC)
	if err != nil {
		return fmt.Errorf("dial EVM node: %w", err)
	}

	parsed, err := ParseOutpostABI()
	if err != nil {
		return fmt.Errorf("parse outpost ABI: %w", err)
	}

	eventHash := gethcrypto.Keccak256Hash([]byte("PaymentCreated(uint8,string,string,uint256)"))
	contractAddr := common.HexToAddress(l.cfg.ContractAddr)

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
		Topics:    [][]common.Hash{{eventHash}},
		FromBlock: new(big.Int).SetUint64(l.cfg.StartBlock),
	}

	logsCh := make(chan types.Log, 256)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logsCh)
	if err != nil {
		return fmt.Errorf("subscribe to logs: %w", err)
	}

	l.cfg.Logger.Info("Subscribed to PaymentCreated events", "from_block", l.cfg.StartBlock)

	for {
		select {
		case err := <-sub.Err():
			return fmt.Errorf("subscription error: %w", err)

		case vLog := <-logsCh:
			var data PaymentData
			if err := parsed.UnpackIntoInterface(&data, "PaymentCreated", vLog.Data); err != nil {
				l.cfg.Logger.Error("Failed to unpack PaymentCreated log", "err", err)
				continue
			}
			out <- PaymentEvent{
				TxHash:   vLog.TxHash.Hex(),
				BlockNum: vLog.BlockNumber,
				Payload:  data,
			}
		}
	}
}

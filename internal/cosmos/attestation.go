package cosmos

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	shinzosdk "github.com/shinzonetwork/shinzohub/sdk"

	"github.com/shinzonetwork/shinzo-evm-relayer/internal/evm"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/keys"
)

type AttestationSenderConfig struct {
	GRPCAddr      string
	RPCAddr       string
	ChainID       string
	GasPrices     string
	ConfigDir     string
	SourceChain   string
	SourceChainID uint64
	Logger        *log.Logger
}

type AttestationSender struct {
	cfg AttestationSenderConfig
}

func NewAttestationSender(cfg AttestationSenderConfig) *AttestationSender {
	return &AttestationSender{cfg: cfg}
}

func (s *AttestationSender) Run(in <-chan evm.AttestationJob) error {
	signer, bech32Addr, err := buildSigner(s.cfg.ConfigDir)
	if err != nil {
		return err
	}
	s.cfg.Logger.Info("Cosmos attestation sender ready", "address", bech32Addr)

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

	for job := range in {
		s.cfg.Logger.Info("Processing attestation job",
			"block", job.SourceBlock,
			"attestation_id", job.AttestationID.String(),
			"withdrawal", job.Withdrawal.Hex(),
		)

		delegateSig := make([]byte, 65)
		copy(delegateSig, job.DelegateSig)
		if delegateSig[64] == 27 || delegateSig[64] == 28 {
			delegateSig[64] -= 27
		}

		delegatePub, err := gethcrypto.SigToPub(job.Digest[:], delegateSig)
		if err != nil {
			s.cfg.Logger.Error("Delegate sig recovery failed",
				"attestation_id", job.AttestationID.String(), "err", err)
			continue
		}
		delegateEVMAddr := gethcrypto.PubkeyToAddress(*delegatePub)
		delegateBech32, err := keys.EVMToBech32("shinzo", delegateEVMAddr)
		if err != nil {
			s.cfg.Logger.Error("Delegate bech32 encode failed",
				"attestation_id", job.AttestationID.String(), "err", err)
			continue
		}

		params := shinzosdk.IndexerAttestationParams{
			Signer:            signer,
			ConsensusPubKey:   hex.EncodeToString(job.ConsensusPubKey),
			DelegateAddress:   delegateBech32,
			SourceChain:       s.cfg.SourceChain,
			SourceChainID:     s.cfg.SourceChainID,
			AttestationID:     job.AttestationID.String(),
			DelegateDigest:    job.Digest[:],
			DelegateSignature: job.DelegateSig,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		resp, err := shinzosdk.AddIndexerAttestation(ctx, hubClient, txBuilder, params)
		cancel()
		if err != nil {
			s.cfg.Logger.Error("MsgIndexerAttestation failed",
				"attestation_id", job.AttestationID.String(), "err", err)
			continue
		}

		s.cfg.Logger.Info("Indexer attestation relayed",
			"height", resp.Height,
			"hash", resp.TxHash,
			"attestation_id", job.AttestationID.String(),
			"delegate", delegateBech32,
		)
	}

	return nil
}

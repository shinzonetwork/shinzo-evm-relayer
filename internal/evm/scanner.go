package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/shinzonetwork/shinzo-evm-relayer/internal/cursor"
)

type AttestationJob struct {
	SourceBlock string
	BlockHash   string

	PointerFromExtra [15]byte
	PointerASCII     [30]byte

	AttestationID     *big.Int
	Withdrawal        common.Address
	DelegateKey       common.Address
	ConsensusPubKey   []byte
	CreatedAt         uint64
	SignatureDeadline uint64
	SignatureSubmitted bool
	SigSubmittedAt    uint64

	WithdrawalSig []byte
	Digest        [32]byte
	DelegateSig   []byte

	PointerStored [15]byte
	PointerLocal  [15]byte
}

type ScannerConfig struct {
	RPC           string
	IssuerAddr    string
	ExtraDataTag  string
	BatchSize     uint64
	Confirmations uint64
	StartBlock    uint64
	DataDir       string
	Logger        *log.Logger
}

type Scanner struct {
	cfg ScannerConfig
}

func NewScanner(cfg ScannerConfig) *Scanner {
	return &Scanner{cfg: cfg}
}

func (s *Scanner) Run(out chan<- AttestationJob) error {
	if s.cfg.IssuerAddr == "" {
		return fmt.Errorf("evm.issuer is required for extraData scanning")
	}
	if s.cfg.BatchSize == 0 {
		return fmt.Errorf("evm.scan_batch_size must be > 0")
	}

	tagBytes, err := parseHexBytes(s.cfg.ExtraDataTag, 2)
	if err != nil {
		return fmt.Errorf("bad evm.extradata_tag: %w", err)
	}
	var tag [2]byte
	copy(tag[:], tagBytes)

	client, err := ethclient.Dial(s.cfg.RPC)
	if err != nil {
		return fmt.Errorf("dial EVM node: %w", err)
	}

	issuerAddr := common.HexToAddress(s.cfg.IssuerAddr)
	parsed, err := ParseIssuerABI()
	if err != nil {
		return fmt.Errorf("parse issuer ABI: %w", err)
	}

	domainSep, err := DomainSeparator(context.Background(), client, parsed, issuerAddr)
	if err != nil {
		return fmt.Errorf("fetch DOMAIN_SEPARATOR: %w", err)
	}

	cur, err := cursor.Load(s.cfg.DataDir, s.cfg.StartBlock)
	if err != nil {
		return fmt.Errorf("load cursor: %w", err)
	}

	s.cfg.Logger.Info("Starting extraData scanner",
		"rpc", s.cfg.RPC,
		"issuer", issuerAddr.Hex(),
		"tag", s.cfg.ExtraDataTag,
		"next_block", cur.NextBlock,
		"batch", s.cfg.BatchSize,
		"confirmations", s.cfg.Confirmations,
	)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		head, err := client.BlockNumber(ctx)
		cancel()
		if err != nil {
			s.cfg.Logger.Error("Failed to fetch head block number", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		safeHead := head
		if s.cfg.Confirmations > 0 {
			if head < s.cfg.Confirmations {
				time.Sleep(2 * time.Second)
				continue
			}
			safeHead = head - s.cfg.Confirmations
		}

		if cur.NextBlock > safeHead {
			time.Sleep(2 * time.Second)
			continue
		}

		from := cur.NextBlock
		to := from + s.cfg.BatchSize - 1
		if to > safeHead {
			to = safeHead
		}

		s.cfg.Logger.Info("Scanning blocks", "from", from, "to", to)

		for b := from; b <= to; b++ {
			blkCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			block, err := client.BlockByNumber(blkCtx, new(big.Int).SetUint64(b))
			cancel()
			if err != nil {
				return fmt.Errorf("BlockByNumber(%d): %w", b, err)
			}

			extra := block.Extra()
			if len(extra) != 32 || extra[0] != tag[0] || extra[1] != tag[1] {
				continue
			}

			var pointerASCII [30]byte
			copy(pointerASCII[:], extra[2:32])

			pointer15, err := decodePointer(extra)
			if err != nil {
				s.cfg.Logger.Debug("Skip: pointer decode failed", "block", b, "err", err)
				continue
			}

			attID, err := ResolveFromExtraData(context.Background(), client, parsed, issuerAddr, extra)
			if err != nil {
				s.cfg.Logger.Debug("Skip: resolveFromExtraData failed", "block", b, "err", err)
				continue
			}

			core, err := FetchAttestationCore(context.Background(), client, parsed, issuerAddr, attID)
			if err != nil {
				s.cfg.Logger.Error("Failed to fetch attestation core",
					"block", b, "attestation_id", attID.String(), "err", err)
				continue
			}

			digest, err := ComputeDigest(domainSep, attID, core.Withdrawal, core.DelegateKey, core.ConsensusPubKey, core.CreatedAt, core.SignatureDeadline)
			if err != nil {
				s.cfg.Logger.Error("Failed to compute digest locally",
					"block", b, "attestation_id", attID.String(), "err", err)
				continue
			}

			if !VerifySignatureEither(digest, core.Withdrawal, core.WithdrawalSig) {
				s.cfg.Logger.Error("Withdrawal signature verification failed",
					"block", b, "attestation_id", attID.String())
				continue
			}

			if !core.SignatureSubmitted || len(core.DelegateSig) != 65 {
				s.cfg.Logger.Debug("Skip: delegate signature not yet submitted",
					"block", b, "attestation_id", attID.String())
				continue
			}

			ptrLocal := first15(gethcrypto.Keccak256(domainSep[:], digest[:], core.WithdrawalSig))

			s.cfg.Logger.Info("Valid attestation found in extraData",
				"block", b,
				"attestation_id", attID.String(),
				"withdrawal", core.Withdrawal.Hex(),
				"consensus_pub_key", hex.EncodeToString(core.ConsensusPubKey),
				"ptr_extra_eq_stored", pointer15 == core.PointerStored,
				"ptr_extra_eq_local", pointer15 == ptrLocal,
			)

			out <- AttestationJob{
				SourceBlock:       fmt.Sprintf("%d", b),
				BlockHash:         block.Hash().Hex(),
				PointerFromExtra:  pointer15,
				PointerASCII:      pointerASCII,
				AttestationID:     attID,
				Withdrawal:        core.Withdrawal,
				DelegateKey:       core.DelegateKey,
				ConsensusPubKey:   core.ConsensusPubKey,
				CreatedAt:         core.CreatedAt,
				SignatureDeadline: core.SignatureDeadline,
				SignatureSubmitted: core.SignatureSubmitted,
				SigSubmittedAt:    core.SigSubmittedAt,
				WithdrawalSig:     core.WithdrawalSig,
				Digest:            digest,
				DelegateSig:       core.DelegateSig,
				PointerStored:     core.PointerStored,
				PointerLocal:      ptrLocal,
			}
		}

		cur.NextBlock = to + 1
		if err := cursor.Save(s.cfg.DataDir, cursor.Cursor{NextBlock: cur.NextBlock}); err != nil {
			s.cfg.Logger.Error("Failed to save scan cursor", "err", err)
		}
	}
}

func decodePointer(extra []byte) ([15]byte, error) {
	var out [15]byte
	if len(extra) != 32 {
		return out, fmt.Errorf("extraData length must be 32, got %d", len(extra))
	}
	hexChars := extra[2:32]
	for i := 0; i < 15; i++ {
		hi, ok := fromHexChar(hexChars[2*i])
		if !ok {
			return out, fmt.Errorf("invalid hex char at position %d", 2*i)
		}
		lo, ok := fromHexChar(hexChars[2*i+1])
		if !ok {
			return out, fmt.Errorf("invalid hex char at position %d", 2*i+1)
		}
		out[i] = (hi << 4) | lo
	}
	return out, nil
}

func fromHexChar(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func parseHexBytes(s string, want int) ([]byte, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "0x"))
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != want {
		return nil, fmt.Errorf("expected %d bytes, got %d", want, len(b))
	}
	return b, nil
}

func first15(b []byte) [15]byte {
	var out [15]byte
	copy(out[:], b)
	return out
}

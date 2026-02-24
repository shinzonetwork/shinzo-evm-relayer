package evm

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// AttestationCore holds the on-chain state for a single attestation.
type AttestationCore struct {
	Withdrawal        common.Address
	DelegateKey       [32]byte
	ConsensusPubKey   []byte
	CreatedAt         uint64
	SignatureDeadline uint64
	SignatureSubmitted bool
	SigSubmittedAt    uint64
	WithdrawalSig     []byte
	PointerStored     [15]byte
	DelegateSig       []byte
}

// DomainSeparator calls DOMAIN_SEPARATOR() on the issuer contract.
func DomainSeparator(ctx context.Context, client *ethclient.Client, parsed abi.ABI, issuer common.Address) ([32]byte, error) {
	var zero [32]byte
	data, err := parsed.Pack("DOMAIN_SEPARATOR")
	if err != nil {
		return zero, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &issuer, Data: data}, nil)
	if err != nil {
		return zero, err
	}
	out, err := parsed.Unpack("DOMAIN_SEPARATOR", res)
	if err != nil {
		return zero, err
	}
	if len(out) != 1 {
		return zero, fmt.Errorf("DOMAIN_SEPARATOR: unexpected output count")
	}
	return out[0].([32]byte), nil
}

// ResolveFromExtraData calls resolveFromExtraData() on the issuer and returns
// the attestation ID. Returns an error when the pointer is unknown (result = 0).
func ResolveFromExtraData(ctx context.Context, client *ethclient.Client, parsed abi.ABI, issuer common.Address, extra []byte) (*big.Int, error) {
	data, err := parsed.Pack("resolveFromExtraData", extra)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &issuer, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	out, err := parsed.Unpack("resolveFromExtraData", res)
	if err != nil {
		return nil, err
	}
	if len(out) != 1 {
		return nil, fmt.Errorf("resolveFromExtraData: unexpected output count")
	}
	attID, ok := out[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("resolveFromExtraData: output is not *big.Int")
	}
	if attID.Sign() == 0 {
		return nil, fmt.Errorf("resolveFromExtraData: unknown pointer")
	}
	return attID, nil
}

// FetchAttestationCore calls attestationCore() on the issuer.
func FetchAttestationCore(ctx context.Context, client *ethclient.Client, parsed abi.ABI, issuer common.Address, attID *big.Int) (AttestationCore, error) {
	var a AttestationCore
	data, err := parsed.Pack("attestationCore", attID)
	if err != nil {
		return a, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &issuer, Data: data}, nil)
	if err != nil {
		return a, err
	}
	out, err := parsed.Unpack("attestationCore", res)
	if err != nil {
		return a, err
	}
	if len(out) != 10 {
		return a, fmt.Errorf("attestationCore: expected 10 outputs, got %d", len(out))
	}

	a.Withdrawal = out[0].(common.Address)
	a.DelegateKey = out[1].([32]byte)
	a.ConsensusPubKey = out[2].([]byte)
	a.CreatedAt = out[3].(uint64)
	a.SignatureDeadline = out[4].(uint64)
	a.SignatureSubmitted = out[5].(bool)
	a.SigSubmittedAt = out[6].(uint64)
	a.WithdrawalSig = out[7].([]byte)
	a.PointerStored = out[8].([15]byte)
	a.DelegateSig = out[9].([]byte)

	if a.Withdrawal == (common.Address{}) {
		return a, fmt.Errorf("attestationCore: withdrawal address is zero")
	}
	return a, nil
}

// ParseIssuerABI parses and returns the issuer ABI.
func ParseIssuerABI() (abi.ABI, error) {
	return abi.JSON(strings.NewReader(issuerABI))
}

// ParseOutpostABI parses and returns the outpost ABI.
func ParseOutpostABI() (abi.ABI, error) {
	return abi.JSON(strings.NewReader(outpostABI))
}

// attestationTypeString is the EIP-712 type string for AttestationChallenge.
const attestationTypeString = "AttestationChallenge(uint256 attestationId,address withdrawalAddress,bytes32 delegateKey,bytes32 consensusKeyHash,uint64 createdAt,uint64 signatureDeadline)"

// ComputeDigest computes the EIP-712 digest for an attestation locally,
// mirroring the on-chain attestationDigest() logic.
func ComputeDigest(
	domainSep [32]byte,
	attID *big.Int,
	withdrawal common.Address,
	delegateKey [32]byte,
	consensusPubKey []byte,
	createdAt uint64,
	deadline uint64,
) ([32]byte, error) {
	var digest [32]byte

	consHash := gethcrypto.Keccak256Hash(consensusPubKey)
	typehash := gethcrypto.Keccak256Hash([]byte(attestationTypeString))

	uint256Ty, _ := abi.NewType("uint256", "", nil)
	addressTy, _ := abi.NewType("address", "", nil)
	bytes32Ty, _ := abi.NewType("bytes32", "", nil)
	uint64Ty, _ := abi.NewType("uint64", "", nil)

	args := abi.Arguments{
		{Type: bytes32Ty},
		{Type: uint256Ty},
		{Type: addressTy},
		{Type: bytes32Ty},
		{Type: bytes32Ty},
		{Type: uint64Ty},
		{Type: uint64Ty},
	}

	enc, err := args.Pack(typehash, attID, withdrawal, delegateKey, consHash, createdAt, deadline)
	if err != nil {
		return digest, err
	}

	structHash := gethcrypto.Keccak256Hash(enc)
	b := append([]byte{0x19, 0x01}, domainSep[:]...)
	b = append(b, structHash.Bytes()...)
	d := gethcrypto.Keccak256Hash(b)
	copy(digest[:], d.Bytes())
	return digest, nil
}

// VerifySignatureEither verifies that sig65 was produced by expected over
// digest, accepting both raw secp256k1 and EIP-191 personal signatures.
func VerifySignatureEither(digest [32]byte, expected common.Address, sig65 []byte) bool {
	if len(sig65) != 65 {
		return false
	}
	sig := make([]byte, 65)
	copy(sig, sig65)
	if sig[64] == 27 || sig[64] == 28 {
		sig[64] -= 27
	}

	// Raw digest.
	if pub, err := gethcrypto.SigToPub(digest[:], sig); err == nil {
		if gethcrypto.PubkeyToAddress(*pub) == expected {
			return true
		}
	}

	// EIP-191 personal_sign prefix.
	personal := gethcrypto.Keccak256(
		[]byte("\x19Ethereum Signed Message:\n32"),
		digest[:],
	)
	pub2, err := gethcrypto.SigToPub(personal, sig)
	if err != nil {
		return false
	}
	return gethcrypto.PubkeyToAddress(*pub2) == expected
}

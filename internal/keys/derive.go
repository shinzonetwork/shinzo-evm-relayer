package keys

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/btcsuite/btcutil/bech32"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

func DerivePrivAndEVMAddr(mnemonic, hdPath string) (*ecdsa.PrivateKey, common.Address, error) {
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("create wallet: %w", err)
	}
	path, err := hdwallet.ParseDerivationPath(hdPath)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("parse hd path: %w", err)
	}
	acct, err := wallet.Derive(path, false)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive account: %w", err)
	}
	priv, err := wallet.PrivateKey(acct)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("get private key: %w", err)
	}
	return priv, gethcrypto.PubkeyToAddress(priv.PublicKey), nil
}

func PreviewAddresses(mnemonic, hdPath string) (evmAddr, shinzoAddr string, err error) {
	_, addr, err := DerivePrivAndEVMAddr(mnemonic, hdPath)
	if err != nil {
		return "", "", err
	}
	s, err := EVMToBech32("shinzo", addr)
	return addr.Hex(), s, err
}

func EVMToBech32(hrp string, addr common.Address) (string, error) {
	data5, err := convertBits(addr.Bytes(), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("convert bits: %w", err)
	}
	return bech32.Encode(hrp, data5)
}

func SetBech32HRP(hrp string) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(hrp, hrp+sdk.PrefixPublic)
	cfg.SetBech32PrefixForValidator(hrp+"valoper", hrp+"valoperpub")
	cfg.SetBech32PrefixForConsensusNode(hrp+"valcons", hrp+"valconspub")
	cfg.Seal()
}

func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	var (
		ret    []byte
		acc    uint
		bits   uint
		maxv   = uint((1 << toBits) - 1)
		maxAcc = uint((1 << (fromBits + toBits - 1)) - 1)
	)
	for _, value := range data {
		acc = ((acc << fromBits) | uint(value)) & maxAcc
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	return ret, nil
}

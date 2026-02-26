package cosmos

import (
	"fmt"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	shinzosdk "github.com/shinzonetwork/shinzohub/sdk"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/keys"
)

type privKeySigner struct{ key cryptotypes.PrivKey }

func (s *privKeySigner) GetPrivateKey() cryptotypes.PrivKey { return s.key }
func (s *privKeySigner) GetAccAddress() string {
	return sdk.AccAddress(s.key.PubKey().Address().Bytes()).String()
}

func buildSigner(configDir string) (shinzosdk.TxSigner, string, error) {
	rec, err := keys.Load(configDir)
	if err != nil {
		return nil, "", fmt.Errorf("load relayer key: %w", err)
	}

	priv, evmAddr, err := keys.DerivePrivAndEVMAddr(rec.Mnemonic, rec.HdPath)
	if err != nil {
		return nil, "", fmt.Errorf("derive key: %w", err)
	}

	raw := gethcrypto.FromECDSA(priv)
	if len(raw) != 32 {
		return nil, "", fmt.Errorf("unexpected private key length %d", len(raw))
	}

	cosmosPriv := &ethsecp256k1.PrivKey{Key: raw}
	signer := shinzosdk.TxSigner(&privKeySigner{key: cosmosPriv})

	bech32Addr, err := keys.EVMToBech32("shinzo", evmAddr)
	if err != nil {
		return nil, "", fmt.Errorf("bech32 encode: %w", err)
	}

	return signer, bech32Addr, nil
}

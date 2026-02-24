package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cosmos/go-bip39"
	"github.com/spf13/cobra"

	"github.com/shinzonetwork/shinzo-evm-relayer/internal/config"
	"github.com/shinzonetwork/shinzo-evm-relayer/internal/keys"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage relayer keys",
	}
	cmd.AddCommand(newKeysAddCmd())
	cmd.PersistentFlags().String("name", "", "Name of the key")
	cmd.PersistentFlags().String("mnemonic-file", "", "Path to a file containing the mnemonic")
	cmd.PersistentFlags().String("hd-path", "m/44'/60'/0'/0/0", "HD derivation path (Ethermint)")
	return cmd
}

func newKeysAddCmd() *cobra.Command {
	return &cobra.Command{
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
				return fmt.Errorf("read mnemonic file: %w", err)
			}
			mnemonic := strings.TrimSpace(string(bz))
			if !bip39.IsMnemonicValid(mnemonic) {
				return fmt.Errorf("invalid mnemonic")
			}

			paths, err := config.Resolve()
			if err != nil {
				return err
			}
			if err := config.EnsureDirs(paths); err != nil {
				return err
			}

			rec := keys.Record{Name: name, Mnemonic: mnemonic, HdPath: hdPath}
			if err := keys.Save(paths.ConfigDir, rec); err != nil {
				return fmt.Errorf("save key: %w", err)
			}

			evmAddr, shinzoAddr, err := keys.PreviewAddresses(mnemonic, hdPath)
			if err != nil {
				return err
			}
			fmt.Println("Key saved:", name)
			fmt.Println("EVM:   ", evmAddr)
			fmt.Println("SHINZO:", shinzoAddr)
			return nil
		},
	}
}

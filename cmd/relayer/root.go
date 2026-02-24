package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "shinzo-evm-relayer",
		Short: "Relayer for Shinzo Outpost contract -> ShinzoHub",
	}
	root.AddCommand(newStartCmd())
	root.AddCommand(newKeysCmd())
	return root
}

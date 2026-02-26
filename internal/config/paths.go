package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	HomeDir    string
	ConfigDir  string
	DataDir    string
	LogsDir    string
	ConfigFile string
}

func Resolve() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("get home dir: %w", err)
	}
	root := filepath.Join(home, ".shinzo-evm-relayer")
	return Paths{
		HomeDir:    root,
		ConfigDir:  filepath.Join(root, "config"),
		DataDir:    filepath.Join(root, "data"),
		LogsDir:    filepath.Join(root, "logs"),
		ConfigFile: filepath.Join(root, "config", "config.toml"),
	}, nil
}

func EnsureDirs(paths Paths) error {
	for _, d := range []string{paths.HomeDir, paths.ConfigDir, paths.DataDir, paths.LogsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

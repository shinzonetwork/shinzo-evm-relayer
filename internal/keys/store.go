package keys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Record is the persisted key material stored in key.json.
type Record struct {
	Name     string `json:"name"`
	Mnemonic string `json:"mnemonic"`
	HdPath   string `json:"hd_path"`
}

// Load reads the key record from configDir/key.json.
func Load(configDir string) (Record, error) {
	path := filepath.Join(configDir, "key.json")
	bz, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("read key file: %w", err)
	}
	var rec Record
	if err := json.Unmarshal(bz, &rec); err != nil {
		return Record{}, fmt.Errorf("parse key file: %w", err)
	}
	return rec, nil
}

// Save writes the key record to configDir/key.json with restricted permissions.
func Save(configDir string, rec Record) error {
	path := filepath.Join(configDir, "key.json")
	bz, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	if err := os.WriteFile(path, bz, 0o600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}
	return nil
}

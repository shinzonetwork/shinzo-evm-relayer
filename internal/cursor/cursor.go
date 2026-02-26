package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Cursor struct {
	NextBlock uint64 `json:"next_block"`
}

func Load(dataDir string, defaultStart uint64) (Cursor, error) {
	p := filepath.Join(dataDir, "scan_cursor.json")
	bz, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Cursor{NextBlock: defaultStart}, nil
		}
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(bz, &c); err != nil {
		return Cursor{}, err
	}
	if c.NextBlock == 0 {
		c.NextBlock = defaultStart
	}
	return c, nil
}

func Save(dataDir string, c Cursor) error {
	p := filepath.Join(dataDir, "scan_cursor.json")
	bz, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, bz, 0o644)
}

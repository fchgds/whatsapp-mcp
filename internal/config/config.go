// internal/config/config.go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir  string `json:"data_dir"`
	IPCPort  int    `json:"ipc_port"`
	IPCToken string `json:"ipc_token"`
}

func Load(path string) (Config, error) {
	cfg := Config{DataDir: "data", IPCPort: 8377}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config inválida: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if cfg.IPCToken == "" {
		return cfg, errors.New("falta ipc_token en config.json (generá uno aleatorio)")
	}
	return cfg, nil
}

func (c Config) SessionDBPath() string  { return filepath.Join(c.DataDir, "session.db") }
func (c Config) MessagesDBPath() string { return filepath.Join(c.DataDir, "messages.db") }
func (c Config) BaseURL() string        { return fmt.Sprintf("http://127.0.0.1:%d", c.IPCPort) }

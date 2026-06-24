// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("se esperaba error porque falta el token")
	}
	_ = cfg
}

func TestLoadReadsValues(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	os.WriteFile(p, []byte(`{"data_dir":"d","ipc_port":9000,"ipc_token":"secret","daemon_path":"daemon.exe"}`), 0o600)

	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IPCPort != 9000 || cfg.IPCToken != "secret" || cfg.DataDir != "d" || cfg.DaemonPath != "daemon.exe" {
		t.Fatalf("config inesperada: %+v", cfg)
	}
	if cfg.MessagesDBPath() != filepath.Join("d", "messages.db") {
		t.Fatalf("MessagesDBPath mal: %s", cfg.MessagesDBPath())
	}
	if cfg.BaseURL() != "http://127.0.0.1:9000" {
		t.Fatalf("BaseURL mal: %s", cfg.BaseURL())
	}
}

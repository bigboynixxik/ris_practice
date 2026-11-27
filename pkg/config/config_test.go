package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigHonorsEnvironmentOverDotenv(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("HTTP_TIMEOUT", "7s")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("APP_ENV=local\nHTTP_TIMEOUT=2s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != "dev" || cfg.HTTPTimeout != 7*time.Second {
		t.Fatalf("process settings were replaced: %+v", cfg)
	}
}

func TestLoadConfigRejectsMalformedDotenv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("BROKEN=\"unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected a dotenv parse error")
	}
}

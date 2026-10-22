package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if e := os.WriteFile(p, []byte("#c\nDKO_TEST_999=halo\n"), 0600); e != nil {
		t.Fatal(e)
	}
	os.Unsetenv("DKO_TEST_999")
	if e := LoadDotEnv(p); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("DKO_TEST_999") != "halo" {
		t.Fatal("load failed")
	}
	os.Unsetenv("DKO_TEST_999")
}

func TestProductionFailClosed(t *testing.T) {
	t.Setenv("DKO_ENV", "production")
	t.Setenv("DKO_DATABASE_URL", "")
	t.Setenv("DKO_MASTER_KEY", "")
	t.Setenv("DKO_COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("produksi tanpa konfigurasi aman harus ditolak")
	}
	t.Setenv("DKO_DATABASE_URL", "postgres://example")
	t.Setenv("DKO_MASTER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("DKO_COOKIE_SECURE", "true")
	if _, err := Load(); err != nil {
		t.Fatalf("konfigurasi valid ditolak: %v", err)
	}
}

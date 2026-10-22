package config

import (
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

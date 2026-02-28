package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderDir(t *testing.T) {
	dir := ProviderDir("google")
	if !strings.HasSuffix(dir, filepath.Join("providers", "google")) {
		t.Fatalf("expected path ending in providers/google, got %q", dir)
	}
}

func TestEnsureProviderDir(t *testing.T) {
	old := os.Getenv("OBK_CONFIG_DIR")
	tmp := t.TempDir()
	os.Setenv("OBK_CONFIG_DIR", tmp)
	defer os.Setenv("OBK_CONFIG_DIR", old)

	if err := EnsureProviderDir("google"); err != nil {
		t.Fatalf("ensure provider dir: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmp, "providers", "google"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

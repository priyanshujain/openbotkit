package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoogleCredentialsFileDefault(t *testing.T) {
	cfg := Default()
	path := cfg.GoogleCredentialsFile()
	// With no providers config, falls back to gmail.credentials_file (which applyDefaults sets).
	if !strings.HasSuffix(path, filepath.Join("gmail", "credentials.json")) {
		t.Fatalf("expected fallback to gmail credentials, got %q", path)
	}
}

func TestGoogleCredentialsFileFromProviders(t *testing.T) {
	cfg := Default()
	cfg.Providers = &ProvidersConfig{
		Google: &GoogleProviderConfig{
			CredentialsFile: "/custom/path/credentials.json",
		},
	}
	path := cfg.GoogleCredentialsFile()
	if path != "/custom/path/credentials.json" {
		t.Fatalf("expected providers path, got %q", path)
	}
}

func TestGoogleCredentialsFileProvidesOverGmail(t *testing.T) {
	cfg := Default()
	cfg.Gmail.CredentialsFile = "/gmail/creds.json"
	cfg.Providers = &ProvidersConfig{
		Google: &GoogleProviderConfig{
			CredentialsFile: "/provider/creds.json",
		},
	}
	path := cfg.GoogleCredentialsFile()
	if path != "/provider/creds.json" {
		t.Fatalf("providers should take precedence, got %q", path)
	}
}

func TestGoogleCredentialsFileFallsBackToGmail(t *testing.T) {
	cfg := Default()
	cfg.Gmail.CredentialsFile = "/gmail/creds.json"
	cfg.Providers = &ProvidersConfig{Google: &GoogleProviderConfig{}} // empty
	path := cfg.GoogleCredentialsFile()
	if path != "/gmail/creds.json" {
		t.Fatalf("should fall back to gmail, got %q", path)
	}
}

func TestGoogleTokenDBPath(t *testing.T) {
	path := Default().GoogleTokenDBPath()
	if !strings.HasSuffix(path, filepath.Join("providers", "google", "tokens.db")) {
		t.Fatalf("expected providers/google/tokens.db, got %q", path)
	}
}

func TestProvidersConfigSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Providers = &ProvidersConfig{
		Google: &GoogleProviderConfig{
			CredentialsFile: "/my/creds.json",
		},
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Providers == nil || loaded.Providers.Google == nil {
		t.Fatal("providers.google should persist")
	}
	if loaded.Providers.Google.CredentialsFile != "/my/creds.json" {
		t.Fatalf("credentials file: got %q", loaded.Providers.Google.CredentialsFile)
	}
}

func TestProvidersConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
providers:
  google:
    credentials_file: /from/yaml.json
gmail:
  storage:
    driver: sqlite
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.GoogleCredentialsFile() != "/from/yaml.json" {
		t.Fatalf("expected /from/yaml.json, got %q", cfg.GoogleCredentialsFile())
	}
}

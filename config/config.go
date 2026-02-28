package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for obk.
type Config struct {
	Gmail *GmailConfig `yaml:"gmail,omitempty"`
}

// GmailConfig holds Gmail-specific configuration.
type GmailConfig struct {
	CredentialsFile     string        `yaml:"credentials_file,omitempty"`
	DownloadAttachments bool          `yaml:"download_attachments,omitempty"`
	Storage             StorageConfig `yaml:"storage,omitempty"`
}

// StorageConfig holds database configuration for a source.
type StorageConfig struct {
	Driver string `yaml:"driver,omitempty"` // "sqlite" or "postgres"
	DSN    string `yaml:"dsn,omitempty"`    // postgres DSN; sqlite path auto-derived
}

// Load reads the config file from the default location.
func Load() (*Config, error) {
	return LoadFrom(FilePath())
}

// LoadFrom reads the config file from the given path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

// Save writes the config to the default location.
func (c *Config) Save() error {
	return c.SaveTo(FilePath())
}

// SaveTo writes the config to the given path.
func (c *Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Default returns the default configuration.
func Default() *Config {
	cfg := &Config{
		Gmail: &GmailConfig{
			Storage: StorageConfig{
				Driver: "sqlite",
			},
		},
	}
	cfg.applyDefaults()
	return cfg
}

func (c *Config) applyDefaults() {
	if c.Gmail == nil {
		c.Gmail = &GmailConfig{}
	}
	if c.Gmail.Storage.Driver == "" {
		c.Gmail.Storage.Driver = "sqlite"
	}
	if c.Gmail.CredentialsFile == "" {
		c.Gmail.CredentialsFile = filepath.Join(SourceDir("gmail"), "credentials.json")
	}
}

// GmailDataDSN returns the resolved DSN for the Gmail data store.
func (c *Config) GmailDataDSN() string {
	if c.Gmail.Storage.DSN != "" {
		return c.Gmail.Storage.DSN
	}
	// Default SQLite path.
	return filepath.Join(SourceDir("gmail"), "data.db")
}

// GmailTokenDBPath returns the path to the Gmail token database.
func (c *Config) GmailTokenDBPath() string {
	return filepath.Join(SourceDir("gmail"), "tokens.db")
}

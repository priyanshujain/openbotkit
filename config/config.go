package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Providers    *ProvidersConfig    `yaml:"providers,omitempty"`
	Gmail        *GmailConfig        `yaml:"gmail,omitempty"`
	WhatsApp     *WhatsAppConfig     `yaml:"whatsapp,omitempty"`
	Memory       *MemoryConfig       `yaml:"memory,omitempty"`
	AppleNotes   *AppleNotesConfig   `yaml:"applenotes,omitempty"`
	Daemon       *DaemonConfig       `yaml:"daemon,omitempty"`
	Integrations *IntegrationsConfig `yaml:"integrations,omitempty"`
}

type IntegrationsConfig struct {
	GWS *GWSConfig `yaml:"gws,omitempty"`
}

type GWSConfig struct {
	Enabled  bool     `yaml:"enabled,omitempty"`
	Services []string `yaml:"services,omitempty"`
}

type ProvidersConfig struct {
	Google *GoogleProviderConfig `yaml:"google,omitempty"`
}

type GoogleProviderConfig struct {
	CredentialsFile string `yaml:"credentials_file,omitempty"`
}

type DaemonConfig struct {
	GmailSyncPeriod string        `yaml:"gmail_sync_period,omitempty"` // default "15m"
	JobsStorage     StorageConfig `yaml:"jobs_storage,omitempty"`
}

type WhatsAppConfig struct {
	Storage StorageConfig `yaml:"storage,omitempty"`
}

type GmailConfig struct {
	CredentialsFile     string        `yaml:"credentials_file,omitempty"`
	DownloadAttachments bool          `yaml:"download_attachments,omitempty"`
	Storage             StorageConfig `yaml:"storage,omitempty"`
}

type MemoryConfig struct {
	Storage StorageConfig `yaml:"storage,omitempty"`
}

type AppleNotesConfig struct {
	Storage StorageConfig `yaml:"storage,omitempty"`
}

type StorageConfig struct {
	Driver string `yaml:"driver,omitempty"` // "sqlite" or "postgres"
	DSN    string `yaml:"dsn,omitempty"`
}

func Load() (*Config, error) {
	return LoadFrom(FilePath())
}

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

func (c *Config) Save() error {
	return c.SaveTo(FilePath())
}

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

func Default() *Config {
	cfg := &Config{
		Gmail: &GmailConfig{
			Storage: StorageConfig{
				Driver: "sqlite",
			},
		},
		WhatsApp: &WhatsAppConfig{
			Storage: StorageConfig{
				Driver: "sqlite",
			},
		},
		Memory: &MemoryConfig{
			Storage: StorageConfig{
				Driver: "sqlite",
			},
		},
		AppleNotes: &AppleNotesConfig{
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
	if c.WhatsApp == nil {
		c.WhatsApp = &WhatsAppConfig{}
	}
	if c.WhatsApp.Storage.Driver == "" {
		c.WhatsApp.Storage.Driver = "sqlite"
	}
	if c.Memory == nil {
		c.Memory = &MemoryConfig{}
	}
	if c.Memory.Storage.Driver == "" {
		c.Memory.Storage.Driver = "sqlite"
	}
	if c.AppleNotes == nil {
		c.AppleNotes = &AppleNotesConfig{}
	}
	if c.AppleNotes.Storage.Driver == "" {
		c.AppleNotes.Storage.Driver = "sqlite"
	}
	if c.Daemon == nil {
		c.Daemon = &DaemonConfig{}
	}
	if c.Daemon.GmailSyncPeriod == "" {
		c.Daemon.GmailSyncPeriod = "15m"
	}
}

func (c *Config) GmailDataDSN() string {
	if c.Gmail.Storage.DSN != "" {
		return c.Gmail.Storage.DSN
	}
	return filepath.Join(SourceDir("gmail"), "data.db")
}

func (c *Config) WhatsAppDataDSN() string {
	if c.WhatsApp.Storage.DSN != "" {
		return c.WhatsApp.Storage.DSN
	}
	return filepath.Join(SourceDir("whatsapp"), "data.db")
}

func (c *Config) WhatsAppSessionDBPath() string {
	return filepath.Join(SourceDir("whatsapp"), "session.db")
}

func (c *Config) MemoryDataDSN() string {
	if c.Memory.Storage.DSN != "" {
		return c.Memory.Storage.DSN
	}
	return filepath.Join(SourceDir("memory"), "data.db")
}

func (c *Config) AppleNotesDataDSN() string {
	if c.AppleNotes.Storage.DSN != "" {
		return c.AppleNotes.Storage.DSN
	}
	return filepath.Join(SourceDir("applenotes"), "data.db")
}

func (c *Config) JobsDBDSN() string {
	if c.Daemon.JobsStorage.DSN != "" {
		return c.Daemon.JobsStorage.DSN
	}
	return JobsDBPath()
}

// GoogleCredentialsFile returns the credentials file path, checking the new
// providers config first, falling back to the legacy gmail config.
func (c *Config) GoogleCredentialsFile() string {
	if c.Providers != nil && c.Providers.Google != nil && c.Providers.Google.CredentialsFile != "" {
		return c.Providers.Google.CredentialsFile
	}
	if c.Gmail != nil && c.Gmail.CredentialsFile != "" {
		return c.Gmail.CredentialsFile
	}
	return filepath.Join(ProviderDir("google"), "credentials.json")
}

// GoogleTokenDBPath always points to the new provider location.
func (c *Config) GoogleTokenDBPath() string {
	return filepath.Join(ProviderDir("google"), "tokens.db")
}


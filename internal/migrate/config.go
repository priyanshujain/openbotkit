package migrate

import (
	"fmt"
	"path/filepath"

	"github.com/priyanshujain/openbotkit/config"
)

// migrateConfig updates config.yaml to add the providers section
// if the config is in the legacy format.
func migrateConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsLegacyFormat() {
		return nil
	}

	credFile := filepath.Join(config.ProviderDir("google"), "credentials.json")

	cfg.Providers = &config.ProvidersConfig{
		Google: &config.GoogleProviderConfig{
			CredentialsFile: credFile,
		},
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println("  Updated config.yaml with providers section")
	return nil
}

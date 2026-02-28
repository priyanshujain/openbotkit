package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/priyanshujain/openbotkit/config"
)

// migrateCredentials copies credentials.json from the old gmail dir
// to the new provider dir. The old file is preserved as a backup.
func migrateCredentials() error {
	oldPath := filepath.Join(config.SourceDir("gmail"), "credentials.json")
	newPath := filepath.Join(config.ProviderDir("google"), "credentials.json")

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil
	}

	if _, err := os.Stat(newPath); err == nil {
		return nil // already migrated
	}

	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("read old credentials: %w", err)
	}

	if err := os.WriteFile(newPath, data, 0600); err != nil {
		return fmt.Errorf("write new credentials: %w", err)
	}

	fmt.Printf("  Copied credentials: %s → %s\n", oldPath, newPath)
	return nil
}

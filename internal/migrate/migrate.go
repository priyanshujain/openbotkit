package migrate

import (
	"fmt"
	"os"

	"github.com/priyanshujain/openbotkit/config"
)

// NeedsMigration checks whether the old layout exists.
func NeedsMigration() bool {
	oldCreds := config.SourceDir("gmail") + "/credentials.json"
	_, err := os.Stat(oldCreds)
	return err == nil
}

// Run orchestrates all migrations from the old layout to the new one.
func Run() error {
	fmt.Println("Migrating to new provider-based layout...")

	if err := config.EnsureProviderDir("google"); err != nil {
		return fmt.Errorf("create provider dir: %w", err)
	}

	if err := migrateCredentials(); err != nil {
		return fmt.Errorf("migrate credentials: %w", err)
	}

	if err := migrateTokens(); err != nil {
		return fmt.Errorf("migrate tokens: %w", err)
	}

	if err := migrateConfig(); err != nil {
		return fmt.Errorf("migrate config: %w", err)
	}

	fmt.Println("Migration complete.")
	return nil
}

package telegram

import (
	"fmt"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/73ai/openbotkit/store"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "telegram",
	Short: "Manage Telegram data source",
}

func init() {
	Cmd.AddCommand(authCmd)
	Cmd.AddCommand(syncCmd)
	Cmd.AddCommand(messagesCmd)
	Cmd.AddCommand(chatsCmd)
	Cmd.AddCommand(contactsCmd)
}

func openTelegramDB(cfg *config.Config) (*store.DB, error) {
	if err := config.EnsureSourceDir("telegram"); err != nil {
		return nil, fmt.Errorf("create telegram dir: %w", err)
	}

	db, err := store.Open(store.Config{
		Driver: cfg.Telegram.Storage.Driver,
		DSN:    cfg.TelegramDataDSN(),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := tgsrc.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return db, nil
}

// newClient builds a client with the state storage backed by db, so the update
// manager and the access hashers share one store.
func newClient(cfg *config.Config, db *store.DB) (*tgsrc.Client, error) {
	apiID, apiHash, err := tgsrc.Credentials()
	if err != nil {
		return nil, err
	}

	var state *tgsrc.StateStorage
	if db != nil {
		state = tgsrc.NewStateStorage(db)
	}

	client, err := tgsrc.NewClient(cfg.TelegramSessionPath(), apiID, apiHash, state)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	return client, nil
}

func requireSession(cfg *config.Config) error {
	if !tgsrc.HasSession(cfg.TelegramSessionPath()) {
		return fmt.Errorf("not authenticated; run 'obk telegram auth login' first")
	}
	return nil
}

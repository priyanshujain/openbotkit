package whatsapp

import (
	"context"
	"fmt"
	"os/signal"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/platform"
	wasrc "github.com/73ai/openbotkit/source/whatsapp"
	"github.com/73ai/openbotkit/store"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Start WhatsApp message sync daemon",
	Example: `  obk whatsapp sync
  obk whatsapp sync --account personal`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		account, _ := cmd.Flags().GetString("account")

		if err := config.EnsureSourceDir("whatsapp"); err != nil {
			return fmt.Errorf("create whatsapp dir: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), platform.ShutdownSignals...)
		defer stop()

		client, err := wasrc.NewClient(ctx, cfg.WhatsAppAccountSessionDBPath(account))
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}

		if !client.IsAuthenticated() {
			return fmt.Errorf("not authenticated; run 'obk whatsapp auth login --account %s' first", account)
		}

		dsn := cfg.WhatsAppDataDSN()
		db, err := store.Open(store.Config{
			Driver: cfg.WhatsApp.Storage.Driver,
			DSN:    dsn,
		})
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		if err := config.LinkWhatsAppAccount(account); err != nil {
			return fmt.Errorf("link account: %w", err)
		}

		fmt.Printf("Starting WhatsApp sync (account: %s, Ctrl+C to stop)...\n", account)

		result, err := wasrc.Sync(ctx, client, db, wasrc.SyncOptions{Follow: true})
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Printf("\nSync stopped: %d received, %d history", result.Received, result.HistoryMessages)
		if result.Errors > 0 {
			fmt.Printf(", %d errors", result.Errors)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	syncCmd.Flags().String("account", "default", "Account label")
}

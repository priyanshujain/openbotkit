package whatsapp

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/priyanshujain/openbotkit/config"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Start WhatsApp message sync daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := config.EnsureSourceDir("whatsapp"); err != nil {
			return fmt.Errorf("create whatsapp dir: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		client, err := wasrc.NewClient(ctx, cfg.WhatsAppSessionDBPath())
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}

		if !client.IsAuthenticated() {
			return fmt.Errorf("not authenticated; run 'obk whatsapp auth login' first")
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

		fmt.Println("Starting WhatsApp sync (Ctrl+C to stop)...")

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

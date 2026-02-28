package whatsapp

import (
	"context"
	"fmt"

	"github.com/priyanshujain/openbotkit/config"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var messagesSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a text message to a WhatsApp chat",
	RunE: func(cmd *cobra.Command, args []string) error {
		to, _ := cmd.Flags().GetString("to")
		text, _ := cmd.Flags().GetString("text")

		if to == "" {
			return fmt.Errorf("--to flag is required")
		}
		if text == "" {
			return fmt.Errorf("--text flag is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := config.EnsureSourceDir("whatsapp"); err != nil {
			return fmt.Errorf("create whatsapp dir: %w", err)
		}

		ctx := context.Background()

		client, err := wasrc.NewClient(ctx, cfg.WhatsAppSessionDBPath())
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}
		defer client.Disconnect()

		if !client.IsAuthenticated() {
			return fmt.Errorf("not authenticated; run 'obk whatsapp auth login' first")
		}

		db, err := store.Open(store.Config{
			Driver: cfg.WhatsApp.Storage.Driver,
			DSN:    cfg.WhatsAppDataDSN(),
		})
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		if err := wasrc.Migrate(db); err != nil {
			return fmt.Errorf("migrate schema: %w", err)
		}

		result, err := wasrc.SendText(ctx, client, db, wasrc.SendInput{
			ChatJID: to,
			Text:    text,
		})
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}

		fmt.Printf("Message sent: id=%s at %s\n", result.MessageID, result.Timestamp.Format("2006-01-02 15:04:05"))
		return nil
	},
}

func init() {
	messagesSendCmd.Flags().String("to", "", "Recipient JID (required)")
	messagesSendCmd.Flags().String("text", "", "Message text (required)")
}

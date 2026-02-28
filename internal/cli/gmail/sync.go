package gmail

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/priyanshujain/openbotkit/config"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync emails from Gmail into local storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		dsn := cfg.GmailDataDSN()
		db, err := store.Open(store.Config{
			Driver: cfg.Gmail.Storage.Driver,
			DSN:    dsn,
		})
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		account, _ := cmd.Flags().GetString("account")
		full, _ := cmd.Flags().GetBool("full")
		after, _ := cmd.Flags().GetString("after")
		dlAttachments, _ := cmd.Flags().GetBool("download-attachments")

		attachDir := filepath.Join(config.SourceDir("gmail"), "attachments")

		g := gmailsrc.New(gmailsrc.Config{
			CredentialsFile: cfg.Gmail.CredentialsFile,
			TokenDBPath:     cfg.GmailTokenDBPath(),
		})

		ctx := context.Background()
		result, err := g.Sync(ctx, db, gmailsrc.SyncOptions{
			Full:                full,
			After:               after,
			Account:             account,
			DownloadAttachments: dlAttachments || cfg.Gmail.DownloadAttachments,
			AttachmentsDir:      attachDir,
		})
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Printf("\nSync complete: %d fetched, %d skipped", result.Fetched, result.Skipped)
		if result.Errors > 0 {
			fmt.Printf(", %d errors", result.Errors)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	syncCmd.Flags().String("account", "", "Sync only this account")
	syncCmd.Flags().Bool("full", false, "Re-fetch everything (ignore existing)")
	syncCmd.Flags().String("after", "", "Only sync emails after this date (YYYY/MM/DD)")
	syncCmd.Flags().Bool("download-attachments", false, "Save attachments to disk")
}

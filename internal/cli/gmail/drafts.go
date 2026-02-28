package gmail

import (
	"context"
	"fmt"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/provider/google"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/spf13/cobra"
)

var draftsCmd = &cobra.Command{
	Use:   "drafts",
	Short: "Manage Gmail drafts",
}

var draftsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new draft email",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		to, _ := cmd.Flags().GetStringSlice("to")
		cc, _ := cmd.Flags().GetStringSlice("cc")
		bcc, _ := cmd.Flags().GetStringSlice("bcc")
		subject, _ := cmd.Flags().GetString("subject")
		body, _ := cmd.Flags().GetString("body")
		account, _ := cmd.Flags().GetString("account")

		gp := google.New(google.Config{
			CredentialsFile: cfg.GoogleCredentialsFile(),
			TokenDBPath:     cfg.GoogleTokenDBPath(),
		})
		g := gmailsrc.New(gmailsrc.Config{Provider: gp})

		result, err := g.CreateDraft(context.Background(), gmailsrc.ComposeInput{
			To:      to,
			Cc:      cc,
			Bcc:     bcc,
			Subject: subject,
			Body:    body,
			Account: account,
		})
		if err != nil {
			return fmt.Errorf("create draft failed: %w", err)
		}

		fmt.Printf("Draft created (draft ID: %s, message ID: %s)\n", result.DraftID, result.MessageID)
		return nil
	},
}

func init() {
	draftsCreateCmd.Flags().StringSlice("to", nil, "Recipient email addresses")
	draftsCreateCmd.Flags().String("subject", "", "Email subject")
	draftsCreateCmd.Flags().String("body", "", "Email body")
	draftsCreateCmd.Flags().StringSlice("cc", nil, "CC recipients")
	draftsCreateCmd.Flags().StringSlice("bcc", nil, "BCC recipients")
	draftsCreateCmd.Flags().String("account", "", "Gmail account to use")

	draftsCreateCmd.MarkFlagRequired("to")

	draftsCmd.AddCommand(draftsCreateCmd)
}

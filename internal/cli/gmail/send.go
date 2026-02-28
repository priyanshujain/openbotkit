package gmail

import (
	"context"
	"fmt"

	"github.com/priyanshujain/openbotkit/config"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an email via Gmail",
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

		g := gmailsrc.New(gmailsrc.Config{
			CredentialsFile: cfg.Gmail.CredentialsFile,
			TokenDBPath:     cfg.GmailTokenDBPath(),
		})

		result, err := g.Send(context.Background(), gmailsrc.ComposeInput{
			To:      to,
			Cc:      cc,
			Bcc:     bcc,
			Subject: subject,
			Body:    body,
			Account: account,
		})
		if err != nil {
			return fmt.Errorf("send failed: %w", err)
		}

		fmt.Printf("Email sent (message ID: %s, thread ID: %s)\n", result.MessageID, result.ThreadID)
		return nil
	},
}

func init() {
	sendCmd.Flags().StringSlice("to", nil, "Recipient email addresses")
	sendCmd.Flags().String("subject", "", "Email subject")
	sendCmd.Flags().String("body", "", "Email body")
	sendCmd.Flags().StringSlice("cc", nil, "CC recipients")
	sendCmd.Flags().StringSlice("bcc", nil, "BCC recipients")
	sendCmd.Flags().String("account", "", "Gmail account to send from")

	sendCmd.MarkFlagRequired("to")
	sendCmd.MarkFlagRequired("subject")
	sendCmd.MarkFlagRequired("body")
}

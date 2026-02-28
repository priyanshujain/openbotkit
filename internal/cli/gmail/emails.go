package gmail

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/priyanshujain/openbotkit/config"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var emailsCmd = &cobra.Command{
	Use:   "emails",
	Short: "Query stored Gmail emails",
}

var emailsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored emails with optional filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openGmailDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		account, _ := cmd.Flags().GetString("account")
		from, _ := cmd.Flags().GetString("from")
		subject, _ := cmd.Flags().GetString("subject")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		emails, err := gmailsrc.ListEmails(db, gmailsrc.ListOptions{
			Account: account,
			From:    from,
			Subject: subject,
			After:   after,
			Before:  before,
			Limit:   limit,
		})
		if err != nil {
			return fmt.Errorf("list emails: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(emails)
		}

		if len(emails) == 0 {
			fmt.Println("No emails found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MESSAGE ID\tACCOUNT\tFROM\tDATE\tSUBJECT")
		for _, e := range emails {
			subj := e.Subject
			if len(subj) > 60 {
				subj = subj[:57] + "..."
			}
			from := e.From
			if len(from) > 40 {
				from = from[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				e.MessageID, e.Account, from,
				e.Date.Format("2006-01-02"), subj)
		}
		return w.Flush()
	},
}

var emailsGetCmd = &cobra.Command{
	Use:   "get <message-id>",
	Short: "Show full details of a stored email",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openGmailDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		jsonOut, _ := cmd.Flags().GetBool("json")

		email, err := gmailsrc.GetEmail(db, args[0])
		if err != nil {
			return fmt.Errorf("get email: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(email)
		}

		fmt.Printf("Message ID: %s\n", email.MessageID)
		fmt.Printf("Account:    %s\n", email.Account)
		fmt.Printf("From:       %s\n", email.From)
		fmt.Printf("To:         %s\n", email.To)
		fmt.Printf("Subject:    %s\n", email.Subject)
		fmt.Printf("Date:       %s\n", email.Date.Format("2006-01-02 15:04:05"))
		if len(email.Attachments) > 0 {
			fmt.Printf("Attachments:\n")
			for _, a := range email.Attachments {
				fmt.Printf("  - %s (%s)\n", a.Filename, a.MimeType)
			}
		}
		fmt.Printf("\n--- Body ---\n%s\n", email.Body)
		return nil
	},
}

var emailsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across subject and body",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openGmailDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		jsonOut, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")

		emails, err := gmailsrc.SearchEmails(db, args[0], limit)
		if err != nil {
			return fmt.Errorf("search emails: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(emails)
		}

		if len(emails) == 0 {
			fmt.Println("No emails found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MESSAGE ID\tACCOUNT\tFROM\tDATE\tSUBJECT")
		for _, e := range emails {
			subj := e.Subject
			if len(subj) > 60 {
				subj = subj[:57] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				e.MessageID, e.Account, e.From,
				e.Date.Format("2006-01-02"), subj)
		}
		return w.Flush()
	},
}

func init() {
	emailsListCmd.Flags().String("account", "", "Filter by account")
	emailsListCmd.Flags().String("from", "", "Filter by from address")
	emailsListCmd.Flags().String("subject", "", "Filter by subject")
	emailsListCmd.Flags().String("after", "", "Emails after date (YYYY-MM-DD)")
	emailsListCmd.Flags().String("before", "", "Emails before date (YYYY-MM-DD)")
	emailsListCmd.Flags().Int("limit", 50, "Maximum number of results")
	emailsListCmd.Flags().Bool("json", false, "Output as JSON")

	emailsGetCmd.Flags().Bool("json", false, "Output as JSON")

	emailsSearchCmd.Flags().Bool("json", false, "Output as JSON")
	emailsSearchCmd.Flags().Int("limit", 50, "Maximum number of results")

	emailsCmd.AddCommand(emailsListCmd)
	emailsCmd.AddCommand(emailsGetCmd)
	emailsCmd.AddCommand(emailsSearchCmd)
}

func openGmailDB(cfg *config.Config) (*store.DB, error) {
	if err := config.EnsureSourceDir("gmail"); err != nil {
		return nil, fmt.Errorf("create gmail dir: %w", err)
	}

	dsn := cfg.GmailDataDSN()
	db, err := store.Open(store.Config{
		Driver: cfg.Gmail.Storage.Driver,
		DSN:    dsn,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := gmailsrc.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return db, nil
}

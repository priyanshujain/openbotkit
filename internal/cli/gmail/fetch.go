package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/provider/google"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/spf13/cobra"
	gapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch emails on-demand from Gmail API for a specific date range or query",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		account, _ := cmd.Flags().GetString("account")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		query, _ := cmd.Flags().GetString("query")
		jsonOut, _ := cmd.Flags().GetBool("json")
		dlAttachments, _ := cmd.Flags().GetBool("download-attachments")

		if account == "" {
			return fmt.Errorf("--account is required")
		}

		if err := config.EnsureProviderDir("google"); err != nil {
			return fmt.Errorf("create provider dir: %w", err)
		}

		gp := google.New(google.Config{
			CredentialsFile: cfg.GoogleCredentialsFile(),
			TokenDBPath:     cfg.GoogleTokenDBPath(),
		})

		ctx := context.Background()
		httpClient, err := gp.Client(ctx, account, []string{gapi.GmailReadonlyScope})
		if err != nil {
			return fmt.Errorf("get client: %w", err)
		}

		srv, err := gapi.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			return fmt.Errorf("create gmail service: %w", err)
		}

		fq := gmailsrc.FetchQuery{
			After:  after,
			Before: before,
			Query:  query,
		}

		limiter := gmailsrc.NewRateLimiter()
		msgIDs, err := gmailsrc.SearchIDs(srv, fq, limiter)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		fmt.Printf("Found %d messages\n", len(msgIDs))

		// Open DB for storage.
		db, err := openGmailDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		attachDir := filepath.Join(config.SourceDir("gmail"), "attachments")

		var fetched []*gmailsrc.Email
		for _, id := range msgIDs {
			email, err := gmailsrc.FetchEmail(srv, account, id, limiter)
			if err != nil {
				log.Printf("Error fetching %s: %v", id, err)
				continue
			}

			if dlAttachments {
				if err := gmailsrc.SaveAttachments(email, attachDir); err != nil {
					log.Printf("Error saving attachments for %s: %v", id, err)
				}
			}

			if _, err := gmailsrc.SaveEmail(db, email); err != nil {
				log.Printf("Error saving %s: %v", id, err)
			}

			fetched = append(fetched, email)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(fetched)
		}

		if len(fetched) == 0 {
			fmt.Println("No emails fetched.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MESSAGE ID\tFROM\tDATE\tSUBJECT")
		for _, e := range fetched {
			subj := e.Subject
			if len(subj) > 60 {
				subj = subj[:57] + "..."
			}
			from := e.From
			if len(from) > 40 {
				from = from[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				e.MessageID, from, e.Date.Format("2006-01-02"), subj)
		}
		return w.Flush()
	},
}

func init() {
	fetchCmd.Flags().String("account", "", "Account email (required)")
	fetchCmd.Flags().String("after", "", "Fetch emails after date (YYYY/MM/DD)")
	fetchCmd.Flags().String("before", "", "Fetch emails before date (YYYY/MM/DD)")
	fetchCmd.Flags().String("query", "", "Raw Gmail search query")
	fetchCmd.Flags().Bool("json", false, "Output as JSON")
	fetchCmd.Flags().Bool("download-attachments", false, "Save attachments to disk")
}


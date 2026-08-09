package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/oauth/google"
	contactsrc "github.com/73ai/openbotkit/service/contacts"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/source"
	ansrc "github.com/73ai/openbotkit/source/applenotes"
	finsrc "github.com/73ai/openbotkit/source/finance"
	gmailsrc "github.com/73ai/openbotkit/source/gmail"
	imsrc "github.com/73ai/openbotkit/source/imessage"
	slacksrc "github.com/73ai/openbotkit/source/slack"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	wasrc "github.com/73ai/openbotkit/source/whatsapp"
	wssrc "github.com/73ai/openbotkit/source/websearch"
	"github.com/73ai/openbotkit/store"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all configured data sources",
	Example: `  obk status
  obk status --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		jsonOut, _ := cmd.Flags().GetBool("json")

		if cfg.IsRemote() {
			client, err := remoteClient(cfg)
			if err != nil {
				return err
			}
			health, err := client.Health()
			if err != nil {
				return fmt.Errorf("remote server unreachable: %w", err)
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(health)
			}
			fmt.Printf("Remote server: %s\n", cfg.Remote.Server)
			fmt.Printf("Status: %s\n", health["status"])
			return nil
		}

		gp := google.New(google.Config{
			CredentialsFile: cfg.GoogleCredentialsFile(),
			TokenDBPath:     cfg.GoogleTokenDBPath(),
		})
		g := gmailsrc.New(gmailsrc.Config{Provider: gp})
		source.Register(g)

		wa := wasrc.New(wasrc.Config{
			SessionDBPath: cfg.WhatsAppSessionDBPath(),
		})
		source.Register(wa)

		an := ansrc.New(ansrc.Config{})
		source.Register(an)

		im := imsrc.New(imsrc.Config{})
		source.Register(im)

		fin := finsrc.New(finsrc.Config{})
		source.Register(fin)

		ws := wssrc.New(wssrc.Config{WebSearch: cfg.WebSearch})
		source.Register(ws)

		sl := slacksrc.New(slacksrc.Config{Slack: cfg.Slack})
		source.Register(sl)

		tg := tgsrc.New(tgsrc.Config{SessionPath: cfg.TelegramSessionPath()})
		source.Register(tg)

		ctx := context.Background()

		type sourceStatus struct {
			Name      string  `json:"name"`
			Connected bool    `json:"connected"`
			Accounts  int     `json:"accounts"`
			Items     int64   `json:"items"`
			LastSync  *string `json:"last_sync,omitempty"`
			Error     string  `json:"error,omitempty"`
		}
		var statuses []sourceStatus

		for _, s := range source.All() {
			var db *store.DB
			switch s.Name() {
			case "gmail":
				dsn := cfg.GmailDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.Gmail.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					gmailsrc.Migrate(db)
				}
			case "whatsapp":
				dsn := cfg.WhatsAppDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.WhatsApp.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					wasrc.Migrate(db)
				}
			case "applenotes":
				dsn := cfg.AppleNotesDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.AppleNotes.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					ansrc.Migrate(db)
				}
			case "imessage":
				dsn := cfg.IMessageDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.IMessage.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					imsrc.Migrate(db)
				}
			case "websearch":
				dsn := cfg.WebSearchDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.WebSearch.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					wssrc.Migrate(db)
				}
			case "telegram":
				dsn := cfg.TelegramDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.Telegram.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					tgsrc.Migrate(db)
				}
			}

			st, err := s.Status(ctx, db)
			if db != nil {
				db.Close()
			}

			if err != nil {
				statuses = append(statuses, sourceStatus{Name: s.Name(), Error: err.Error()})
				continue
			}

			var lastSync *string
			if st.LastSyncedAt != nil {
				ts := st.LastSyncedAt.Format("2006-01-02 15:04")
				lastSync = &ts
			}

			statuses = append(statuses, sourceStatus{
				Name:      s.Name(),
				Connected: st.Connected,
				Accounts:  len(st.Accounts),
				Items:     st.ItemCount,
				LastSync:  lastSync,
			})
		}

		// Contacts is a cross-cutting store (populated by gmail, whatsapp,
		// imessage syncs), not a standalone source. Report its count separately.
		if cdb, err := store.Open(store.Config{
			Driver: cfg.Contacts.Storage.Driver,
			DSN:    cfg.ContactsDataDSN(),
		}); err == nil {
			contactsrc.Migrate(cdb)
			count, _ := contactsrc.CountContacts(cdb)
			cdb.Close()
			statuses = append(statuses, sourceStatus{
				Name:      "contacts",
				Connected: count > 0,
				Items:     count,
			})
		}

		// History is a derived store (captures from chat sessions),
		// not a standalone source. Report its count separately.
		histDir := config.HistoryDir()
		historysrc.EnsureDir(histDir)
		histStore := historysrc.NewStore(histDir)
		if count, err := histStore.CountConversations(); err == nil {
			lastCapture, _ := histStore.LastCaptureTime()
			var lastSync *string
			if lastCapture != nil {
				ts := lastCapture.Format("2006-01-02 15:04")
				lastSync = &ts
			}
			statuses = append(statuses, sourceStatus{
				Name:      "history",
				Connected: count > 0,
				Items:     count,
				LastSync:  lastSync,
			})
		} else {
			statuses = append(statuses, sourceStatus{Name: "history", Error: err.Error()})
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(statuses)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SOURCE\tCONNECTED\tACCOUNTS\tITEMS\tLAST SYNC")
		for _, st := range statuses {
			if st.Error != "" {
				fmt.Fprintf(w, "%s\terror\t-\t-\t-\n", st.Name)
				continue
			}
			connected := "no"
			if st.Connected {
				connected = "yes"
			}
			accounts := "-"
			if st.Accounts > 0 {
				accounts = fmt.Sprintf("%d", st.Accounts)
			}
			lastSync := "never"
			if st.LastSync != nil {
				lastSync = *st.LastSync
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", st.Name, connected, accounts, st.Items, lastSync)
		}
		return w.Flush()
	},
}

func init() {
	statusCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(statusCmd)
}

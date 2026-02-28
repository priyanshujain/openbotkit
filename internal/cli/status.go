package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/provider/google"
	"github.com/priyanshujain/openbotkit/source"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	memorysrc "github.com/priyanshujain/openbotkit/source/memory"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all configured data sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
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

		mem := memorysrc.New(memorysrc.Config{})
		source.Register(mem)

		ctx := context.Background()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SOURCE\tCONNECTED\tACCOUNTS\tITEMS\tLAST SYNC")

		for _, s := range source.All() {
			// Try to open the source's database.
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
			case "memory":
				dsn := cfg.MemoryDataDSN()
				db, _ = store.Open(store.Config{
					Driver: cfg.Memory.Storage.Driver,
					DSN:    dsn,
				})
				if db != nil {
					memorysrc.Migrate(db)
				}
			}

			st, err := s.Status(ctx, db)
			if db != nil {
				db.Close()
			}

			if err != nil {
				fmt.Fprintf(w, "%s\terror\t-\t-\t-\n", s.Name())
				continue
			}

			connected := "no"
			if st.Connected {
				connected = "yes"
			}

			accounts := "-"
			if len(st.Accounts) > 0 {
				accounts = fmt.Sprintf("%d", len(st.Accounts))
			}

			lastSync := "never"
			if st.LastSyncedAt != nil {
				lastSync = st.LastSyncedAt.Format("2006-01-02 15:04")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				s.Name(), connected, accounts, st.ItemCount, lastSync)
		}

		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

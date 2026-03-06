package whatsapp

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/priyanshujain/openbotkit/config"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/spf13/cobra"
)

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Query synced WhatsApp contacts",
}

var contactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List synced contacts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openWhatsAppDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		contacts, err := wasrc.ListContacts(db, query, limit)
		if err != nil {
			return fmt.Errorf("list contacts: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(contacts)
		}

		if len(contacts) == 0 {
			fmt.Println("No contacts found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PHONE\tNAME\tPUSH NAME\tBUSINESS")
		for _, c := range contacts {
			name := c.FullName
			if name == "" {
				name = c.FirstName
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Phone, name, c.PushName, c.BusinessName)
		}
		return w.Flush()
	},
}

func init() {
	contactsListCmd.Flags().String("query", "", "Search contacts by name or phone")
	contactsListCmd.Flags().Int("limit", 50, "Maximum number of results")
	contactsListCmd.Flags().Bool("json", false, "Output as JSON")

	contactsCmd.AddCommand(contactsListCmd)
}

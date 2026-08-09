package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/spf13/cobra"
)

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Query stored Telegram users",
}

var contactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List synced Telegram users",
	Example: `  obk telegram contacts list
  obk telegram contacts list --search ann --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		query, _ := cmd.Flags().GetString("search")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		users, err := tgsrc.ListUsers(db, query, limit)
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(users)
		}

		if len(users) == 0 {
			fmt.Println("No users found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "USER ID\tNAME\tUSERNAME\tPHONE\tBOT")
		for _, u := range users {
			username := u.Username
			if username != "" {
				username = "@" + username
			}
			isBot := "no"
			if u.IsBot {
				isBot = "yes"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", u.UserID, truncate(u.DisplayName(), 30), username, u.Phone, isBot)
		}
		return w.Flush()
	},
}

func init() {
	contactsListCmd.Flags().String("search", "", "Filter by name, username or phone")
	contactsListCmd.Flags().Int("limit", 50, "Maximum number of results")
	contactsListCmd.Flags().Bool("json", false, "Output as JSON")

	contactsCmd.AddCommand(contactsListCmd)
}

package gmail

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/priyanshujain/openbotkit/config"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/spf13/cobra"
)

var attachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "Query stored Gmail attachments",
}

var attachmentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List attachment metadata",
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

		emailID, _ := cmd.Flags().GetString("email-id")
		jsonOut, _ := cmd.Flags().GetBool("json")

		attachments, err := gmailsrc.ListAttachments(db, emailID)
		if err != nil {
			return fmt.Errorf("list attachments: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(attachments)
		}

		if len(attachments) == 0 {
			fmt.Println("No attachments found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "EMAIL ID\tFILENAME\tMIME TYPE\tPATH")
		for _, a := range attachments {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				a.EmailMessageID, a.Filename, a.MimeType, a.SavedPath)
		}
		return w.Flush()
	},
}

func init() {
	attachmentsListCmd.Flags().String("email-id", "", "Filter by email message ID")
	attachmentsListCmd.Flags().Bool("json", false, "Output as JSON")

	attachmentsCmd.AddCommand(attachmentsListCmd)
}

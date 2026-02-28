package gmail

import (
	"github.com/spf13/cobra"
)

// Cmd is the top-level "gmail" command group.
var Cmd = &cobra.Command{
	Use:   "gmail",
	Short: "Manage Gmail data source",
}

func init() {
	Cmd.AddCommand(authCmd)
	Cmd.AddCommand(syncCmd)
	Cmd.AddCommand(emailsCmd)
	Cmd.AddCommand(attachmentsCmd)
}

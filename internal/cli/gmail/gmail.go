package gmail

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "gmail",
	Short: "Manage Gmail data source",
}

func init() {
	Cmd.AddCommand(syncCmd)
	Cmd.AddCommand(fetchCmd)
	Cmd.AddCommand(emailsCmd)
	Cmd.AddCommand(attachmentsCmd)
}

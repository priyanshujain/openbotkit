package auth

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication for data source providers",
}

func init() {
	Cmd.AddCommand(googleCmd)
	Cmd.AddCommand(whatsappCmd)
}

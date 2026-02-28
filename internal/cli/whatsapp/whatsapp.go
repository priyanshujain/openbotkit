package whatsapp

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "whatsapp",
	Short: "Manage WhatsApp data source",
}

func init() {
	Cmd.AddCommand(syncCmd)
	Cmd.AddCommand(messagesCmd)
	Cmd.AddCommand(chatsCmd)
}

package cli

import (
	"fmt"
	"os"

	"github.com/priyanshujain/openbotkit/internal/cli/gmail"
	memorycli "github.com/priyanshujain/openbotkit/internal/cli/memory"
	whatsappcli "github.com/priyanshujain/openbotkit/internal/cli/whatsapp"
	"github.com/spf13/cobra"
)

var Version = "dev" // set via -ldflags

var rootCmd = &cobra.Command{
	Use:   "obk",
	Short: "OpenBotKit — toolkit for building AI personal assistants",
	Long:  "OpenBotKit (obk) is a toolkit for building AI personal assistants through data source integrations.",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the obk version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("obk version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(gmail.Cmd)
	rootCmd.AddCommand(memorycli.Cmd)
	rootCmd.AddCommand(whatsappcli.Cmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

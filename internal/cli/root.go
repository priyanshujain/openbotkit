package cli

import (
	"fmt"
	"os"

	applenotescli "github.com/73ai/openbotkit/internal/cli/applenotes"
	backupcli "github.com/73ai/openbotkit/internal/cli/backup"
	contactscli "github.com/73ai/openbotkit/internal/cli/contacts"
	financecli "github.com/73ai/openbotkit/internal/cli/finance"
	"github.com/73ai/openbotkit/internal/cli/gmail"
	historycli "github.com/73ai/openbotkit/internal/cli/history"
	imessagecli "github.com/73ai/openbotkit/internal/cli/imessage"
	learningscli "github.com/73ai/openbotkit/internal/cli/learnings"
	memorycli "github.com/73ai/openbotkit/internal/cli/memory"
	slackcli "github.com/73ai/openbotkit/internal/cli/slack"
	telegramcli "github.com/73ai/openbotkit/internal/cli/telegram"
	websearchcli "github.com/73ai/openbotkit/internal/cli/websearch"
	whatsappcli "github.com/73ai/openbotkit/internal/cli/whatsapp"
	xcli "github.com/73ai/openbotkit/internal/cli/x"
	telegramsrc "github.com/73ai/openbotkit/source/telegram"
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
	// source packages cannot import this one, so push the stamped version out.
	telegramsrc.SetAppVersion(Version)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(applenotescli.Cmd)
	rootCmd.AddCommand(backupcli.Cmd)
	rootCmd.AddCommand(contactscli.Cmd)
	rootCmd.AddCommand(financecli.Cmd)
	rootCmd.AddCommand(imessagecli.Cmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(gmail.Cmd)
	rootCmd.AddCommand(historycli.Cmd)
	rootCmd.AddCommand(learningscli.Cmd)
	rootCmd.AddCommand(memorycli.Cmd)
	rootCmd.AddCommand(slackcli.Cmd)
	rootCmd.AddCommand(telegramcli.Cmd)
	rootCmd.AddCommand(websearchcli.Cmd)
	rootCmd.AddCommand(whatsappcli.Cmd)
	rootCmd.AddCommand(xcli.Cmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

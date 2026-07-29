package slack

import (
	"fmt"
	"os"

	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/spf13/cobra"
)

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Work with Slack files",
}

var mediaDownloadCmd = &cobra.Command{
	Use:   "download <url>",
	Short: "Download a Slack file to stdout",
	Long: `Download a Slack file and write its bytes to stdout.

Accepts a url_private from a message's files[], or a /files/… permalink.
Progress and errors go to stderr, so stdout can be redirected to a file.`,
	Example: `  obk slack media download https://files.slack.com/files-pri/T1-F1/shot.jpg > shot.jpg
  obk slack media download https://acme.slack.com/files/U01ABC/F01ABC/shot.jpg > shot.jpg`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if stdoutIsTerminal() {
			return fmt.Errorf("refusing to write binary data to the terminal; redirect it: obk slack media download %s > file", args[0])
		}

		client, err := loadClient()
		if err != nil {
			return err
		}

		downloadURL, err := slacksrc.ResolveFileURL(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "downloading %s\n", downloadURL)

		return client.DownloadFile(cmd.Context(), downloadURL, os.Stdout)
	},
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func init() {
	mediaCmd.AddCommand(mediaDownloadCmd)
}

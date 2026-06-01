package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/spf13/cobra"
)

type channelExport struct {
	Channel    string           `json:"channel"`
	ExportedAt string           `json:"exported_at"`
	Messages   []slacksrc.Message `json:"messages"`
}

var exportCmd = &cobra.Command{
	Use:   "export <channel>",
	Short: "Export all messages from a Slack channel to a JSON file",
	Example: `  obk slack export bug_support_qa
  obk slack export "#general" -o general.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return err
		}

		channelRef := args[0]
		channelID, err := client.ResolveChannel(cmd.Context(), channelRef)
		if err != nil {
			return fmt.Errorf("resolve channel: %w", err)
		}

		outputFile, _ := cmd.Flags().GetString("output")
		if outputFile == "" {
			safe := strings.TrimPrefix(channelRef, "#")
			outputFile = fmt.Sprintf("slack_export_%s_%s.json", safe, time.Now().Format("20060102"))
		}

		fmt.Fprintf(os.Stderr, "Fetching messages from %s...\n", channelRef)
		msgs, err := client.ConversationsHistoryAll(cmd.Context(), channelID, slacksrc.HistoryOptions{})
		if err != nil {
			return fmt.Errorf("fetch messages: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Fetched %d messages\n", len(msgs))

		export := channelExport{
			Channel:    channelRef,
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
			Messages:   msgs,
		}

		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(export); err != nil {
			return fmt.Errorf("write JSON: %w", err)
		}

		fmt.Printf("Exported to %s\n", outputFile)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringP("output", "o", "", "Output file (default: slack_export_<channel>_<date>.json)")
}

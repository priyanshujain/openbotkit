package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/spf13/cobra"
)

var replyCmd = &cobra.Command{
	Use:   "reply [permalink]",
	Short: "Post a reply in a Slack thread",
	Long: `Post a reply in a Slack thread, as you.

Accepts a message permalink, or --channel-id plus --thread-id. The body comes
from --text or from stdin; piped stdin wins. It is sent verbatim, so Slack
mrkdwn is your responsibility. The message posts immediately, with no
confirmation prompt.`,
	Example: `  obk slack reply https://acme.slack.com/archives/C01ABC23DEF/p1785334150236249 --text "fixed in v2.4"
  echo "fixed in v2.4" | obk slack reply --channel-id C01ABC23DEF --thread-id p1785334150236249`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		channelFlag, _ := cmd.Flags().GetString("channel-id")
		threadFlag, _ := cmd.Flags().GetString("thread-id")
		text, _ := cmd.Flags().GetString("text")

		if len(args) == 1 && (channelFlag != "" || threadFlag != "") {
			return fmt.Errorf("pass a permalink or --channel-id/--thread-id, not both")
		}
		if len(args) == 0 && (channelFlag == "" || threadFlag == "") {
			return fmt.Errorf("pass a permalink, or both --channel-id and --thread-id")
		}

		// Parse the address before loading credentials, so bad input fails
		// on its own terms instead of on a missing workspace.
		ref, flagTS, err := parseThreadAddress(args, threadFlag)
		if err != nil {
			return err
		}

		if piped, err := readPipedStdin(); err != nil {
			return err
		} else if piped != "" {
			text = piped
		}
		if text == "" {
			return fmt.Errorf("empty message; pass --text or pipe the body on stdin")
		}

		client, _, workspace, err := loadClientAndCache()
		if err != nil {
			return err
		}

		var channelID, ts string
		if ref != nil {
			if !slacksrc.MatchesWorkspace(ref.Subdomain, workspace) {
				return fmt.Errorf("permalink belongs to workspace %q but obk is authenticated to %q; run: obk slack auth login", ref.Subdomain, workspace)
			}
			channelID = ref.ChannelID
			if ts, err = ref.RootTS(); err != nil {
				return err
			}
		} else {
			if channelID, err = client.ResolveChannel(cmd.Context(), channelFlag); err != nil {
				return fmt.Errorf("resolve channel: %w", err)
			}
			ts = flagTS
		}

		// Slack wants the thread parent, so a link to a reply is walked up first.
		rootTS, err := slacksrc.ResolveThreadRoot(cmd.Context(), client, channelID, ts)
		if err != nil {
			return err
		}

		postedTS, err := client.PostMessage(cmd.Context(), channelID, text, rootTS)
		if err != nil {
			return fmt.Errorf("post reply: %w", err)
		}

		out := map[string]string{
			"channel_id": channelID,
			"thread_id":  slacksrc.TSToMessageID(rootTS),
			"ts":         postedTS,
			"message_id": slacksrc.TSToMessageID(postedTS),
		}
		if link, err := client.GetPermalink(cmd.Context(), channelID, postedTS); err == nil {
			out["permalink"] = link
		} else {
			fmt.Fprintf(os.Stderr, "posted, but could not fetch permalink: %v\n", err)
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

// readPipedStdin returns stdin when it is a pipe or file, and "" when it is a
// terminal, so an interactive run does not block waiting for input.
func readPipedStdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", nil
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(data), nil
}

func init() {
	replyCmd.Flags().String("channel-id", "", "Channel ID or name (with --thread-id)")
	replyCmd.Flags().String("thread-id", "", "Message ID of any message in the thread, e.g. p1785334150236249")
	replyCmd.Flags().String("text", "", "Message body; omit to read from stdin")
}

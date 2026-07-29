package slack

import (
	"encoding/json"
	"fmt"
	"os"

	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/spf13/cobra"
)

var threadCmd = &cobra.Command{
	Use:   "thread [permalink]",
	Short: "Read a complete Slack thread as JSON",
	Long: `Read every message in a Slack thread, with user IDs resolved to names.

Accepts a message permalink, or --channel-id plus --thread-id. Any message in
the thread works: a link to a reply returns the whole thread.`,
	Example: `  obk slack thread https://acme.slack.com/archives/C01ABC23DEF/p1785334150236249
  obk slack thread --channel-id C01ABC23DEF --thread-id p1785334150236249
  obk slack thread --channel-id support --thread-id p1785334150236249`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		channelFlag, _ := cmd.Flags().GetString("channel-id")
		threadFlag, _ := cmd.Flags().GetString("thread-id")
		refreshUsers, _ := cmd.Flags().GetBool("refresh-users")

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

		client, cache, workspace, err := loadClientAndCache()
		if err != nil {
			return err
		}
		if refreshUsers {
			cache.Refresh()
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

		thread, err := slacksrc.FetchThread(cmd.Context(), client, cache, channelID, ts)
		if err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(thread)
	},
}

// parseThreadAddress validates the permalink or --thread-id form. It needs no
// config or network, so callers run it before loading credentials.
func parseThreadAddress(args []string, threadFlag string) (*slacksrc.MessageRef, string, error) {
	if len(args) == 1 {
		ref, err := slacksrc.ParsePermalink(args[0])
		if err != nil {
			return nil, "", err
		}
		return ref, "", nil
	}
	ts, err := slacksrc.MessageIDToTS(threadFlag)
	if err != nil {
		return nil, "", err
	}
	return nil, ts, nil
}

func init() {
	threadCmd.Flags().String("channel-id", "", "Channel ID or name (with --thread-id)")
	threadCmd.Flags().String("thread-id", "", "Message ID of any message in the thread, e.g. p1785334150236249")
	threadCmd.Flags().Bool("refresh-users", false, "Re-fetch user names instead of using the cache")
}

package slack

import (
	"fmt"

	"github.com/73ai/openbotkit/config"
	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "slack",
	Short: "Manage Slack data source",
}

func loadClient() (*slacksrc.Client, error) {
	client, _, _, err := loadClientAndCache()
	return client, err
}

// loadClientAndCache also returns the user name cache and the workspace it is
// keyed on, for commands that resolve IDs to names.
func loadClientAndCache() (*slacksrc.Client, *slacksrc.UserCache, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, "", fmt.Errorf("load config: %w", err)
	}
	if cfg.Slack == nil || cfg.Slack.DefaultWorkspace == "" {
		return nil, nil, "", fmt.Errorf("no Slack workspace configured; run: obk slack auth login")
	}
	workspace := cfg.Slack.DefaultWorkspace
	creds, err := slacksrc.LoadCredentials(workspace)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load credentials: %w", err)
	}
	client := slacksrc.NewClient(creds.Token, creds.Cookie)
	return client, slacksrc.NewUserCache(workspace, client), workspace, nil
}

func init() {
	Cmd.AddCommand(authCmd)
	Cmd.AddCommand(searchCmd)
	Cmd.AddCommand(channelsCmd)
	Cmd.AddCommand(readCmd)
	Cmd.AddCommand(threadCmd)
	Cmd.AddCommand(mediaCmd)
}

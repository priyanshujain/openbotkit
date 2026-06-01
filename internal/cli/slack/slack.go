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
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.Slack == nil || cfg.Slack.DefaultWorkspace == "" {
		return nil, fmt.Errorf("no Slack workspace configured; run: obk slack auth login")
	}
	creds, err := slacksrc.LoadCredentials(cfg.Slack.DefaultWorkspace)
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	return slacksrc.NewClient(creds.Token, creds.Cookie), nil
}

func init() {
	Cmd.AddCommand(authCmd)
	Cmd.AddCommand(searchCmd)
	Cmd.AddCommand(channelsCmd)
	Cmd.AddCommand(readCmd)
}

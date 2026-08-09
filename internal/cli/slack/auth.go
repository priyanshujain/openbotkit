package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/73ai/openbotkit/config"
	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/73ai/openbotkit/source/slack/desktop"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Slack authentication",
}

var authLoginCmd = &cobra.Command{
	Use:     "login",
	Short:   "Authenticate with a Slack workspace",
	Example: `  obk slack auth login`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var authMode string

		options := []huh.Option[string]{
			huh.NewOption("Manual token entry", "token"),
		}
		if runtime.GOOS == "darwin" {
			options = append([]huh.Option[string]{
				huh.NewOption("Auto-detect from Slack Desktop", "desktop"),
			}, options...)
		}

		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("How would you like to authenticate?").
					Options(options...).
					Value(&authMode),
			),
		).Run()
		if err != nil {
			return err
		}

		switch authMode {
		case "desktop":
			return authLoginDesktop()
		case "token":
			return authLoginToken()
		default:
			return fmt.Errorf("unknown auth mode: %s", authMode)
		}
	},
}

func authLoginDesktop() error {
	fmt.Println("Extracting credentials from Slack Desktop...")
	fmt.Println("Note: macOS will ask for permission to access \"Slack Safe Storage\" in your keychain.")
	fmt.Println("Click \"Always Allow\" so you won't be prompted again.")
	creds, err := desktop.Extract()
	if err != nil {
		return fmt.Errorf("desktop extraction failed: %w", err)
	}

	workspace := slacksrc.SanitizeWorkspaceName(creds.TeamName)
	if err := slacksrc.SaveCredentials(workspace, creds.Token, creds.Cookie); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.Slack == nil {
		cfg.Slack = &config.SlackConfig{}
	}
	if cfg.Slack.Workspaces == nil {
		cfg.Slack.Workspaces = make(map[string]config.SlackWorkspace)
	}
	cfg.Slack.Workspaces[workspace] = config.SlackWorkspace{
		TeamID:   creds.TeamID,
		TeamName: creds.TeamName,
		AuthMode: "desktop",
	}
	if cfg.Slack.DefaultWorkspace == "" {
		cfg.Slack.DefaultWorkspace = workspace
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Authenticated with workspace %q (team: %s)\n", workspace, creds.TeamName)
	return nil
}

func authLoginToken() error {
	var token, cookie, workspace string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Workspace name").
				Description("A short name for this workspace").
				Placeholder("my-company").
				Value(&workspace),
			huh.NewInput().
				Title("Slack token").
				Description("xoxp-, xoxb-, or xoxc- token").
				Value(&token),
			huh.NewInput().
				Title("Cookie (optional, for xoxc tokens)").
				Description("xoxd- cookie value").
				Value(&cookie),
		),
	).Run()
	if err != nil {
		return err
	}

	workspace = slacksrc.SanitizeWorkspaceName(strings.TrimSpace(workspace))
	token = strings.TrimSpace(token)
	cookie = strings.TrimSpace(cookie)

	if workspace == "" {
		return fmt.Errorf("workspace name is required")
	}
	if err := slacksrc.ValidateToken(token); err != nil {
		return err
	}
	if strings.HasPrefix(token, "xoxc-") && cookie == "" {
		return fmt.Errorf("xoxc- tokens require a cookie (xoxd-) value")
	}

	client := slacksrc.NewClient(token, cookie)
	teamID, teamName, _, err := client.AuthTest(context.Background())
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	if err := slacksrc.SaveCredentials(workspace, token, cookie); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.Slack == nil {
		cfg.Slack = &config.SlackConfig{}
	}
	if cfg.Slack.Workspaces == nil {
		cfg.Slack.Workspaces = make(map[string]config.SlackWorkspace)
	}
	cfg.Slack.Workspaces[workspace] = config.SlackWorkspace{
		TeamID:   teamID,
		TeamName: teamName,
		AuthMode: "token",
	}
	if cfg.Slack.DefaultWorkspace == "" {
		cfg.Slack.DefaultWorkspace = workspace
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Authenticated with workspace %q (team: %s)\n", workspace, teamName)
	return nil
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout [workspace]",
	Short: "Remove Slack credentials",
	Example: `  obk slack auth logout
  obk slack auth logout my-company
  obk slack auth logout my-company --force`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		var workspace string
		if len(args) > 0 {
			workspace = args[0]
		} else if cfg.Slack != nil {
			workspace = cfg.Slack.DefaultWorkspace
		}
		if workspace == "" {
			return fmt.Errorf("specify a workspace name or configure a default")
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("About to remove credentials for workspace %q. Continue? (y/N): ", workspace)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		slacksrc.DeleteCredentials(workspace)

		if cfg.Slack != nil {
			delete(cfg.Slack.Workspaces, workspace)
			if cfg.Slack.DefaultWorkspace == workspace {
				cfg.Slack.DefaultWorkspace = ""
				for name := range cfg.Slack.Workspaces {
					cfg.Slack.DefaultWorkspace = name
					break
				}
			}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
		}

		fmt.Printf("Logged out of workspace %q\n", workspace)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "list",
	Short: "List Slack workspace credentials",
	Example: `  obk slack auth list
  obk slack auth list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		jsonOut, _ := cmd.Flags().GetBool("json")

		if cfg.Slack == nil || len(cfg.Slack.Workspaces) == 0 {
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode([]any{})
			}
			fmt.Println("No Slack workspaces configured.")
			fmt.Println("Run: obk slack auth login")
			return nil
		}

		type wsInfo struct {
			Name     string `json:"name"`
			TeamName string `json:"team_name"`
			AuthMode string `json:"auth_mode"`
			Status   string `json:"status"`
			Default  bool   `json:"default"`
		}
		var workspaces []wsInfo

		for name, ws := range cfg.Slack.Workspaces {
			creds, err := slacksrc.LoadCredentials(name)
			status := "valid"
			if err != nil {
				status = "no credentials"
			} else {
				client := slacksrc.NewClient(creds.Token, creds.Cookie)
				if _, _, _, err := client.AuthTest(context.Background()); err != nil {
					status = fmt.Sprintf("invalid (%v)", err)
				}
			}
			workspaces = append(workspaces, wsInfo{
				Name: name, TeamName: ws.TeamName, AuthMode: ws.AuthMode,
				Status: status, Default: name == cfg.Slack.DefaultWorkspace,
			})
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(workspaces)
		}

		fmt.Printf("Default workspace: %s\n\n", cfg.Slack.DefaultWorkspace)
		for _, ws := range workspaces {
			marker := " "
			if ws.Default {
				marker = "*"
			}
			fmt.Printf(" %s %s (team: %s, auth: %s) — %s\n", marker, ws.Name, ws.TeamName, ws.AuthMode, ws.Status)
		}
		return nil
	},
}

func init() {
	authLogoutCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	authStatusCmd.Flags().Bool("json", false, "Output as JSON")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

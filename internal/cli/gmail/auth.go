package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/tty"
	"github.com/73ai/openbotkit/oauth/google"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Google account authentication and scopes",
	RunE:  authInteractiveRun,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate a Google account via OAuth2",
	Example: `  obk gmail auth login --scopes gmail.readonly
  obk gmail auth login --scopes gmail.modify,calendar --email user@example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := config.EnsureProviderDir("google"); err != nil {
			return fmt.Errorf("create provider dir: %w", err)
		}

		scopeStr, _ := cmd.Flags().GetString("scopes")
		emailHint, _ := cmd.Flags().GetString("email")

		scopes := parseScopes(scopeStr)
		if len(scopes) == 0 {
			return authInteractiveRun(cmd, args)
		}

		gp := google.New(google.Config{
			CredentialsFile: cfg.GoogleCredentialsFile(),
			TokenDBPath:     cfg.GoogleTokenDBPath(),
		})

		ctx := context.Background()
		email, err := gp.GrantScopes(ctx, emailHint, expandScopes(scopes))
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		if err := config.LinkSource("gmail"); err != nil {
			return fmt.Errorf("link source: %w", err)
		}

		fmt.Printf("Authenticated as %s\n", email)
		fmt.Printf("Granted: %s\n", strings.Join(scopes, ", "))
		return nil
	},
}

var authRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke specific scopes for a Google account",
	Example: `  obk gmail auth revoke --email user@example.com --scopes gmail.readonly
  obk gmail auth revoke --email user@example.com --scopes gmail.modify,calendar --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		emailFlag, _ := cmd.Flags().GetString("email")
		scopeStr, _ := cmd.Flags().GetString("scopes")

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("About to revoke scopes for %s. Continue? (y/N): ", emailFlag)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if emailFlag == "" {
			return fmt.Errorf("--email is required")
		}
		scopes := parseScopes(scopeStr)
		if len(scopes) == 0 {
			return fmt.Errorf("--scopes is required")
		}

		gp := google.New(google.Config{
			CredentialsFile: cfg.GoogleCredentialsFile(),
			TokenDBPath:     cfg.GoogleTokenDBPath(),
		})

		ctx := context.Background()
		if err := gp.RevokeScopes(ctx, emailFlag, expandScopes(scopes)); err != nil {
			return fmt.Errorf("revoke failed: %w", err)
		}

		fmt.Printf("Revoked %s for %s\n", strings.Join(scopes, ", "), emailFlag)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "list",
	Short: "List authenticated Google accounts and scopes",
	Example: `  obk gmail auth list
  obk gmail auth list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		gp := google.New(google.Config{
			CredentialsFile: cfg.GoogleCredentialsFile(),
			TokenDBPath:     cfg.GoogleTokenDBPath(),
		})

		ctx := context.Background()
		accounts, err := gp.Accounts(ctx)
		if err != nil || len(accounts) == 0 {
			fmt.Println("No authenticated Google accounts.")
			return nil
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			type accountInfo struct {
				Email  string   `json:"email"`
				Scopes []string `json:"scopes"`
			}
			var infos []accountInfo
			for _, a := range accounts {
				scopes, _ := gp.GrantedScopes(ctx, a)
				infos = append(infos, accountInfo{Email: a, Scopes: scopes})
			}
			return json.NewEncoder(os.Stdout).Encode(infos)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ACCOUNT\tSCOPES")
		for _, a := range accounts {
			scopes, _ := gp.GrantedScopes(ctx, a)
			fmt.Fprintf(w, "%s\t%s\n", a, strings.Join(scopes, ", "))
		}
		return w.Flush()
	},
}

type scopeChoice struct {
	Label string
	Scope string
}

var availableScopeChoices = []scopeChoice{
	{Label: "Gmail (read)", Scope: "https://www.googleapis.com/auth/gmail.readonly"},
	{Label: "Gmail (read + write)", Scope: "https://www.googleapis.com/auth/gmail.modify"},
	{Label: "Calendar (read)", Scope: "https://www.googleapis.com/auth/calendar.readonly"},
	{Label: "Calendar (read + write)", Scope: "https://www.googleapis.com/auth/calendar"},
	{Label: "Drive", Scope: "https://www.googleapis.com/auth/drive"},
	{Label: "Drive (read)", Scope: "https://www.googleapis.com/auth/drive.readonly"},
	{Label: "Docs", Scope: "https://www.googleapis.com/auth/documents"},
	{Label: "Sheets", Scope: "https://www.googleapis.com/auth/spreadsheets"},
	{Label: "Tasks", Scope: "https://www.googleapis.com/auth/tasks"},
	{Label: "Contacts", Scope: "https://www.googleapis.com/auth/contacts"},
}

func authInteractiveRun(cmd *cobra.Command, args []string) error {
	if err := tty.RequireInteractive("obk gmail auth login --scopes gmail.readonly"); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := config.EnsureProviderDir("google"); err != nil {
		return fmt.Errorf("create provider dir: %w", err)
	}

	gp := google.New(google.Config{
		CredentialsFile: cfg.GoogleCredentialsFile(),
		TokenDBPath:     cfg.GoogleTokenDBPath(),
	})

	ctx := context.Background()
	accounts, _ := gp.Accounts(ctx)

	if len(accounts) == 0 {
		return authInteractiveNewAccount(ctx, gp)
	}

	return authInteractiveManage(ctx, gp, accounts)
}

func authInteractiveNewAccount(ctx context.Context, gp *google.Google) error {
	fmt.Print("\n  No Google accounts connected.\n\n")

	var selectedScopes []string
	options := make([]huh.Option[string], len(availableScopeChoices))
	for i, sc := range availableScopeChoices {
		options[i] = huh.NewOption(sc.Label, sc.Scope)
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select access to enable (space to toggle, enter to confirm)").
				Options(options...).
				Height(len(availableScopeChoices) + 2).
				Value(&selectedScopes),
		),
	).Run()
	if err != nil {
		return err
	}

	if len(selectedScopes) == 0 {
		fmt.Println("No scopes selected.")
		return nil
	}

	email, err := gp.GrantScopes(ctx, "", selectedScopes)
	if err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	fmt.Printf("\n  Authenticated as %s\n", email)
	for _, s := range selectedScopes {
		fmt.Printf("  ✓ %s enabled\n", scopeLabel(s))
	}
	return nil
}

func authInteractiveManage(ctx context.Context, gp *google.Google, accounts []string) error {
	fmt.Println("\n  Google accounts:")
	for _, a := range accounts {
		scopes, _ := gp.GrantedScopes(ctx, a)
		fmt.Printf("    %s\n", a)
		for _, s := range scopes {
			fmt.Printf("      ✓ %s\n", scopeLabel(s))
		}
	}
	fmt.Println()

	choices := make([]huh.Option[string], 0, len(accounts)+1)
	for _, a := range accounts {
		choices = append(choices, huh.NewOption("Manage access for "+a, a))
	}
	choices = append(choices, huh.NewOption("Add a new account", "__new__"))

	var action string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(choices...).
				Value(&action),
		),
	).Run()
	if err != nil {
		return err
	}

	if action == "__new__" {
		return authInteractiveNewAccount(ctx, gp)
	}

	existing, _ := gp.GrantedScopes(ctx, action)
	grantedSet := make(map[string]bool, len(existing))
	for _, s := range existing {
		grantedSet[s] = true
	}

	var selectedScopes []string
	options := make([]huh.Option[string], len(availableScopeChoices))
	for i, sc := range availableScopeChoices {
		label := sc.Label
		if grantedSet[sc.Scope] {
			label += " ✓ granted"
		}
		opt := huh.NewOption(label, sc.Scope)
		if grantedSet[sc.Scope] {
			opt = opt.Selected(true)
		}
		options[i] = opt
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Manage access for " + action + " (space to toggle, enter to confirm)").
				Options(options...).
				Height(len(availableScopeChoices) + 2).
				Value(&selectedScopes),
		),
	).Run()
	if err != nil {
		return err
	}

	managedSet := make(map[string]bool, len(availableScopeChoices))
	for _, sc := range availableScopeChoices {
		managedSet[sc.Scope] = true
	}

	newSet := make(map[string]bool, len(selectedScopes))
	for _, s := range selectedScopes {
		newSet[s] = true
	}

	var toAdd, toRemove []string
	for _, s := range selectedScopes {
		if !grantedSet[s] {
			toAdd = append(toAdd, s)
		}
	}
	for _, s := range existing {
		if !newSet[s] && managedSet[s] {
			toRemove = append(toRemove, s)
		}
	}

	if len(toAdd) > 0 {
		_, err := gp.GrantScopes(ctx, action, toAdd)
		if err != nil {
			return fmt.Errorf("grant scopes: %w", err)
		}
		for _, s := range toAdd {
			fmt.Printf("  ✓ %s added for %s\n", scopeLabel(s), action)
		}
	}

	if len(toRemove) > 0 {
		if err := gp.RevokeScopes(ctx, action, toRemove); err != nil {
			return fmt.Errorf("revoke scopes: %w", err)
		}
		for _, s := range toRemove {
			fmt.Printf("  ✗ %s removed for %s\n", scopeLabel(s), action)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		fmt.Println("  No changes.")
	}

	return nil
}

func scopeLabel(scope string) string {
	for _, sc := range availableScopeChoices {
		if sc.Scope == scope {
			return sc.Label
		}
	}
	return scope
}

func init() {
	authLoginCmd.Flags().String("scopes", "", "Comma-separated scopes (e.g. gmail.readonly,calendar.readonly)")
	authLoginCmd.Flags().String("email", "", "Email hint for account selection")

	authRevokeCmd.Flags().String("email", "", "Account email to revoke scopes for")
	authRevokeCmd.Flags().String("scopes", "", "Comma-separated scopes to revoke")
	authRevokeCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	authStatusCmd.Flags().Bool("json", false, "Output as JSON")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authRevokeCmd)
	authCmd.AddCommand(authStatusCmd)
}

var scopeAliases = map[string]string{
	"gmail.readonly":    "https://www.googleapis.com/auth/gmail.readonly",
	"gmail.compose":     "https://www.googleapis.com/auth/gmail.compose",
	"gmail.modify":      "https://www.googleapis.com/auth/gmail.modify",
	"calendar.readonly": "https://www.googleapis.com/auth/calendar.readonly",
	"calendar":          "https://www.googleapis.com/auth/calendar",
	"drive":             "https://www.googleapis.com/auth/drive",
	"drive.readonly":    "https://www.googleapis.com/auth/drive.readonly",
	"docs":              "https://www.googleapis.com/auth/documents",
	"sheets":            "https://www.googleapis.com/auth/spreadsheets",
	"tasks":             "https://www.googleapis.com/auth/tasks",
	"contacts":          "https://www.googleapis.com/auth/contacts",
}

func parseScopes(s string) []string {
	if s == "" {
		return nil
	}
	var scopes []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			scopes = append(scopes, part)
		}
	}
	return scopes
}

func expandScopes(scopes []string) []string {
	expanded := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if full, ok := scopeAliases[s]; ok {
			expanded = append(expanded, full)
		} else {
			expanded = append(expanded, s)
		}
	}
	return expanded
}

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/internal/tty"
	"github.com/priyanshujain/openbotkit/provider/google"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Guided first-time setup for OpenBotKit",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tty.RequireInteractive("configure manually with 'obk config' and 'obk auth google login'"); err != nil {
			return err
		}

		fmt.Print("\n  Welcome to OpenBotKit setup!\n\n")

		// Step 1: Source selection.
		var sources []string
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Which sources would you like to set up?").
					Options(
						huh.NewOption("Gmail", "gmail"),
						huh.NewOption("WhatsApp", "whatsapp"),
					).
					Value(&sources),
			),
		).Run()
		if err != nil {
			return err
		}

		if len(sources) == 0 {
			fmt.Println("No sources selected. You can run setup again later.")
			return nil
		}

		needsGoogle := false
		for _, s := range sources {
			if s == "gmail" {
				needsGoogle = true
				break
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if needsGoogle {
			if err := setupGoogle(cfg); err != nil {
				return err
			}
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Println("\n  Setup complete!")
		fmt.Println("  Next steps:")
		for _, s := range sources {
			switch s {
			case "gmail":
				fmt.Println("    - Run: obk gmail sync")
			case "whatsapp":
				fmt.Println("    - Run: obk auth whatsapp login")
			}
		}
		return nil
	},
}

func setupGoogle(cfg *config.Config) error {
	if err := config.EnsureProviderDir("google"); err != nil {
		return fmt.Errorf("create provider dir: %w", err)
	}

	// Step 2: Credentials path.
	defaultCredPath := cfg.GoogleCredentialsFile()
	var credPath string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Path to Google OAuth credentials.json").
				Description("Drag and drop the file here, or type the path").
				Placeholder(defaultCredPath).
				Value(&credPath),
		),
	).Run()
	if err != nil {
		return err
	}

	credPath = cleanPath(credPath)
	if credPath == "" {
		credPath = defaultCredPath
	}

	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s", credPath)
	}

	// Save to config.
	if cfg.Providers == nil {
		cfg.Providers = &config.ProvidersConfig{}
	}
	if cfg.Providers.Google == nil {
		cfg.Providers.Google = &config.GoogleProviderConfig{}
	}
	cfg.Providers.Google.CredentialsFile = credPath

	// Step 3: Scope selection.
	var scopes []string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select Gmail access level").
				Options(
					huh.NewOption("Gmail (read)", "https://www.googleapis.com/auth/gmail.readonly").Selected(true),
					huh.NewOption("Gmail (read + write)", "https://www.googleapis.com/auth/gmail.modify"),
				).
				Value(&scopes),
		),
	).Run()
	if err != nil {
		return err
	}

	if len(scopes) == 0 {
		scopes = []string{"https://www.googleapis.com/auth/gmail.readonly"}
	}

	// Step 4: OAuth flow.
	gp := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     cfg.GoogleTokenDBPath(),
	})

	ctx := context.Background()
	email, err := gp.GrantScopes(ctx, "", scopes)
	if err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	fmt.Printf("\n  Authenticated as %s\n", email)

	// Step 5: Sync window.
	var syncDays string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How much email history to sync?").
				Options(
					huh.NewOption("Last 7 days", "7"),
					huh.NewOption("Last 30 days", "30"),
					huh.NewOption("Everything", "0"),
				).
				Value(&syncDays),
		),
	).Run()
	if err != nil {
		return err
	}

	fmt.Printf("  Sync window: %s days\n", syncDays)
	return nil
}

// cleanPath handles drag-and-drop paths that may have quotes and whitespace.
func cleanPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "'\"")
	s = strings.TrimSpace(s)
	return s
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/internal/skills"
	"github.com/priyanshujain/openbotkit/internal/tty"
	"github.com/priyanshujain/openbotkit/provider/google"
	ansrc "github.com/priyanshujain/openbotkit/source/applenotes"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

// gwsServices lists the Google Workspace services available via gws.
var gwsServices = []string{"calendar", "drive", "docs", "sheets", "tasks", "people"}

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
		sourceOptions := []huh.Option[string]{
			huh.NewOption("Gmail", "gmail"),
			huh.NewOption("WhatsApp", "whatsapp"),
			huh.NewOption("Google Calendar", "calendar"),
			huh.NewOption("Google Drive", "drive"),
			huh.NewOption("Google Docs", "docs"),
			huh.NewOption("Google Sheets", "sheets"),
			huh.NewOption("Google Tasks", "tasks"),
			huh.NewOption("Google Contacts", "people"),
		}
		if runtime.GOOS == "darwin" {
			sourceOptions = append(sourceOptions, huh.NewOption("Apple Notes", "applenotes"))
		}

		err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Which sources would you like to set up?").
					Options(sourceOptions...).
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
		var selectedGWS []string
		for _, s := range sources {
			if s == "gmail" {
				needsGoogle = true
			}
			if isGWSService(s) {
				needsGoogle = true
				selectedGWS = append(selectedGWS, s)
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

		if len(selectedGWS) > 0 {
			if err := setupGWS(cfg, selectedGWS); err != nil {
				return err
			}
		}

		for _, s := range sources {
			if s == "applenotes" {
				if err := setupAppleNotes(cfg); err != nil {
					return err
				}
			}
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		// Install skills based on current auth state.
		fmt.Println("\n  -- Installing skills --")
		result, err := skills.Install(cfg)
		if err != nil {
			return fmt.Errorf("install skills: %w", err)
		}
		for _, name := range result.Installed {
			fmt.Printf("  + %s\n", name)
		}
		for _, name := range result.Skipped {
			fmt.Printf("  - %s (skipped)\n", name)
		}
		for _, name := range result.Removed {
			fmt.Printf("  x %s (removed)\n", name)
		}
		fmt.Printf("  %d skills installed to %s\n", len(result.Installed), skills.SkillsDir())

		fmt.Println("\n  Setup complete!")
		fmt.Println("  Next steps:")
		for _, s := range sources {
			switch s {
			case "gmail":
				fmt.Println("    - Run: obk gmail sync")
			case "whatsapp":
				fmt.Println("    - Run: obk auth whatsapp login")
			case "applenotes":
				fmt.Println("    - Apple Notes is ready (synced during setup)")
			}
		}
		return nil
	},
}

func setupGoogle(cfg *config.Config) error {
	if err := config.EnsureProviderDir("google"); err != nil {
		return fmt.Errorf("create provider dir: %w", err)
	}

	// Credentials path.
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

	// Scope selection.
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

	// OAuth flow.
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

	// Sync window.
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

func setupGWS(cfg *config.Config, services []string) error {
	fmt.Printf("\n  -- Google Workspace Setup (%s) --\n", strings.Join(services, ", "))
	fmt.Println("  These services are powered by gws (Google Workspace CLI).")

	// Check for gws binary.
	gwsPath, err := exec.LookPath("gws")
	if err != nil {
		fmt.Println("\n  Checking for gws... not found.")
		fmt.Println("  Install gws (requires Node.js):")
		fmt.Println("    npm install -g @googleworkspace/cli")
		fmt.Println("\n  Waiting for gws to be installed... (run the command above in another tab)")
		fmt.Println("  Press Ctrl+C to cancel.")

		const maxAttempts = 60 // 5 minutes
		for attempt := range maxAttempts {
			time.Sleep(5 * time.Second)
			gwsPath, err = exec.LookPath("gws")
			if err == nil {
				break
			}
			fmt.Println("  Checking... not found")
			if attempt == maxAttempts-1 {
				return fmt.Errorf("gws not found after %d attempts — install it and re-run obk setup", maxAttempts)
			}
		}
		fmt.Printf("  Checking... found gws at %s\n", gwsPath)
	} else {
		fmt.Printf("  gws found at %s\n", gwsPath)
	}

	// Share credentials with gws.
	credPath := cfg.GoogleCredentialsFile()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	gwsCredDir := filepath.Join(home, ".config", "gws")
	gwsCredPath := filepath.Join(gwsCredDir, "client_secret.json")

	if err := os.MkdirAll(gwsCredDir, 0700); err != nil {
		return fmt.Errorf("create gws config dir: %w", err)
	}

	credData, err := os.ReadFile(credPath)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}
	if err := os.WriteFile(gwsCredPath, credData, 0600); err != nil {
		return fmt.Errorf("copy credentials to gws: %w", err)
	}
	fmt.Printf("  Shared credentials with gws (%s)\n", gwsCredPath)

	// Run gws auth login.
	scopeArg := strings.Join(services, ",")
	fmt.Println("\n  Opening browser for Google Workspace access...")
	authCmd := exec.Command(gwsPath, "auth", "login", "--scopes", scopeArg)
	authCmd.Stdout = os.Stdout
	authCmd.Stderr = os.Stderr
	authCmd.Stdin = os.Stdin
	if err := authCmd.Run(); err != nil {
		return fmt.Errorf("gws auth login: %w", err)
	}
	fmt.Println("  Google Workspace authenticated.")

	// Save to config.
	if cfg.Integrations == nil {
		cfg.Integrations = &config.IntegrationsConfig{}
	}
	if cfg.Integrations.GWS == nil {
		cfg.Integrations.GWS = &config.GWSConfig{}
	}
	cfg.Integrations.GWS.Enabled = true

	// Merge with existing services.
	existing := make(map[string]bool)
	for _, s := range cfg.Integrations.GWS.Services {
		existing[s] = true
	}
	for _, s := range services {
		if !existing[s] {
			cfg.Integrations.GWS.Services = append(cfg.Integrations.GWS.Services, s)
		}
	}

	return nil
}

func setupAppleNotes(cfg *config.Config) error {
	if err := config.EnsureSourceDir("applenotes"); err != nil {
		return fmt.Errorf("create applenotes dir: %w", err)
	}

	fmt.Println("\n  Setting up Apple Notes...")
	fmt.Println("  Running initial sync (this may take a few seconds)...")

	db, err := store.Open(store.Config{
		Driver: cfg.AppleNotes.Storage.Driver,
		DSN:    cfg.AppleNotesDataDSN(),
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	result, err := ansrc.Sync(db, ansrc.SyncOptions{})
	if err != nil {
		return fmt.Errorf("apple notes sync: %w", err)
	}

	if err := config.LinkSource("applenotes"); err != nil {
		return fmt.Errorf("link source: %w", err)
	}

	fmt.Printf("  Synced %d notes\n", result.Synced)
	return nil
}

func isGWSService(s string) bool {
	for _, svc := range gwsServices {
		if s == svc {
			return true
		}
	}
	return false
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

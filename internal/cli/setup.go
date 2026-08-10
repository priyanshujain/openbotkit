package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/browser/cookies"
	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/internal/tty"
	"github.com/73ai/openbotkit/oauth/google"
	"github.com/73ai/openbotkit/provider"
	"github.com/73ai/openbotkit/remote"
	backupsvc "github.com/73ai/openbotkit/service/backup"
	contactsrc "github.com/73ai/openbotkit/service/contacts"
	ansrc "github.com/73ai/openbotkit/source/applenotes"
	imsrc "github.com/73ai/openbotkit/source/imessage"
	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/73ai/openbotkit/source/slack/desktop"
	xclient "github.com/73ai/openbotkit/source/twitter/client"
	"github.com/73ai/openbotkit/store"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var gwsServices = []string{"calendar", "drive", "docs", "sheets", "tasks", "people"}

var setupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "Guided first-time setup for OpenBotKit",
	Example: `  obk setup`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tty.RequireInteractive("configure manually with 'obk config' and 'obk gmail auth login'"); err != nil {
			return err
		}

		fmt.Print("\n  Welcome to OpenBotKit setup!\n\n")

		var mode string
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("How will you deploy OpenBotKit?").
					Options(
						huh.NewOption("Local (everything runs on this machine)", "local"),
						huh.NewOption("Remote (server in Docker, this machine as controller)", "remote"),
					).
					Value(&mode),
			),
		).Run()
		if err != nil {
			return err
		}

		if mode == "remote" {
			return setupRemote()
		}

		var sources []string
		sourceOptions := []huh.Option[string]{
			huh.NewOption("LLM Models (for obk chat)", "models"),
			huh.NewOption("Telegram Bot (talk to your assistant)", "telegram"),
			huh.NewOption("Telegram (your account, read + send messages)", "telegram_source"),
			huh.NewOption("Gmail", "gmail"),
			huh.NewOption("WhatsApp", "whatsapp"),
			huh.NewOption("Google Calendar", "calendar"),
			huh.NewOption("Google Drive", "drive"),
			huh.NewOption("Google Docs", "docs"),
			huh.NewOption("Google Sheets", "sheets"),
			huh.NewOption("Google Tasks", "tasks"),
			huh.NewOption("Google Contacts", "people"),
		}
		sourceOptions = append(sourceOptions, huh.NewOption("X (Twitter)", "x"))
		sourceOptions = append(sourceOptions, huh.NewOption("Slack", "slack"))
		sourceOptions = append(sourceOptions, huh.NewOption("Backup (Cloudflare R2 or Google Drive)", "backup"))
		if runtime.GOOS == "darwin" {
			sourceOptions = append(sourceOptions, huh.NewOption("Apple Notes", "applenotes"))
			sourceOptions = append(sourceOptions, huh.NewOption("Apple Contacts", "applecontacts"))
			sourceOptions = append(sourceOptions, huh.NewOption("iMessage", "imessage"))
		}

		err = huh.NewForm(
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
		needsTelegram := false
		var selectedGWS []string
		for _, s := range sources {
			if s == "gmail" {
				needsGoogle = true
			}
			if s == "telegram" {
				needsTelegram = true
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

		// Step 1: LLM models (needed for Telegram bot).
		for _, s := range sources {
			if s == "models" {
				if err := setupModels(cfg); err != nil {
					return err
				}
			}
		}

		// Step 2: Telegram bot setup.
		if needsTelegram {
			if err := setupTelegram(cfg); err != nil {
				return err
			}
		}

		// Step 3: If Telegram is configured and Google sources are selected,
		// set up ngrok first so we know the callback URL before creating
		// Google OAuth credentials.
		hasTelegram := cfg.Channels != nil && cfg.Channels.Telegram != nil && cfg.Channels.Telegram.BotToken != ""
		if hasTelegram && needsGoogle {
			if err := setupNgrok(cfg); err != nil {
				return err
			}
		}

		// Step 4: Google OAuth credentials + authentication.
		if needsGoogle {
			if err := setupGoogle(cfg); err != nil {
				return err
			}
		}

		// Step 5: GWS services (incremental scope grant).
		if len(selectedGWS) > 0 {
			if err := setupGWS(cfg, selectedGWS); err != nil {
				return err
			}
		}

		// Step 6: Other sources.
		for _, s := range sources {
			switch s {
			case "applenotes":
				if err := setupAppleNotes(cfg); err != nil {
					return err
				}
			case "applecontacts":
				if err := setupAppleContacts(cfg); err != nil {
					return err
				}
			case "imessage":
				if err := setupIMessage(cfg); err != nil {
					return err
				}
			case "whatsapp":
				if err := config.EnsureSourceDir("whatsapp"); err != nil {
					return fmt.Errorf("create whatsapp dir: %w", err)
				}
				fmt.Println("\n  WhatsApp requires QR code login.")
				fmt.Println("  Run after setup: obk whatsapp auth login")
			case "telegram_source":
				if err := setupTelegramSource(cfg); err != nil {
					return err
				}
			case "x":
				if err := setupX(cfg); err != nil {
					return err
				}
			case "slack":
				if err := setupSlack(cfg); err != nil {
					return err
				}
			case "backup":
				if err := setupBackup(cfg); err != nil {
					return err
				}
			}
		}

		if cfg.Timezone == "" {
			systemTZ := time.Now().Location().String()
			var tz string
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Timezone").
						Description("Used for scheduling and display").
						Placeholder(systemTZ).
						Value(&tz),
				),
			).Run()
			if err != nil {
				return err
			}
			tz = strings.TrimSpace(tz)
			if tz == "" {
				tz = systemTZ
			}
			if _, err := time.LoadLocation(tz); err != nil {
				fmt.Printf("  Invalid timezone %q, using %s\n", tz, systemTZ)
				tz = systemTZ
			}
			cfg.Timezone = tz
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

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
				fmt.Println("    - Run: obk whatsapp auth login")
			case "applenotes":
				fmt.Println("    - Apple Notes is ready (synced during setup)")
			case "applecontacts":
				fmt.Println("    - Apple Contacts is ready (synced during setup)")
			case "imessage":
				fmt.Println("    - iMessage is ready (synced during setup)")
			case "slack":
				fmt.Println("    - Slack is ready! Try: obk slack channels")
			case "telegram":
				fmt.Println("    - Telegram bot is ready! Send it a message.")
			case "telegram_source":
				fmt.Println("    - Run: obk telegram auth login")
			case "backup":
				fmt.Println("    - Backup configured! Run `obk backup now` for your first backup.")
			}
		}
		return nil
	},
}

func setupRemote() error {
	fmt.Println("\n  -- Remote Deployment Setup --")
	fmt.Println("  Deploy the server using Docker:")
	fmt.Println("    docker compose -f infrastructure/docker/docker-compose.yml up -d")
	fmt.Println()

	var serverURL, username, password string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Remote server URL").
				Placeholder("http://your-server:8443").
				Value(&serverURL),
			huh.NewInput().
				Title("Username").
				Value(&username),
			huh.NewInput().
				Title("Password").
				EchoMode(huh.EchoModePassword).
				Value(&password),
		),
	).Run()
	if err != nil {
		return err
	}

	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return fmt.Errorf("server URL is required")
	}

	fmt.Print("  Testing connection... ")
	client := remote.NewClient(serverURL, username, password)
	if _, err := client.Health(); err != nil {
		fmt.Println("failed!")
		return fmt.Errorf("cannot reach server: %w", err)
	}
	fmt.Println("ok!")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg.Mode = config.ModeRemote
	passwordRef := "keychain:obk/remote"
	if err := provider.StoreCredential(passwordRef, password); err != nil {
		fmt.Printf("  Warning: could not store password in keychain: %v\n", err)
		cfg.Remote = &config.RemoteConfig{
			Server:   serverURL,
			Username: username,
			Password: password,
		}
	} else {
		cfg.Remote = &config.RemoteConfig{
			Server:      serverURL,
			Username:    username,
			PasswordRef: passwordRef,
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println("\n  Remote setup complete!")
	fmt.Println("  Your CLI commands will now proxy to the remote server.")
	return nil
}

func setupTelegram(cfg *config.Config) error {
	fmt.Println("\n  -- Telegram Bot Setup --")

	var botToken string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Telegram bot token").
				Description("Create a bot via @BotFather on Telegram and paste the token").
				Value(&botToken),
		),
	).Run()
	if err != nil {
		return err
	}
	botToken = strings.TrimSpace(botToken)
	if botToken == "" {
		return fmt.Errorf("bot token is required")
	}

	var ownerIDStr string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Your Telegram user ID").
				Description("Send /start to @userinfobot to get your ID").
				Value(&ownerIDStr),
		),
	).Run()
	if err != nil {
		return err
	}
	ownerIDStr = strings.TrimSpace(ownerIDStr)
	if ownerIDStr == "" {
		return fmt.Errorf("owner ID is required")
	}

	var ownerID int64
	if _, err := fmt.Sscanf(ownerIDStr, "%d", &ownerID); err != nil {
		return fmt.Errorf("invalid owner ID %q: must be a number", ownerIDStr)
	}

	if cfg.Channels == nil {
		cfg.Channels = &config.ChannelsConfig{}
	}
	if cfg.Channels.Telegram == nil {
		cfg.Channels.Telegram = &config.TelegramConfig{}
	}
	cfg.Channels.Telegram.BotToken = botToken
	cfg.Channels.Telegram.OwnerID = ownerID

	fmt.Printf("  Telegram bot configured (owner: %d)\n", ownerID)
	return nil
}

func setupGoogle(cfg *config.Config) error {
	if err := config.EnsureProviderDir("google"); err != nil {
		return fmt.Errorf("create provider dir: %w", err)
	}

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

	if cfg.Providers == nil {
		cfg.Providers = &config.ProvidersConfig{}
	}
	if cfg.Providers.Google == nil {
		cfg.Providers.Google = &config.GoogleProviderConfig{}
	}
	cfg.Providers.Google.CredentialsFile = credPath

	var scope string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Gmail access level").
				Options(
					huh.NewOption("Read only", "https://www.googleapis.com/auth/gmail.readonly"),
					huh.NewOption("Read + Write", "https://www.googleapis.com/auth/gmail.modify"),
				).
				Value(&scope),
		),
	).Run()
	if err != nil {
		return err
	}

	scopes := []string{scope}

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

	var syncDaysInt int
	if _, err := fmt.Sscanf(syncDays, "%d", &syncDaysInt); err == nil {
		cfg.Gmail.SyncDays = syncDaysInt
	}

	fmt.Printf("  Sync window: %s days\n", syncDays)
	return nil
}

func setupGWS(cfg *config.Config, services []string) error {
	fmt.Printf("\n  -- Google Workspace Setup (%s) --\n", strings.Join(services, ", "))
	fmt.Println("  These services are powered by gws (Google Workspace CLI).")

	gwsPath, err := exec.LookPath("gws")
	if err != nil {
		fmt.Println("\n  Checking for gws... not found.")
		fmt.Println("  Install gws (requires Node.js):")
		fmt.Println("    npm install -g @googleworkspace/cli")

		var choice string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("gws is not installed").
				Options(
					huh.NewOption("Wait for install (run the command above in another tab)", "wait"),
					huh.NewOption("Skip GWS setup for now", "skip"),
				).
				Value(&choice),
		)).Run(); err != nil {
			return err
		}
		if choice == "skip" {
			fmt.Println("  Skipping GWS setup. You can configure it later with: obk setup")
			return nil
		}

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

	// Authenticate via obk's own OAuth instead of gws auth login.
	scopes := gwsScopesForServices(services)
	gp := google.New(google.Config{
		CredentialsFile: cfg.GoogleCredentialsFile(),
		TokenDBPath:     cfg.GoogleTokenDBPath(),
	})

	// Use existing account if one exists (incremental grant).
	accounts, _ := gp.Accounts(context.Background())
	var account string
	if len(accounts) > 0 {
		account = accounts[0]
	}

	email, err := gp.GrantScopes(context.Background(), account, scopes)
	if err != nil {
		return fmt.Errorf("google auth: %w", err)
	}
	fmt.Printf("  Google Workspace authenticated as %s\n", email)

	if cfg.Integrations == nil {
		cfg.Integrations = &config.IntegrationsConfig{}
	}
	if cfg.Integrations.GWS == nil {
		cfg.Integrations.GWS = &config.GWSConfig{}
	}
	cfg.Integrations.GWS.Enabled = true

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
	fmt.Println("\n  Setting up Apple Notes...")
	fmt.Println("  macOS will ask for permission to access Notes.")
	fmt.Println("  Click \"OK\" to grant access.")
	fmt.Println()

	if err := ansrc.CheckPermission(); err != nil {
		fmt.Println("  Permission denied or Notes not accessible.")
		fmt.Println("  Grant access in System Settings > Privacy & Security > Automation.")
		fmt.Println("  Then re-run: obk setup")
		return fmt.Errorf("apple notes permission: %w", err)
	}

	fmt.Println("  Permission granted. Running initial sync...")

	if err := config.EnsureSourceDir("applenotes"); err != nil {
		return fmt.Errorf("create applenotes dir: %w", err)
	}

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

func setupAppleContacts(cfg *config.Config) error {
	fmt.Println("\n  Setting up Apple Contacts...")
	fmt.Println("  macOS will ask for permission to access Contacts.")
	fmt.Println("  Click \"OK\" to grant access.")
	fmt.Println()

	if err := contactsrc.CheckAppleContactsPermission(); err != nil {
		fmt.Println("  Permission denied or Contacts not accessible.")
		fmt.Println("  Grant access in System Settings > Privacy & Security > Contacts.")
		fmt.Println("  Then re-run: obk setup")
		return fmt.Errorf("apple contacts permission: %w", err)
	}

	fmt.Println("  Permission granted. Running initial sync...")

	if err := config.EnsureSourceDir("contacts"); err != nil {
		return fmt.Errorf("create contacts dir: %w", err)
	}

	db, err := store.Open(store.Config{
		Driver: cfg.Contacts.Storage.Driver,
		DSN:    cfg.ContactsDataDSN(),
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	result, err := contactsrc.Sync(db, nil, contactsrc.SyncOptions{
		Sources: []string{"applecontacts"},
	})
	if err != nil {
		return fmt.Errorf("apple contacts sync: %w", err)
	}

	if err := config.LinkSource("applecontacts"); err != nil {
		return fmt.Errorf("link source: %w", err)
	}

	fmt.Printf("  Synced %d contacts (%d new, %d linked)\n", result.Created+result.Linked, result.Created, result.Linked)
	return nil
}

func setupIMessage(cfg *config.Config) error {
	fmt.Println("\n  Setting up iMessage...")
	fmt.Println("  iMessage requires Full Disk Access to read the chat database.")
	fmt.Println("  Grant access in System Settings > Privacy & Security > Full Disk Access.")
	fmt.Println()

	if err := config.EnsureSourceDir("imessage"); err != nil {
		return fmt.Errorf("create imessage dir: %w", err)
	}

	db, err := store.Open(store.Config{
		Driver: cfg.IMessage.Storage.Driver,
		DSN:    cfg.IMessageDataDSN(),
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	result, err := imsrc.Sync(db, imsrc.SyncOptions{})
	if err != nil {
		return fmt.Errorf("imessage sync: %w", err)
	}

	if err := config.LinkSource("imessage"); err != nil {
		return fmt.Errorf("link source: %w", err)
	}

	fmt.Printf("  Synced %d messages\n", result.Synced)
	return nil
}

func setupX(cfg *config.Config) error {
	fmt.Println("\n  -- X (Twitter) Setup --")

	available := cookies.AvailableBrowsers()
	var opts []huh.Option[string]
	for _, b := range available {
		label := b
		if b == "Safari" {
			label = "Safari (requires Full Disk Access for terminal)"
		}
		opts = append(opts, huh.NewOption(label, b))
	}

	var browser string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which browser are you signed in to x.com on?").
				Options(opts...).
				Value(&browser),
		),
	).Run()
	if err != nil {
		return err
	}

	if browser == "Safari" {
		fmt.Println()
		fmt.Println("  Note: Safari cookies are protected by macOS.")
		fmt.Println("  Your terminal app needs Full Disk Access to read them.")
		fmt.Println("  If this fails, grant access in System Settings > Privacy & Security > Full Disk Access.")
		fmt.Println()
	}

	if browser == "Chrome" {
		fmt.Println("  macOS will ask for permission to access \"Chrome Safe Storage\" in your keychain.")
		fmt.Println("  Click \"Always Allow\" so you won't be prompted again.")
		fmt.Println()
	}

	fmt.Printf("  Extracting X session from %s...\n", browser)

	session, _, err := xclient.ExtractSessionFromBrowserByName(browser)
	if err != nil {
		if browser == "Safari" && cookies.IsPermissionError(err) {
			fmt.Println()
			fmt.Println("  Safari access was denied.")
			fmt.Println("  To fix: System Settings > Privacy & Security > Full Disk Access")
			fmt.Println("  Add your terminal app, restart it, and run setup again.")
			fmt.Println()
		} else {
			fmt.Printf("  Failed: %v\n", err)
			fmt.Println("  Make sure you're signed in to x.com in that browser.")
		}
		fmt.Println("  You can try again later: obk x auth login")
		return nil
	}

	if err := xclient.SaveSession(session); err != nil {
		return fmt.Errorf("save X credentials: %w", err)
	}

	if cfg.X == nil {
		cfg.X = &config.XConfig{}
	}

	if err := config.LinkSource("x"); err != nil {
		return fmt.Errorf("link X source: %w", err)
	}

	fmt.Printf("  Authenticated with X (from %s)!\n", browser)
	return nil
}

func setupSlack(cfg *config.Config) error {
	fmt.Println("\n  -- Slack Setup --")

	if runtime.GOOS == "darwin" {
		fmt.Println("  Detecting Slack Desktop credentials...")
		fmt.Println("  macOS will ask for permission to access \"Slack Safe Storage\" in your keychain.")
		fmt.Println("  Click \"Always Allow\" so you won't be prompted again.")
		fmt.Println()

		creds, err := desktop.Extract()
		if err == nil {
			workspace := slacksrc.SanitizeWorkspaceName(creds.TeamName)
			if err := slacksrc.SaveCredentials(workspace, creds.Token, creds.Cookie); err != nil {
				return fmt.Errorf("save credentials: %w", err)
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
			fmt.Printf("  Authenticated with workspace %q (team: %s)\n", workspace, creds.TeamName)
			return nil
		}
		fmt.Printf("  Desktop auto-detect failed: %v\n", err)
		fmt.Println("  Falling back to manual token entry.")
	}

	fmt.Println("  Run: obk slack auth login")
	return nil
}

func setupBackup(cfg *config.Config) error {
	fmt.Println("\n  -- Backup Setup --")

	var destination string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where would you like to back up to?").
				Options(
					huh.NewOption("Cloudflare R2 (S3-compatible)", "r2"),
					huh.NewOption("Google Drive", "gdrive"),
				).
				Value(&destination),
		),
	).Run()
	if err != nil {
		return err
	}

	if cfg.Backup == nil {
		cfg.Backup = &config.BackupConfig{}
	}
	cfg.Backup.Destination = destination

	switch destination {
	case "r2":
		if err := setupBackupR2(cfg); err != nil {
			return err
		}
	case "gdrive":
		if err := setupBackupGDrive(cfg); err != nil {
			return err
		}
	}

	cfg.Backup.Enabled = true
	cfg.Backup.Schedule = "6h"

	if err := config.EnsureSourceDir("backup"); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	if err := config.LinkSource("backup"); err != nil {
		return fmt.Errorf("link source: %w", err)
	}

	fmt.Println("  Backup configured!")
	return nil
}

func setupBackupR2(cfg *config.Config) error {
	var bucket, endpoint, accessKey, secretKey string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("R2 Bucket name").
				Description("Cloudflare Dashboard → R2 Object Storage → your bucket").
				Value(&bucket),
			huh.NewInput().
				Title("R2 Endpoint").
				Description("Bucket → Settings → S3 API → copy the endpoint URL").
				Placeholder("https://<account-id>.r2.cloudflarestorage.com").
				Value(&endpoint),
			huh.NewInput().
				Title("Access Key ID").
				Description("R2 → Manage R2 API Tokens → Create API Token").
				Value(&accessKey),
			huh.NewInput().
				Title("Secret Access Key").
				Description("Shown once when you create the API token above").
				EchoMode(huh.EchoModePassword).
				Value(&secretKey),
		),
	).Run()
	if err != nil {
		return err
	}

	bucket = strings.TrimSpace(bucket)
	endpoint = strings.TrimSpace(endpoint)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)

	if bucket == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		return fmt.Errorf("all R2 fields are required")
	}

	fmt.Print("  Validating R2 connection... ")
	ctx := context.Background()
	if err := backupsvc.ValidateR2(ctx, endpoint, accessKey, secretKey, bucket); err != nil {
		fmt.Println("failed!")
		return fmt.Errorf("R2 validation: %w", err)
	}
	fmt.Println("ok!")

	accessKeyRef := "keychain:obk/r2-access-key"
	secretKeyRef := "keychain:obk/r2-secret-key"

	if err := provider.StoreCredential(accessKeyRef, accessKey); err != nil {
		return fmt.Errorf("store R2 access key: %w", err)
	}
	if err := provider.StoreCredential(secretKeyRef, secretKey); err != nil {
		return fmt.Errorf("store R2 secret key: %w", err)
	}

	if cfg.Backup.R2 == nil {
		cfg.Backup.R2 = &config.R2Config{}
	}
	cfg.Backup.R2.Bucket = bucket
	cfg.Backup.R2.Endpoint = endpoint
	cfg.Backup.R2.AccessKeyRef = accessKeyRef
	cfg.Backup.R2.SecretKeyRef = secretKeyRef

	return nil
}

func setupBackupGDrive(cfg *config.Config) error {
	gp := google.New(google.Config{
		CredentialsFile: cfg.GoogleCredentialsFile(),
		TokenDBPath:     cfg.GoogleTokenDBPath(),
	})

	ctx := context.Background()

	accounts, _ := gp.Accounts(ctx)
	var account string
	if len(accounts) > 0 {
		account = accounts[0]
	}

	scopes := []string{"https://www.googleapis.com/auth/drive.file"}

	if account != "" {
		granted, err := gp.GrantedScopes(ctx, account)
		if err == nil {
			hasDriveScope := false
			for _, s := range granted {
				if s == "https://www.googleapis.com/auth/drive.file" {
					hasDriveScope = true
					break
				}
			}
			if !hasDriveScope {
				fmt.Println("  Granting Google Drive file scope...")
				_, err := gp.GrantScopes(ctx, account, scopes)
				if err != nil {
					return fmt.Errorf("grant Drive scope: %w", err)
				}
			}
		}
	} else {
		fmt.Println("  No Google account found. Starting OAuth flow...")
		email, err := gp.GrantScopes(ctx, "", scopes)
		if err != nil {
			return fmt.Errorf("Google auth: %w", err)
		}
		account = email
		fmt.Printf("  Authenticated as %s\n", email)
	}

	folderName := "obk-backup"

	httpClient, err := gp.Client(ctx, account, scopes)
	if err != nil {
		return fmt.Errorf("get Drive client: %w", err)
	}

	fmt.Print("  Finding or creating folder... ")
	folderID, err := backupsvc.FindOrCreateDriveFolder(ctx, httpClient, folderName)
	if err != nil {
		fmt.Println("failed!")
		return fmt.Errorf("create Drive folder: %w", err)
	}
	fmt.Printf("ok! (ID: %s)\n", folderID)

	if cfg.Backup.GDrive == nil {
		cfg.Backup.GDrive = &config.GDriveConfig{}
	}
	cfg.Backup.GDrive.FolderID = folderID

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

// gwsScopesForServices maps service names to Google OAuth scopes.
func gwsScopesForServices(services []string) []string {
	scopeMap := map[string]string{
		"calendar": "https://www.googleapis.com/auth/calendar",
		"drive":    "https://www.googleapis.com/auth/drive",
		"docs":     "https://www.googleapis.com/auth/documents",
		"sheets":   "https://www.googleapis.com/auth/spreadsheets",
		"tasks":    "https://www.googleapis.com/auth/tasks",
		"people":   "https://www.googleapis.com/auth/contacts",
	}
	var scopes []string
	for _, svc := range services {
		if s, ok := scopeMap[svc]; ok {
			scopes = append(scopes, s)
		}
	}
	return scopes
}

func cleanPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "'\"")
	s = strings.TrimSpace(s)
	return s
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

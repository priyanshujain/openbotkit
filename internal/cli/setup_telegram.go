package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var setupTelegramSourceCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Set up the Telegram data source",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return setupTelegramSource(cfg)
	},
}

// setupTelegramSource collects the user's own api_id/api_hash. Shipping a
// shared pair would earn API_ID_PUBLISHED_FLOOD for everyone, and a single
// revocation would take the integration down for every install.
func setupTelegramSource(cfg *config.Config) error {
	fmt.Println("\n  -- Telegram Setup --")
	fmt.Println()
	fmt.Println("  Telegram needs an app of your own, which takes a minute to create:")
	fmt.Println("    1. Open https://my.telegram.org and sign in with your phone number")
	fmt.Println("    2. Go to \"API development tools\"")
	fmt.Println("    3. Create an app (any title and short name will do)")
	fmt.Println("    4. Copy the api_id and api_hash it shows you")
	fmt.Println()

	var apiID, apiHash string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("api_id").
				Description("The numeric ID from my.telegram.org").
				Value(&apiID),
			huh.NewInput().
				Title("api_hash").
				Description("The 32-character hash shown next to it").
				EchoMode(huh.EchoModePassword).
				Value(&apiHash),
		),
	).Run()
	if err != nil {
		return err
	}

	apiID = strings.TrimSpace(apiID)
	apiHash = strings.TrimSpace(apiHash)
	if apiID == "" || apiHash == "" {
		return fmt.Errorf("both api_id and api_hash are required")
	}

	id, err := strconv.Atoi(apiID)
	if err != nil {
		return fmt.Errorf("api_id must be a number, got %q", apiID)
	}

	if err := config.EnsureSourceDir("telegram"); err != nil {
		return fmt.Errorf("create telegram dir: %w", err)
	}
	if err := tgsrc.StoreCredentials(id, apiHash); err != nil {
		return fmt.Errorf("store telegram credentials: %w", err)
	}

	if cfg.Telegram == nil {
		cfg.Telegram = &config.TelegramSourceConfig{}
	}
	cfg.Telegram.APIIDRef = tgsrc.APIIDRef
	cfg.Telegram.APIHashRef = tgsrc.APIHashRef
	if cfg.Telegram.Storage.Driver == "" {
		cfg.Telegram.Storage.Driver = "sqlite"
	}
	if cfg.Telegram.BackfillDays == 0 {
		cfg.Telegram.BackfillDays = 90
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println("\n  Telegram credentials saved to your keychain.")
	fmt.Println("  Next: obk telegram auth login")
	return nil
}

func init() {
	setupCmd.AddCommand(setupTelegramSourceCmd)
}

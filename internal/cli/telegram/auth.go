package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Telegram authentication",
}

var authLoginCmd = &cobra.Command{
	Use:     "login",
	Short:   "Authenticate Telegram by scanning a QR code",
	Example: `  obk telegram auth login`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if tgsrc.HasSession(cfg.TelegramSessionPath()) {
			return fmt.Errorf("already authenticated; run 'obk telegram auth logout' first to re-authenticate")
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client, err := newClient(cfg, db)
		if err != nil {
			return err
		}

		addr, _ := cmd.Flags().GetString("addr")
		if err := tgsrc.ServeQR(cmd.Context(), client, addr, db); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		if err := config.LinkSource("telegram"); err != nil {
			return fmt.Errorf("link source: %w", err)
		}

		fmt.Println("\nSuccessfully authenticated Telegram")
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Disconnect and clear the Telegram session",
	Example: `  obk telegram auth logout
  obk telegram auth logout --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Print("About to disconnect the Telegram session. Continue? (y/N): ")
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

		client, err := newClient(cfg, nil)
		if err != nil {
			return err
		}

		if err := client.Logout(context.Background()); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}

		if err := config.UnlinkSource("telegram"); err != nil {
			return fmt.Errorf("unlink source: %w", err)
		}

		fmt.Println("Logged out of Telegram")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "list",
	Short: "Show Telegram authentication status",
	Example: `  obk telegram auth list
  obk telegram auth list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		jsonOut, _ := cmd.Flags().GetBool("json")

		report := func(authenticated bool, username string) error {
			if jsonOut {
				out := map[string]any{"authenticated": authenticated}
				if username != "" {
					out["username"] = username
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}
			if !authenticated {
				fmt.Println("Not authenticated.")
				return nil
			}
			if username != "" {
				fmt.Printf("Authenticated as @%s\n", username)
			} else {
				fmt.Println("Authenticated.")
			}
			return nil
		}

		if !tgsrc.HasSession(cfg.TelegramSessionPath()) {
			return report(false, "")
		}

		client, err := newClient(cfg, nil)
		if err != nil {
			return err
		}

		ok, self, err := client.IsAuthenticated(cmd.Context())
		if err != nil {
			return fmt.Errorf("check session: %w", err)
		}
		username := ""
		if self != nil {
			username = self.Username
		}
		return report(ok, username)
	},
}

func init() {
	authLoginCmd.Flags().String("addr", "127.0.0.1:8086", "Address for the local QR login page")
	authLogoutCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	authStatusCmd.Flags().Bool("json", false, "Output as JSON")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

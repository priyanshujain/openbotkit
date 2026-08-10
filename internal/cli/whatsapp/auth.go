package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/browser"
	wasrc "github.com/73ai/openbotkit/source/whatsapp"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage WhatsApp authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate WhatsApp by scanning a QR code",
	Example: `  obk whatsapp auth login
  obk whatsapp auth login --account assistant`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.IsRemote() {
			return authLoginRemote(cfg)
		}

		account, _ := cmd.Flags().GetString("account")

		if err := config.EnsureSourceDir("whatsapp"); err != nil {
			return fmt.Errorf("create whatsapp dir: %w", err)
		}
		if account != "default" {
			dir := cfg.WhatsAppAccountDir(account)
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("create account dir: %w", err)
			}
		}

		w := wasrc.New(wasrc.Config{
			SessionDBPath: cfg.WhatsAppAccountSessionDBPath(account),
			DataDSN:       cfg.WhatsAppDataDSN(),
		})

		ctx := context.Background()
		if err := w.Login(ctx); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		if err := config.LinkWhatsAppAccount(account); err != nil {
			return fmt.Errorf("link account: %w", err)
		}

		fmt.Printf("\nSuccessfully authenticated WhatsApp (account: %s)\n", account)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Disconnect and clear WhatsApp session",
	Example: `  obk whatsapp auth logout
  obk whatsapp auth logout --force
  obk whatsapp auth logout --account assistant`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		account, _ := cmd.Flags().GetString("account")
		if !force {
			fmt.Printf("About to disconnect WhatsApp session (account: %s). Continue? (y/N): ", account)
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

		w := wasrc.New(wasrc.Config{
			SessionDBPath: cfg.WhatsAppAccountSessionDBPath(account),
		})

		ctx := context.Background()
		if err := w.Logout(ctx); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}

		if err := config.UnlinkWhatsAppAccount(account); err != nil {
			return fmt.Errorf("unlink account: %w", err)
		}

		fmt.Printf("Logged out of WhatsApp (account: %s)\n", account)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "list",
	Short: "List WhatsApp authentication status",
	Example: `  obk whatsapp auth list
  obk whatsapp auth list --json
  obk whatsapp auth list --account assistant`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		account, _ := cmd.Flags().GetString("account")

		ctx := context.Background()
		client, err := wasrc.NewClient(ctx, cfg.WhatsAppAccountSessionDBPath(account))
		if err != nil {
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"authenticated": false, "account": account})
			}
			fmt.Printf("Not authenticated (account: %s, no session found).\n", account)
			return nil
		}
		defer client.Disconnect()

		if !client.IsAuthenticated() {
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"authenticated": false, "account": account})
			}
			fmt.Printf("Not authenticated (account: %s).\n", account)
			return nil
		}

		user := client.WM().Store.ID.User
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"authenticated": true, "user": user, "account": account})
		}
		fmt.Printf("Authenticated as %s (account: %s)\n", user, account)
		return nil
	},
}

func authLoginRemote(cfg *config.Config) error {
	if cfg.Remote == nil || cfg.Remote.Server == "" {
		return fmt.Errorf("remote server not configured — run 'obk setup' to configure")
	}

	url := strings.TrimRight(cfg.Remote.Server, "/") + "/auth/whatsapp"
	if err := browser.Open(url); err != nil {
		fmt.Printf("Open this URL in your browser to authenticate WhatsApp:\n%s\n", url)
	} else {
		fmt.Printf("Opening this URL in your browser to authenticate WhatsApp:\n%s\n", url)
	}

	client, err := newRemoteClient(cfg)
	if err != nil {
		return err
	}
	fmt.Println("\nWaiting for authentication to complete...")
	if err := client.WaitWhatsAppAuth(); err != nil {
		return err
	}
	fmt.Println("WhatsApp authenticated successfully!")
	return nil
}

func init() {
	authLoginCmd.Flags().String("account", "default", "Account label")
	authLogoutCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	authLogoutCmd.Flags().String("account", "default", "Account label")
	authStatusCmd.Flags().Bool("json", false, "Output as JSON")
	authStatusCmd.Flags().String("account", "default", "Account label")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

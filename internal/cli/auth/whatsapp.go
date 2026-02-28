package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/priyanshujain/openbotkit/config"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/spf13/cobra"
)

var whatsappCmd = &cobra.Command{
	Use:   "whatsapp",
	Short: "Manage WhatsApp authentication",
}

var whatsappLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate WhatsApp by scanning a QR code",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := config.EnsureSourceDir("whatsapp"); err != nil {
			return fmt.Errorf("create whatsapp dir: %w", err)
		}

		w := wasrc.New(wasrc.Config{
			SessionDBPath: cfg.WhatsAppSessionDBPath(),
		})

		ctx := context.Background()
		if err := w.Login(ctx); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		fmt.Println("\nSuccessfully authenticated WhatsApp")
		return nil
	},
}

var whatsappLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Disconnect and clear WhatsApp session",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		w := wasrc.New(wasrc.Config{
			SessionDBPath: cfg.WhatsAppSessionDBPath(),
		})

		ctx := context.Background()
		if err := w.Logout(ctx); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}

		os.Remove(cfg.WhatsAppSessionDBPath())

		fmt.Println("Logged out of WhatsApp")
		return nil
	},
}

var whatsappStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show WhatsApp authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		ctx := context.Background()
		client, err := wasrc.NewClient(ctx, cfg.WhatsAppSessionDBPath())
		if err != nil {
			fmt.Println("Not authenticated (no session found).")
			return nil
		}
		defer client.Disconnect()

		if !client.IsAuthenticated() {
			fmt.Println("Not authenticated.")
			return nil
		}

		fmt.Printf("Authenticated as %s\n", client.WM().Store.ID.User)
		return nil
	},
}

func init() {
	whatsappCmd.AddCommand(whatsappLoginCmd)
	whatsappCmd.AddCommand(whatsappLogoutCmd)
	whatsappCmd.AddCommand(whatsappStatusCmd)
}

package gmail

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/priyanshujain/openbotkit/config"
	gmailsrc "github.com/priyanshujain/openbotkit/source/gmail"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Gmail authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate a Gmail account via OAuth2",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := config.EnsureSourceDir("gmail"); err != nil {
			return fmt.Errorf("create gmail dir: %w", err)
		}

		// Check that credentials file exists.
		if _, err := os.Stat(cfg.Gmail.CredentialsFile); os.IsNotExist(err) {
			return fmt.Errorf("credentials file not found: %s\nPlace your Google OAuth client credentials JSON at that path", cfg.Gmail.CredentialsFile)
		}

		// Prompt for email address.
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter the Gmail address to authenticate: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		if email == "" {
			return fmt.Errorf("email address is required")
		}

		g := gmailsrc.New(gmailsrc.Config{
			CredentialsFile: cfg.Gmail.CredentialsFile,
			TokenDBPath:     cfg.GmailTokenDBPath(),
		})

		ctx := context.Background()
		if err := g.Login(ctx, email); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		fmt.Printf("\nSuccessfully authenticated %s\n", email)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored tokens for a Gmail account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		account, _ := cmd.Flags().GetString("account")

		tokenStore, err := gmailsrc.NewTokenStore(cfg.GmailTokenDBPath())
		if err != nil {
			return fmt.Errorf("open token store: %w", err)
		}
		defer tokenStore.Close()

		if account == "" {
			// List accounts and prompt.
			accounts, err := tokenStore.ListAccounts()
			if err != nil || len(accounts) == 0 {
				fmt.Println("No authenticated accounts.")
				return nil
			}
			fmt.Println("Authenticated accounts:")
			for i, a := range accounts {
				fmt.Printf("  %d. %s\n", i+1, a)
			}
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Enter account to logout (or 'all'): ")
			input, _ := reader.ReadString('\n')
			account = strings.TrimSpace(input)
			if account == "all" {
				for _, a := range accounts {
					tokenStore.DeleteToken(a)
				}
				fmt.Println("Logged out of all accounts.")
				return nil
			}
		}

		if err := tokenStore.DeleteToken(account); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}
		fmt.Printf("Logged out of %s\n", account)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connected accounts and token state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		tokenStore, err := gmailsrc.NewTokenStore(cfg.GmailTokenDBPath())
		if err != nil {
			fmt.Println("No authenticated accounts (token store not found).")
			return nil
		}
		defer tokenStore.Close()

		accounts, err := tokenStore.ListAccounts()
		if err != nil || len(accounts) == 0 {
			fmt.Println("No authenticated accounts.")
			return nil
		}

		fmt.Println("Authenticated Gmail accounts:")
		for _, a := range accounts {
			expiry, err := tokenStore.TokenExpiry(a)
			status := "refresh token only"
			if err == nil && expiry != nil {
				status = fmt.Sprintf("token expires %s", expiry.Format("2006-01-02 15:04:05"))
			}
			fmt.Printf("  %s (%s)\n", a, status)
		}
		return nil
	},
}

func init() {
	authLogoutCmd.Flags().String("account", "", "Account to logout")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

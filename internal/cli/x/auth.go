package x

import (
	"fmt"
	"strings"

	"github.com/73ai/openbotkit/agent/audit"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/browser/cookies"
	"github.com/73ai/openbotkit/internal/tty"
	"github.com/73ai/openbotkit/source/twitter/client"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage X authentication",
	RunE:  authBrowserRun,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to X",
	Example: `  obk x auth login
  obk x auth login --token <browser-cookie-token>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		token = strings.TrimSpace(token)

		if token != "" {
			logXAudit("x.auth.login_token", "", "token-based login", "")
			session := client.NewSession(token)

			fmt.Println("Verifying session...")
			if err := client.ValidateSession(cmd.Context(), session); err != nil {
				logXAudit("x.auth.login_token", "", "", err.Error())
				return fmt.Errorf("token is not valid: %w", err)
			}

			if err := client.SaveSession(session); err != nil {
				logXAudit("x.auth.login_token", "", "", err.Error())
				return fmt.Errorf("save credentials: %w", err)
			}
			if err := config.LinkSource("x"); err != nil {
				return fmt.Errorf("link source: %w", err)
			}
			logXAudit("x.auth.login_token", "", "token login successful", "")
			fmt.Println("Authenticated with X successfully.")
			fmt.Println("Run 'obk x sync' to fetch your timeline.")
			return nil
		}

		return authBrowserRun(cmd, args)
	},
}

func authBrowserRun(cmd *cobra.Command, args []string) error {
	if err := tty.RequireInteractive("obk x auth login --token <token>"); err != nil {
		return err
	}

	browser, err := selectBrowser()
	if err != nil {
		return err
	}

	if browser == "Safari" {
		printSafariFDANote()
	}

	fmt.Printf("Extracting X session from %s...\n", browser)
	logXAudit("x.auth.login_browser", "browser="+browser, "extracting cookies", "")

	session, _, err := client.ExtractSessionFromBrowserByName(browser)
	if err != nil {
		logXAudit("x.auth.login_browser", "browser="+browser, "", err.Error())
		if browser == "Safari" && cookies.IsPermissionError(err) {
			fmt.Println("\nSafari cookie access was denied.")
			printSafariFDAInstructions()
			return fmt.Errorf("Safari requires Full Disk Access")
		}
		fmt.Printf("Failed to extract X session from %s: %v\n", browser, err)
		fmt.Println("Make sure you're signed in to x.com in that browser.")
		return fmt.Errorf("cookie extraction failed: %w", err)
	}

	fmt.Println("Verifying session...")
	if err := client.ValidateSession(cmd.Context(), session); err != nil {
		logXAudit("x.auth.login_browser", "browser="+browser, "", err.Error())
		fmt.Printf("Session from %s is not valid: %v\n", browser, err)
		fmt.Println("Make sure you're signed in to x.com in that browser.")
		return fmt.Errorf("session validation failed: %w", err)
	}

	if err := client.SaveSession(session); err != nil {
		logXAudit("x.auth.login_browser", "browser="+browser, "", err.Error())
		return fmt.Errorf("save credentials: %w", err)
	}

	if err := config.LinkSource("x"); err != nil {
		return fmt.Errorf("link source: %w", err)
	}

	logXAudit("x.auth.login_browser", "browser="+browser, "login successful", "")
	fmt.Printf("Authenticated with X (from %s).\n", browser)
	fmt.Println("Run 'obk x sync' to fetch your timeline.")
	return nil
}

func selectBrowser() (string, error) {
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
		return "", err
	}
	return browser, nil
}

func printSafariFDANote() {
	fmt.Println()
	fmt.Println("  Note: Safari cookies are protected by macOS.")
	fmt.Println("  Your terminal app needs Full Disk Access to read them.")
	fmt.Println()
}

func printSafariFDAInstructions() {
	fmt.Println()
	fmt.Println("  To grant Full Disk Access:")
	fmt.Println("  1. Open System Settings > Privacy & Security > Full Disk Access")
	fmt.Println("  2. Click '+' and add your terminal app")
	fmt.Println("  3. Restart your terminal and try again")
	fmt.Println()
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of X",
	RunE: func(cmd *cobra.Command, args []string) error {
		logXAudit("x.auth.logout", "", "signing out", "")
		if err := client.DeleteSession(); err != nil {
			logXAudit("x.auth.logout", "", "", err.Error())
			return fmt.Errorf("delete credentials: %w", err)
		}
		logXAudit("x.auth.logout", "", "signed out", "")
		fmt.Println("Signed out of X.")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check X authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := client.LoadSession()
		if err != nil {
			fmt.Println("Not signed in. Run 'obk x auth login' to connect.")
			return nil
		}
		if err := client.ValidateSession(cmd.Context(), session); err != nil {
			fmt.Printf("Session expired or invalid: %v\n", err)
			fmt.Println("Run 'obk x auth login' to re-authenticate.")
			return nil
		}
		fmt.Println("Signed in to X.")
		if session.Username != "" {
			fmt.Printf("Account: @%s\n", session.Username)
		}
		return nil
	},
}

func logXAudit(toolName, input, output, errMsg string) {
	audit.LogQuick(config.AuditJSONLPath(), "cli", toolName, input, output, errMsg)
}

func init() {
	authLoginCmd.Flags().String("token", "", "auth_token from browser cookies (advanced)")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

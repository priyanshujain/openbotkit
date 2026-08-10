package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/charmbracelet/huh"
)

var errSetupSkipped = errors.New("skipped by user")

const serverPort = "8443"

func setupNgrok(cfg *config.Config) error {
	fmt.Println("\n  -- Public Tunnel Setup (ngrok) --")
	fmt.Println("  Telegram users authenticate on their phone, so Google's OAuth")
	fmt.Println("  callback needs a public URL. ngrok provides this for free.")

	ngrokPath, err := ensureNgrok()
	if errors.Is(err, errSetupSkipped) {
		fmt.Println("  Skipping ngrok setup. You can configure it later with: obk setup")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("  ngrok found at %s\n", ngrokPath)

	fmt.Println("\n  If you don't have an ngrok account, create one (free):")
	fmt.Println("    https://dashboard.ngrok.com/signup")

	var authtoken string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Paste your ngrok authtoken").
				Description("Find it at https://dashboard.ngrok.com/get-started/your-authtoken").
				Value(&authtoken),
		),
	).Run()
	if err != nil {
		return err
	}
	authtoken = strings.TrimSpace(authtoken)
	if authtoken == "" {
		return fmt.Errorf("ngrok authtoken is required")
	}

	out, err := exec.Command(ngrokPath, "config", "add-authtoken", authtoken).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ngrok add-authtoken: %s: %w", out, err)
	}

	fmt.Println("\n  Detecting your ngrok domain...")
	domain, err := detectNgrokDomain(ngrokPath)
	if err != nil {
		fmt.Printf("  Auto-detect failed: %v\n", err)
		fmt.Println("  Falling back to manual entry.")
		domain, err = promptNgrokDomain()
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("  Detected domain: %s\n", domain)
	}

	ngrokCfgPath := filepath.Join(config.Dir(), "ngrok.yml")
	if err := writeNgrokConfig(ngrokCfgPath, authtoken, domain); err != nil {
		return fmt.Errorf("write ngrok config: %w", err)
	}
	fmt.Printf("  ngrok config written to %s\n", ngrokCfgPath)

	fmt.Println("\n  Installing ngrok as a system service...")
	if out, err := exec.Command(ngrokPath, "service", "install", "--config", ngrokCfgPath).CombinedOutput(); err != nil {
		fmt.Printf("  ngrok service install: %s\n", strings.TrimSpace(string(out)))
		fmt.Println("  (You may need to start ngrok manually: ngrok start --config " + ngrokCfgPath + " obk)")
	} else {
		if out, err := exec.Command(ngrokPath, "service", "start").CombinedOutput(); err != nil {
			fmt.Printf("  ngrok service start: %s\n", strings.TrimSpace(string(out)))
		} else {
			fmt.Println("  ngrok service started")
		}
	}

	callbackURL := buildCallbackURL(domain)
	fmt.Printf("\n  Your callback URL: %s\n", callbackURL)

	printGoogleConsoleGuide(callbackURL)

	if cfg.Integrations == nil {
		cfg.Integrations = &config.IntegrationsConfig{}
	}
	if cfg.Integrations.GWS == nil {
		cfg.Integrations.GWS = &config.GWSConfig{}
	}
	cfg.Integrations.GWS.CallbackURL = callbackURL
	cfg.Integrations.GWS.NgrokDomain = domain

	return nil
}

// detectNgrokDomain starts a temporary ngrok tunnel to discover the account's
// auto-assigned dev domain, then shuts the tunnel down.
func detectNgrokDomain(ngrokPath string) (string, error) {
	// Start a temporary tunnel on an unused port (we don't need it to connect).
	cmd := exec.Command(ngrokPath, "http", "19999", "--log", "stderr")
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start ngrok: %w", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Poll the local agent API until the tunnel is up.
	var domain string
	for range 20 {
		time.Sleep(500 * time.Millisecond)
		d, err := queryNgrokTunnelDomain()
		if err == nil && d != "" {
			domain = d
			break
		}
	}
	if domain == "" {
		return "", fmt.Errorf("tunnel did not start within 10s")
	}
	return domain, nil
}

// queryNgrokTunnelDomain queries the ngrok local agent API for the tunnel's public URL.
func queryNgrokTunnelDomain() (string, error) {
	resp, err := http.Get("http://127.0.0.1:4040/api/tunnels")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Tunnels []struct {
			PublicURL string `json:"public_url"`
		} `json:"tunnels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	for _, t := range result.Tunnels {
		if strings.HasPrefix(t.PublicURL, "https://") {
			u, err := url.Parse(t.PublicURL)
			if err != nil {
				continue
			}
			return u.Host, nil
		}
	}
	return "", fmt.Errorf("no https tunnel found")
}

func promptNgrokDomain() (string, error) {
	var domain string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter your ngrok static domain").
				Description("Dashboard > Universal Gateway > Domains (e.g. panda-new-kit.ngrok-free.app)").
				Value(&domain),
		),
	).Run()
	if err != nil {
		return "", err
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("ngrok domain is required")
	}
	return domain, nil
}

func ensureNgrok() (string, error) {
	p, err := exec.LookPath("ngrok")
	if err == nil {
		return p, nil
	}

	fmt.Println("\n  ngrok not found.")
	if runtime.GOOS == "darwin" {
		fmt.Println("    brew install ngrok/ngrok/ngrok")
	} else {
		fmt.Println("    See https://ngrok.com/download for install instructions")
	}

	var choice string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("ngrok is not installed").
			Options(
				huh.NewOption("Wait for install (run the command above in another tab)", "wait"),
				huh.NewOption("Skip ngrok setup for now", "skip"),
			).
			Value(&choice),
	)).Run(); err != nil {
		return "", err
	}
	if choice == "skip" {
		return "", errSetupSkipped
	}

	const maxAttempts = 60
	for attempt := range maxAttempts {
		time.Sleep(5 * time.Second)
		p, err = exec.LookPath("ngrok")
		if err == nil {
			return p, nil
		}
		fmt.Println("  Checking... not found")
		if attempt == maxAttempts-1 {
			return "", fmt.Errorf("ngrok not found after %d attempts — install it and re-run obk setup", maxAttempts)
		}
	}
	return "", fmt.Errorf("ngrok not found")
}

func writeNgrokConfig(path, authtoken, domain string) error {
	content := fmt.Sprintf(`version: "3"
agent:
  authtoken: %s
tunnels:
  obk:
    proto: http
    addr: %s
    domain: %s
`, authtoken, serverPort, domain)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

func buildCallbackURL(domain string) string {
	return "https://" + domain + "/auth/google/callback"
}

func printGoogleConsoleGuide(callbackURL string) {
	fmt.Println("\n  -- Google Cloud Console Setup --")
	fmt.Println("  You need a Web Application OAuth client (not Desktop).")
	fmt.Println("  Steps:")
	fmt.Println("    1. Go to console.cloud.google.com > APIs & Services > Credentials")
	fmt.Println("    2. Create Credentials > OAuth client ID > Web application")
	fmt.Println("    3. Under 'Authorized redirect URIs', add BOTH:")
	fmt.Printf("       - %s\n", callbackURL)
	fmt.Println("       - http://localhost:8085/callback")
	fmt.Println("    4. Download the JSON credentials file")
	fmt.Println()
}

package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// implicitScopes are always requested so we can extract the user's email.
var implicitScopes = []string{"openid", "email"}

func loadConfig(credFile string, scopes []string) (*oauth2.Config, error) {
	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	allScopes := mergeScopes(scopes, implicitScopes)
	cfg, err := google.ConfigFromJSON(b, allScopes...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	cfg.RedirectURL = "http://localhost:8085/callback"
	return cfg, nil
}

func getTokenViaCallback(config *oauth2.Config, authOpts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback received without code parameter")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<h1>Authentication successful!</h1><p>You can close this tab.</p>")
		codeCh <- code
	})

	server := &http.Server{
		Addr:    ":8085",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()

	opts := append([]oauth2.AuthCodeOption{oauth2.AccessTypeOffline}, authOpts...)
	authURL := config.AuthCodeURL("state-token", opts...)
	fmt.Printf("\nOpen this URL in your browser to authorize:\n%s\n\n", authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Close()
		return nil, err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}
	return tok, nil
}

// fetchUserEmail calls the Google userinfo endpoint to get the authenticated email.
func fetchUserEmail(ctx context.Context, httpClient *http.Client) (string, error) {
	resp, err := httpClient.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, body)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decode userinfo: %w", err)
	}
	if info.Email == "" {
		return "", fmt.Errorf("userinfo response missing email")
	}
	return info.Email, nil
}

// mergeScopes combines two scope lists, deduplicating.
func mergeScopes(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

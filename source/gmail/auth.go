package gmail

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// authenticate creates an authenticated Gmail service for a single account.
func authenticate(ctx context.Context, credFile string, email string, tokenStore *TokenStore) (*gapi.Service, error) {
	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(b, gapi.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	tok, err := tokenStore.LoadFullToken(email)
	if err != nil {
		// No stored token — run the OAuth callback flow.
		oauthCfg.RedirectURL = "http://localhost:8085/callback"
		tok, err = getTokenViaCallback(oauthCfg)
		if err != nil {
			return nil, fmt.Errorf("oauth callback flow: %w", err)
		}

		if err := tokenStore.SaveRefreshToken(email, tok.RefreshToken); err != nil {
			return nil, fmt.Errorf("save refresh token: %w", err)
		}
		if err := tokenStore.SaveAccessToken(email, tok); err != nil {
			return nil, fmt.Errorf("save access token: %w", err)
		}
	}

	baseSource := oauthCfg.TokenSource(ctx, tok)
	persistSource := newDBTokenSource(email, tokenStore, baseSource, tok)
	httpClient := oauth2.NewClient(ctx, persistSource)

	srv, err := gapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create gmail service for %s: %w", email, err)
	}
	return srv, nil
}

// Login performs the OAuth2 login flow for a single email account.
// This is the public API for CLI and library use.
func Login(ctx context.Context, credFile string, email string, tokenStore *TokenStore) error {
	b, err := os.ReadFile(credFile)
	if err != nil {
		return fmt.Errorf("read credentials file: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(b, gapi.GmailReadonlyScope)
	if err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}

	oauthCfg.RedirectURL = "http://localhost:8085/callback"
	tok, err := getTokenViaCallback(oauthCfg)
	if err != nil {
		return fmt.Errorf("oauth callback flow: %w", err)
	}

	if err := tokenStore.SaveRefreshToken(email, tok.RefreshToken); err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	if err := tokenStore.SaveAccessToken(email, tok); err != nil {
		return fmt.Errorf("save access token: %w", err)
	}
	return nil
}

func getTokenViaCallback(config *oauth2.Config) (*oauth2.Token, error) {
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

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
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

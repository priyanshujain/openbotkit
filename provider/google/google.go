package google

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

type Config struct {
	CredentialsFile string
	TokenDBPath     string
}

type Google struct {
	cfg Config
}

func New(cfg Config) *Google {
	return &Google{cfg: cfg}
}

func (g *Google) Name() string {
	return "google"
}

func (g *Google) Client(ctx context.Context, account string, scopes []string) (*http.Client, error) {
	store, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}
	defer store.Close()

	has, err := store.HasScopes(account, scopes)
	if err != nil {
		return nil, fmt.Errorf("check scopes: %w", err)
	}
	if !has {
		return nil, fmt.Errorf("account %q missing required scopes; run 'obk auth google login' to grant", account)
	}

	tok, _, err := store.LoadToken(account)
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}

	oauthCfg, err := loadConfig(g.cfg.CredentialsFile, scopes)
	if err != nil {
		return nil, err
	}

	baseSource := oauthCfg.TokenSource(ctx, tok)
	persistSource := newDBTokenSource(account, store, baseSource, tok)
	return oauth2.NewClient(ctx, persistSource), nil
}

func (g *Google) GrantScopes(ctx context.Context, account string, scopes []string) (string, error) {
	oauthCfg, err := loadConfig(g.cfg.CredentialsFile, scopes)
	if err != nil {
		return "", err
	}

	var authOpts []oauth2.AuthCodeOption
	if account != "" {
		// Incremental auth: keep existing scopes, add new ones.
		authOpts = append(authOpts,
			oauth2.SetAuthURLParam("login_hint", account),
			oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		)
	}

	tok, err := getTokenViaCallback(oauthCfg, authOpts...)
	if err != nil {
		return "", fmt.Errorf("oauth flow: %w", err)
	}

	// Get the actual email from the OAuth response.
	tmpClient := oauthCfg.Client(ctx, tok)
	email, err := fetchUserEmail(ctx, tmpClient)
	if err != nil {
		return "", fmt.Errorf("fetch user email: %w", err)
	}

	// Merge with existing scopes if this is an incremental grant.
	store, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return "", fmt.Errorf("open token store: %w", err)
	}
	defer store.Close()

	allScopes := scopes
	if account != "" {
		_, existing, err := store.LoadToken(account)
		if err == nil {
			allScopes = mergeScopes(existing, scopes)
		}
	}

	if err := store.SaveToken(email, tok, allScopes); err != nil {
		return "", fmt.Errorf("save token: %w", err)
	}

	return email, nil
}

func (g *Google) GrantedScopes(ctx context.Context, account string) ([]string, error) {
	store, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}
	defer store.Close()

	_, scopes, err := store.LoadToken(account)
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	return scopes, nil
}

func (g *Google) RevokeScopes(ctx context.Context, account string, scopes []string) error {
	store, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return fmt.Errorf("open token store: %w", err)
	}
	defer store.Close()

	_, existing, err := store.LoadToken(account)
	if err != nil {
		return fmt.Errorf("load token: %w", err)
	}

	// Compute remaining scopes.
	remove := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		remove[s] = true
	}
	var remaining []string
	for _, s := range existing {
		if !remove[s] {
			remaining = append(remaining, s)
		}
	}

	if len(remaining) == 0 {
		// Revoking all scopes — just delete the token.
		return store.DeleteToken(account)
	}

	// Re-auth with only the remaining scopes (no include_granted_scopes).
	oauthCfg, err := loadConfig(g.cfg.CredentialsFile, remaining)
	if err != nil {
		return err
	}

	tok, err := getTokenViaCallback(oauthCfg,
		oauth2.SetAuthURLParam("login_hint", account),
	)
	if err != nil {
		return fmt.Errorf("oauth flow: %w", err)
	}

	return store.SaveToken(account, tok, remaining)
}

func (g *Google) Accounts(ctx context.Context) ([]string, error) {
	store, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}
	defer store.Close()

	return store.ListAccounts()
}

package gmail

import (
	"context"
	"fmt"

	gapi "google.golang.org/api/gmail/v1"
)

// account represents an authenticated Gmail account.
type account struct {
	Email   string
	Service *gapi.Service
}

// client holds authenticated connections to one or more Gmail accounts.
type client struct {
	accounts   []*account
	tokenStore *TokenStore
}

// newClient authenticates each email account and returns a client.
func newClient(ctx context.Context, cfg Config, emails []string) (*client, error) {
	tokenStore, err := NewTokenStore(cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}

	c := &client{tokenStore: tokenStore}

	for _, email := range emails {
		srv, err := authenticate(ctx, cfg.CredentialsFile, email, tokenStore)
		if err != nil {
			tokenStore.Close()
			return nil, fmt.Errorf("authenticate %s: %w", email, err)
		}
		c.accounts = append(c.accounts, &account{
			Email:   email,
			Service: srv,
		})
	}

	return c, nil
}

// close releases resources held by the client.
func (c *client) close() error {
	if c.tokenStore != nil {
		return c.tokenStore.Close()
	}
	return nil
}

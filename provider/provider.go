package provider

import (
	"context"
	"net/http"
)

// Provider manages authentication credentials and tokens for a cloud service.
// Sources (Gmail, Calendar, etc.) declare what scopes they need and get
// an authenticated HTTP client from their provider.
type Provider interface {
	Name() string
	Client(ctx context.Context, account string, scopes []string) (*http.Client, error)
	GrantScopes(ctx context.Context, account string, scopes []string) (grantedAccount string, err error)
	GrantedScopes(ctx context.Context, account string) ([]string, error)
	RevokeScopes(ctx context.Context, account string, scopes []string) error
	Accounts(ctx context.Context) ([]string, error)
}

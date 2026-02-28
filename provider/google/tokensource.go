package google

import (
	"fmt"
	"sync"

	"golang.org/x/oauth2"
)

// dbTokenSource wraps an oauth2.TokenSource and persists refreshed tokens
// back to the database on every refresh.
type dbTokenSource struct {
	email   string
	store   *TokenStore
	base    oauth2.TokenSource
	mu      sync.Mutex
	current *oauth2.Token
}

func newDBTokenSource(email string, store *TokenStore, base oauth2.TokenSource, initial *oauth2.Token) oauth2.TokenSource {
	return &dbTokenSource{
		email:   email,
		store:   store,
		base:    base,
		current: initial,
	}
}

func (s *dbTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current.Valid() {
		return s.current, nil
	}

	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}

	if err := s.store.SaveAccessToken(s.email, tok); err != nil {
		return nil, fmt.Errorf("save refreshed access token: %w", err)
	}
	if tok.RefreshToken != "" && tok.RefreshToken != s.current.RefreshToken {
		if err := s.store.SaveRefreshToken(s.email, tok.RefreshToken); err != nil {
			return nil, fmt.Errorf("save rotated refresh token: %w", err)
		}
	}

	s.current = tok
	return tok, nil
}

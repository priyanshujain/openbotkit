package google

import (
	"fmt"
	"sync"

	"golang.org/x/oauth2"
)

type dbTokenSource struct {
	email   string
	dbPath  string
	base    oauth2.TokenSource
	mu      sync.Mutex
	current *oauth2.Token
}

func newDBTokenSource(email, dbPath string, base oauth2.TokenSource, initial *oauth2.Token) oauth2.TokenSource {
	return &dbTokenSource{
		email:   email,
		dbPath:  dbPath,
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

	if err := s.persistToken(tok); err != nil {
		return nil, err
	}

	s.current = tok
	return tok, nil
}

func (s *dbTokenSource) persistToken(tok *oauth2.Token) error {
	store, err := NewTokenStore(s.dbPath)
	if err != nil {
		return fmt.Errorf("open token store for refresh: %w", err)
	}
	defer store.Close()

	if err := store.SaveAccessToken(s.email, tok); err != nil {
		return fmt.Errorf("save refreshed access token: %w", err)
	}
	if tok.RefreshToken != "" && tok.RefreshToken != s.current.RefreshToken {
		if err := store.SaveRefreshToken(s.email, tok.RefreshToken); err != nil {
			return fmt.Errorf("save rotated refresh token: %w", err)
		}
	}
	return nil
}

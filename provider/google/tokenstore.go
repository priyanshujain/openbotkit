package google

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
)

// TokenStore persists Google OAuth tokens in a local SQLite database.
// Uses a single unified table instead of the legacy 2-table design.
type TokenStore struct {
	db *sql.DB
}

func NewTokenStore(dbPath string) (*TokenStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open token db: %w", err)
	}
	ts := &TokenStore{db: db}
	if err := ts.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate token db: %w", err)
	}
	return ts, nil
}

func (ts *TokenStore) migrate() error {
	_, err := ts.db.Exec(`
		CREATE TABLE IF NOT EXISTS oauth_tokens (
			email TEXT PRIMARY KEY,
			refresh_token TEXT NOT NULL,
			access_token TEXT NOT NULL DEFAULT '',
			token_type TEXT NOT NULL DEFAULT 'Bearer',
			expiry DATETIME,
			granted_scopes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (ts *TokenStore) SaveToken(email string, tok *oauth2.Token, scopes []string) error {
	scopeStr := strings.Join(scopes, " ")
	_, err := ts.db.Exec(`
		INSERT INTO oauth_tokens (email, refresh_token, access_token, token_type, expiry, granted_scopes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(email) DO UPDATE SET
			refresh_token = excluded.refresh_token,
			access_token = excluded.access_token,
			token_type = excluded.token_type,
			expiry = excluded.expiry,
			granted_scopes = excluded.granted_scopes,
			updated_at = CURRENT_TIMESTAMP
	`, email, tok.RefreshToken, tok.AccessToken, tok.TokenType, tok.Expiry.UTC(), scopeStr)
	return err
}

func (ts *TokenStore) LoadToken(email string) (*oauth2.Token, []string, error) {
	var refreshToken, accessToken, tokenType, scopeStr string
	var expiry sql.NullTime
	err := ts.db.QueryRow(`
		SELECT refresh_token, access_token, token_type, expiry, granted_scopes
		FROM oauth_tokens WHERE email = ?
	`, email).Scan(&refreshToken, &accessToken, &tokenType, &expiry, &scopeStr)
	if err != nil {
		return nil, nil, err
	}

	tok := &oauth2.Token{
		AccessToken:  accessToken,
		TokenType:    tokenType,
		RefreshToken: refreshToken,
	}
	if expiry.Valid {
		tok.Expiry = expiry.Time
	}

	var scopes []string
	if scopeStr != "" {
		scopes = strings.Split(scopeStr, " ")
	}
	return tok, scopes, nil
}

func (ts *TokenStore) UpdateScopes(email string, scopes []string) error {
	scopeStr := strings.Join(scopes, " ")
	_, err := ts.db.Exec(`
		UPDATE oauth_tokens SET granted_scopes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE email = ?
	`, scopeStr, email)
	return err
}

func (ts *TokenStore) SaveAccessToken(email string, tok *oauth2.Token) error {
	_, err := ts.db.Exec(`
		UPDATE oauth_tokens SET
			access_token = ?,
			token_type = ?,
			expiry = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE email = ?
	`, tok.AccessToken, tok.TokenType, tok.Expiry.UTC(), email)
	return err
}

func (ts *TokenStore) SaveRefreshToken(email string, refreshToken string) error {
	_, err := ts.db.Exec(`
		UPDATE oauth_tokens SET refresh_token = ?, updated_at = CURRENT_TIMESTAMP
		WHERE email = ?
	`, refreshToken, email)
	return err
}

func (ts *TokenStore) ListAccounts() ([]string, error) {
	rows, err := ts.db.Query(`SELECT email FROM oauth_tokens ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		accounts = append(accounts, email)
	}
	return accounts, rows.Err()
}

func (ts *TokenStore) DeleteToken(email string) error {
	_, err := ts.db.Exec(`DELETE FROM oauth_tokens WHERE email = ?`, email)
	return err
}

func (ts *TokenStore) HasScopes(email string, required []string) (bool, error) {
	var scopeStr string
	err := ts.db.QueryRow(`SELECT granted_scopes FROM oauth_tokens WHERE email = ?`, email).Scan(&scopeStr)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	granted := make(map[string]bool)
	for _, s := range strings.Split(scopeStr, " ") {
		granted[s] = true
	}
	for _, r := range required {
		if !granted[r] {
			return false, nil
		}
	}
	return true, nil
}

func (ts *TokenStore) TokenExpiry(email string) (*time.Time, error) {
	var expiry sql.NullTime
	err := ts.db.QueryRow(`SELECT expiry FROM oauth_tokens WHERE email = ?`, email).Scan(&expiry)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !expiry.Valid {
		return nil, nil
	}
	t := expiry.Time
	return &t, nil
}

func (ts *TokenStore) Close() error {
	return ts.db.Close()
}

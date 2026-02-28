package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/priyanshujain/openbotkit/config"
)

// migrateTokens reads the old 2-table token store and writes to the new
// unified oauth_tokens table. Sets granted_scopes to gmail.readonly
// for all migrated tokens (the only scope the old system used).
func migrateTokens() error {
	oldPath := filepath.Join(config.SourceDir("gmail"), "tokens.db")
	newPath := filepath.Join(config.ProviderDir("google"), "tokens.db")

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil
	}

	if _, err := os.Stat(newPath); err == nil {
		return nil // already migrated
	}

	oldDB, err := sql.Open("sqlite3", oldPath)
	if err != nil {
		return fmt.Errorf("open old token db: %w", err)
	}
	defer oldDB.Close()

	newDB, err := sql.Open("sqlite3", newPath)
	if err != nil {
		return fmt.Errorf("open new token db: %w", err)
	}
	defer newDB.Close()

	// Create the new schema.
	if _, err := newDB.Exec(`
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
	`); err != nil {
		return fmt.Errorf("create new schema: %w", err)
	}

	// Read from old tables.
	rows, err := oldDB.Query(`SELECT email, refresh_token FROM refresh_tokens`)
	if err != nil {
		return fmt.Errorf("query old refresh tokens: %w", err)
	}
	defer rows.Close()

	const defaultScope = "https://www.googleapis.com/auth/gmail.readonly"

	for rows.Next() {
		var email, refreshToken string
		if err := rows.Scan(&email, &refreshToken); err != nil {
			return fmt.Errorf("scan refresh token: %w", err)
		}

		// Try to get the access token.
		var accessToken, tokenType string
		var expiry sql.NullTime
		err := oldDB.QueryRow(`SELECT access_token, token_type, expiry FROM access_tokens WHERE email = ?`, email).
			Scan(&accessToken, &tokenType, &expiry)
		if err == sql.ErrNoRows {
			tokenType = "Bearer"
		} else if err != nil {
			return fmt.Errorf("query access token for %s: %w", email, err)
		}

		expiryVal := sql.NullTime{}
		if expiry.Valid {
			expiryVal = expiry
		}

		if _, err := newDB.Exec(`
			INSERT INTO oauth_tokens (email, refresh_token, access_token, token_type, expiry, granted_scopes)
			VALUES (?, ?, ?, ?, ?, ?)
		`, email, refreshToken, accessToken, tokenType, expiryVal, defaultScope); err != nil {
			return fmt.Errorf("insert token for %s: %w", email, err)
		}

		fmt.Printf("  Migrated token for %s\n", email)
	}

	return rows.Err()
}

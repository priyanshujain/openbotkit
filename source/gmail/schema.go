package gmail

import "github.com/priyanshujain/openbotkit/store"

// schemaSQLite is the DDL for Gmail tables in SQLite.
const schemaSQLite = `
CREATE TABLE IF NOT EXISTS gmail_emails (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id TEXT NOT NULL,
	account TEXT NOT NULL,
	from_addr TEXT,
	to_addr TEXT,
	subject TEXT,
	date DATETIME,
	body TEXT,
	html_body TEXT,
	fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(message_id, account)
);

CREATE TABLE IF NOT EXISTS gmail_attachments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email_id INTEGER REFERENCES gmail_emails(id),
	filename TEXT,
	mime_type TEXT,
	saved_path TEXT
);

CREATE INDEX IF NOT EXISTS idx_gmail_emails_account ON gmail_emails(account);
CREATE INDEX IF NOT EXISTS idx_gmail_emails_date ON gmail_emails(date);
CREATE INDEX IF NOT EXISTS idx_gmail_emails_from ON gmail_emails(from_addr);
`

// schemaPostgres is the DDL for Gmail tables in Postgres.
const schemaPostgres = `
CREATE TABLE IF NOT EXISTS gmail_emails (
	id BIGSERIAL PRIMARY KEY,
	message_id TEXT NOT NULL,
	account TEXT NOT NULL,
	from_addr TEXT,
	to_addr TEXT,
	subject TEXT,
	date TIMESTAMPTZ,
	body TEXT,
	html_body TEXT,
	fetched_at TIMESTAMPTZ DEFAULT NOW(),
	UNIQUE(message_id, account)
);

CREATE TABLE IF NOT EXISTS gmail_attachments (
	id BIGSERIAL PRIMARY KEY,
	email_id BIGINT REFERENCES gmail_emails(id),
	filename TEXT,
	mime_type TEXT,
	saved_path TEXT
);

CREATE INDEX IF NOT EXISTS idx_gmail_emails_account ON gmail_emails(account);
CREATE INDEX IF NOT EXISTS idx_gmail_emails_date ON gmail_emails(date);
CREATE INDEX IF NOT EXISTS idx_gmail_emails_from ON gmail_emails(from_addr);
`

// Migrate creates the Gmail schema in the database.
func Migrate(db *store.DB) error {
	schema := schemaSQLite
	if db.IsPostgres() {
		schema = schemaPostgres
	}
	_, err := db.Exec(schema)
	return err
}

package gmail

import "time"

// Email holds a fully parsed Gmail message.
type Email struct {
	MessageID   string
	Account     string
	From        string
	To          string
	Subject     string
	Date        time.Time
	Body        string // plain text
	HTMLBody    string // HTML
	Attachments []Attachment
}

// Attachment holds attachment metadata and binary data.
type Attachment struct {
	Filename  string
	MimeType  string
	Data      []byte
	SavedPath string // populated after saving to disk
}

// Config holds the configuration for a Gmail source.
type Config struct {
	CredentialsFile string
	TokenDBPath     string
}

// SyncOptions controls the behavior of a sync operation.
type SyncOptions struct {
	Full                bool   // re-fetch everything, ignoring existing
	After               string // only emails after this date (YYYY/MM/DD)
	Account             string // filter to a single account
	DownloadAttachments bool   // save attachments to disk
	AttachmentsDir      string // base directory for attachments
}

// SyncResult summarizes the outcome of a sync operation.
type SyncResult struct {
	Fetched int
	Skipped int
	Errors  int
}

// ListOptions controls email listing queries.
type ListOptions struct {
	Account string
	From    string
	Subject string
	After   string // YYYY-MM-DD
	Before  string // YYYY-MM-DD
	Limit   int
	Offset  int
}

// FetchQuery defines a Gmail search filter.
type FetchQuery struct {
	From  string // e.g. "anthropic.com"
	After string // date string "2025/07/17"
	Query string // raw Gmail query (takes precedence)
}

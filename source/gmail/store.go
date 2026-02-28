package gmail

import (
	"fmt"
	"strings"
	"time"

	"github.com/priyanshujain/openbotkit/store"
)

// EmailExists checks if an email with the given message ID and account exists.
func EmailExists(db *store.DB, messageID, account string) (bool, error) {
	var count int
	err := db.QueryRow(
		db.Rebind("SELECT COUNT(*) FROM gmail_emails WHERE message_id = ? AND account = ?"),
		messageID, account,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return count > 0, nil
}

// SaveEmail inserts an email and its attachments into the database.
func SaveEmail(db *store.DB, email *Email) (int64, error) {
	res, err := db.Exec(
		db.Rebind(`INSERT INTO gmail_emails (message_id, account, from_addr, to_addr, subject, date, body, html_body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
		email.MessageID, email.Account, email.From, email.To,
		email.Subject, email.Date, email.Body, email.HTMLBody,
	)
	if err != nil {
		// Check if it's a uniqueness violation (already exists).
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate key") {
			var id int64
			err2 := db.QueryRow(
				db.Rebind("SELECT id FROM gmail_emails WHERE message_id = ? AND account = ?"),
				email.MessageID, email.Account,
			).Scan(&id)
			if err2 != nil {
				return 0, fmt.Errorf("lookup existing email: %w", err2)
			}
			return id, nil
		}
		return 0, fmt.Errorf("insert email: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get email id: %w", err)
	}

	for _, att := range email.Attachments {
		_, err := db.Exec(
			db.Rebind(`INSERT INTO gmail_attachments (email_id, filename, mime_type, saved_path) VALUES (?, ?, ?, ?)`),
			id, att.Filename, att.MimeType, att.SavedPath,
		)
		if err != nil {
			return id, fmt.Errorf("insert attachment: %w", err)
		}
	}

	return id, nil
}

// ListEmails queries stored emails with optional filters.
func ListEmails(db *store.DB, opts ListOptions) ([]Email, error) {
	var conditions []string
	var args []interface{}

	if opts.Account != "" {
		conditions = append(conditions, "account = ?")
		args = append(args, opts.Account)
	}
	if opts.From != "" {
		conditions = append(conditions, "LOWER(from_addr) LIKE ?")
		args = append(args, "%"+strings.ToLower(opts.From)+"%")
	}
	if opts.Subject != "" {
		conditions = append(conditions, "LOWER(subject) LIKE ?")
		args = append(args, "%"+strings.ToLower(opts.Subject)+"%")
	}
	if opts.After != "" {
		conditions = append(conditions, "date >= ?")
		args = append(args, opts.After)
	}
	if opts.Before != "" {
		conditions = append(conditions, "date <= ?")
		args = append(args, opts.Before)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(
		`SELECT id, message_id, account, from_addr, to_addr, subject, date, body, html_body
		 FROM gmail_emails %s ORDER BY date DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, opts.Offset)

	rows, err := db.Query(db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("list emails: %w", err)
	}
	defer rows.Close()

	var emails []Email
	for rows.Next() {
		var e Email
		var id int64
		err := rows.Scan(&id, &e.MessageID, &e.Account, &e.From, &e.To, &e.Subject, &e.Date, &e.Body, &e.HTMLBody)
		if err != nil {
			return nil, fmt.Errorf("scan email: %w", err)
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// GetEmail retrieves a single email by message ID.
func GetEmail(db *store.DB, messageID string) (*Email, error) {
	var e Email
	var id int64
	err := db.QueryRow(
		db.Rebind(`SELECT id, message_id, account, from_addr, to_addr, subject, date, body, html_body
		 FROM gmail_emails WHERE message_id = ?`),
		messageID,
	).Scan(&id, &e.MessageID, &e.Account, &e.From, &e.To, &e.Subject, &e.Date, &e.Body, &e.HTMLBody)
	if err != nil {
		return nil, fmt.Errorf("get email: %w", err)
	}

	// Load attachments.
	rows, err := db.Query(
		db.Rebind(`SELECT filename, mime_type, saved_path FROM gmail_attachments WHERE email_id = ?`),
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("get attachments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.Filename, &a.MimeType, &a.SavedPath); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		e.Attachments = append(e.Attachments, a)
	}

	return &e, rows.Err()
}

// SearchEmails performs a text search across subject and body.
func SearchEmails(db *store.DB, query string, limit int) ([]Email, error) {
	if limit <= 0 {
		limit = 50
	}
	pattern := "%" + strings.ToLower(query) + "%"

	rows, err := db.Query(
		db.Rebind(`SELECT id, message_id, account, from_addr, to_addr, subject, date, body, html_body
		 FROM gmail_emails
		 WHERE LOWER(subject) LIKE ? OR LOWER(body) LIKE ?
		 ORDER BY date DESC LIMIT ?`),
		pattern, pattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search emails: %w", err)
	}
	defer rows.Close()

	var emails []Email
	for rows.Next() {
		var e Email
		var id int64
		if err := rows.Scan(&id, &e.MessageID, &e.Account, &e.From, &e.To, &e.Subject, &e.Date, &e.Body, &e.HTMLBody); err != nil {
			return nil, fmt.Errorf("scan email: %w", err)
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// CountEmails returns the total number of stored emails, optionally filtered by account.
func CountEmails(db *store.DB, account string) (int64, error) {
	query := "SELECT COUNT(*) FROM gmail_emails"
	var args []interface{}
	if account != "" {
		query += " WHERE account = ?"
		args = append(args, account)
	}

	var count int64
	err := db.QueryRow(db.Rebind(query), args...).Scan(&count)
	return count, err
}

// LastSyncTime returns the most recent fetched_at time from gmail_emails.
func LastSyncTime(db *store.DB) (*time.Time, error) {
	var t time.Time
	err := db.QueryRow("SELECT MAX(fetched_at) FROM gmail_emails").Scan(&t)
	if err != nil {
		return nil, err
	}
	if t.IsZero() {
		return nil, nil
	}
	return &t, nil
}

// AttachmentRow holds attachment metadata from a query.
type AttachmentRow struct {
	EmailMessageID string
	Filename       string
	MimeType       string
	SavedPath      string
}

// ListAttachments returns attachment metadata, optionally filtered by email message ID.
func ListAttachments(db *store.DB, emailMessageID string) ([]AttachmentRow, error) {
	query := `SELECT e.message_id, a.filename, a.mime_type, a.saved_path
		FROM gmail_attachments a
		JOIN gmail_emails e ON e.id = a.email_id`
	var args []interface{}
	if emailMessageID != "" {
		query += " WHERE e.message_id = ?"
		args = append(args, emailMessageID)
	}
	query += " ORDER BY e.date DESC, a.filename"

	rows, err := db.Query(db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	var results []AttachmentRow
	for rows.Next() {
		var r AttachmentRow
		if err := rows.Scan(&r.EmailMessageID, &r.Filename, &r.MimeType, &r.SavedPath); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

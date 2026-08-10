package scheduler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/73ai/openbotkit/store"
)

var triggerTemplates = map[string]string{
	"gmail":      `SELECT id, subject, from_addr, date FROM emails WHERE id > ? AND (%s) ORDER BY id LIMIT 50`,
	"whatsapp":   `SELECT id, message_id, sender_name, text, timestamp FROM whatsapp_messages WHERE id > ? AND (%s) ORDER BY id LIMIT 50`,
	"imessage":   `SELECT id, guid, sender_id, text, date_utc FROM imessage_messages WHERE id > ? AND (%s) ORDER BY id LIMIT 50`,
	"applenotes": `SELECT id, title, folder, modified_at FROM applenotes_notes WHERE id > ? AND (%s) ORDER BY id LIMIT 50`,
	"telegram":   `SELECT id, message_id, sender_name, text, timestamp FROM telegram_messages WHERE id > ? AND (%s) ORDER BY id LIMIT 50`,
}

func ValidateTriggerSource(source string) error {
	if _, ok := triggerTemplates[source]; !ok {
		return fmt.Errorf("unknown trigger source %q; supported: gmail, whatsapp, telegram, imessage, applenotes", source)
	}
	return nil
}

// allowedColumns defines which column names are permitted per trigger source.
var allowedColumns = map[string]map[string]bool{
	"gmail":      {"SUBJECT": true, "FROM_ADDR": true, "DATE": true},
	"whatsapp":   {"MESSAGE_ID": true, "SENDER_NAME": true, "TEXT": true, "TIMESTAMP": true},
	"imessage":   {"GUID": true, "SENDER_ID": true, "TEXT": true, "DATE_UTC": true},
	"applenotes": {"TITLE": true, "FOLDER": true, "MODIFIED_AT": true},
	"telegram":   {"MESSAGE_ID": true, "SENDER_NAME": true, "TEXT": true, "TIMESTAMP": true},
}

// allowedKeywords are SQL keywords permitted in trigger WHERE clauses.
var allowedKeywords = map[string]bool{
	"AND": true, "OR": true, "NOT": true,
	"LIKE": true, "GLOB": true, "ESCAPE": true,
	"IN": true, "BETWEEN": true,
	"IS": true, "NULL": true,
}

var identPattern = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

// ValidateTriggerQuery validates that a trigger WHERE clause only uses allowed
// columns and keywords. Uses an allowlist approach instead of a denylist
// to prevent SQL injection via UNION, subqueries, functions, etc.
func ValidateTriggerQuery(source, query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("trigger query must not be empty")
	}
	if strings.Contains(query, "--") {
		return fmt.Errorf("trigger query must not contain SQL comments")
	}
	if strings.Contains(query, ";") {
		return fmt.Errorf("trigger query must not contain semicolons")
	}
	if strings.Count(query, "(") != strings.Count(query, ")") {
		return fmt.Errorf("trigger query has unbalanced parentheses")
	}

	cols := allowedColumns[source]
	if cols == nil {
		return fmt.Errorf("unknown trigger source %q", source)
	}

	// Strip string literals to avoid false positives on keywords inside strings.
	stripped := stripStringLiterals(query)

	// Every identifier must be an allowed column or SQL keyword.
	idents := identPattern.FindAllString(stripped, -1)
	for _, ident := range idents {
		upper := strings.ToUpper(ident)
		if cols[upper] || allowedKeywords[upper] {
			continue
		}
		return fmt.Errorf("trigger query contains disallowed identifier %q", ident)
	}

	return nil
}

// stripStringLiterals removes content between single quotes (SQL string literals).
func stripStringLiterals(s string) string {
	var b strings.Builder
	inStr := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			if inStr && i+1 < len(s) && s[i+1] == '\'' {
				i++ // skip escaped quote ('')
				continue
			}
			inStr = !inStr
			continue
		}
		if !inStr {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func BuildTriggerQuery(source, whereClause string, lastTriggerID int64) (string, []any, error) {
	tmpl, ok := triggerTemplates[source]
	if !ok {
		return "", nil, fmt.Errorf("unknown trigger source %q", source)
	}
	q := fmt.Sprintf(tmpl, whereClause)
	return q, []any{lastTriggerID}, nil
}

type TriggerMatch struct {
	MaxID int64
	Rows  []map[string]string
}

func CheckTrigger(db *store.DB, source, whereClause string, lastTriggerID int64) (*TriggerMatch, error) {
	query, args, err := BuildTriggerQuery(source, whereClause, lastTriggerID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("trigger query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("trigger columns: %w", err)
	}

	var match TriggerMatch
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("trigger scan: %w", err)
		}
		row := make(map[string]string, len(cols))
		for i, col := range cols {
			row[col] = fmt.Sprintf("%v", vals[i])
		}
		if id, ok := row["id"]; ok {
			var n int64
			fmt.Sscanf(id, "%d", &n)
			if n > match.MaxID {
				match.MaxID = n
			}
		}
		match.Rows = append(match.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trigger rows: %w", err)
	}

	if len(match.Rows) == 0 {
		return nil, nil
	}
	return &match, nil
}

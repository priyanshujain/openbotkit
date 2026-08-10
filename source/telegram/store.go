package telegram

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/73ai/openbotkit/store"
)

func SaveMessage(db *store.DB, msg *Message) error {
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_messages
			(message_id, chat_id, sender_id, sender_name, text, timestamp, media_type, reply_to_id, is_outgoing, edit_date)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(message_id, chat_id) DO UPDATE SET
				text = excluded.text,
				sender_name = CASE WHEN excluded.sender_name != '' THEN excluded.sender_name ELSE telegram_messages.sender_name END,
				media_type = excluded.media_type,
				edit_date = excluded.edit_date`),
		msg.MessageID, msg.ChatID, msg.SenderID, msg.SenderName,
		msg.Text, msg.Timestamp.UTC(), msg.MediaType, msg.ReplyToID,
		msg.IsOutgoing, msg.EditDate,
	)
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func DeleteMessages(db *store.DB, chatID int64, messageIDs []int) error {
	for _, id := range messageIDs {
		_, err := db.Exec(
			db.Rebind("DELETE FROM telegram_messages WHERE chat_id = ? AND message_id = ?"),
			chatID, id,
		)
		if err != nil {
			return fmt.Errorf("delete message %d: %w", id, err)
		}
	}
	return nil
}

// DeleteMessagesNonChannel removes messages by ID across every non-channel peer.
// updateDeleteMessages carries no peer, but non-channel message IDs share one
// per-account sequence, so the ID alone identifies the row.
func DeleteMessagesNonChannel(db *store.DB, messageIDs []int) error {
	for _, id := range messageIDs {
		_, err := db.Exec(
			db.Rebind("DELETE FROM telegram_messages WHERE message_id = ? AND chat_id > ?"),
			id, int64(-channelIDOffset),
		)
		if err != nil {
			return fmt.Errorf("delete message %d: %w", id, err)
		}
	}
	return nil
}

func UpsertChat(db *store.DB, c *Chat) error {
	var lastMsg any
	if c.LastMessageAt != nil {
		lastMsg = c.LastMessageAt.UTC()
	}
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_chats (chat_id, type, title, username, access_hash, is_group, is_channel, last_message_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(chat_id) DO UPDATE SET
				type = excluded.type,
				title = CASE WHEN excluded.title != '' THEN excluded.title ELSE telegram_chats.title END,
				username = CASE WHEN excluded.username != '' THEN excluded.username ELSE telegram_chats.username END,
				access_hash = CASE WHEN excluded.access_hash != 0 THEN excluded.access_hash ELSE telegram_chats.access_hash END,
				is_group = excluded.is_group,
				is_channel = excluded.is_channel,
				last_message_at = COALESCE(excluded.last_message_at, telegram_chats.last_message_at)`),
		c.ChatID, c.Type, c.Title, c.Username, c.AccessHash, c.IsGroup, c.IsChannel, lastMsg,
	)
	if err != nil {
		return fmt.Errorf("upsert chat: %w", err)
	}
	return nil
}

// SetChatAccessHash records an access hash without touching identity columns.
// The update manager calls this with nothing but an ID and a hash, so a full
// upsert would reset is_group/is_channel on every supergroup it sees.
func SetChatAccessHash(db *store.DB, chatID, accessHash int64) error {
	stub := stubChat(chatID)
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_chats (chat_id, type, title, username, access_hash, is_group, is_channel)
			VALUES (?, ?, '', '', ?, ?, ?)
			ON CONFLICT(chat_id) DO UPDATE SET access_hash = excluded.access_hash`),
		chatID, stub.Type, accessHash, stub.IsGroup, stub.IsChannel,
	)
	if err != nil {
		return fmt.Errorf("set chat access hash: %w", err)
	}
	return nil
}

// TouchChatLastMessage records activity on a chat whose identity we cannot
// resolve, leaving every other column alone.
func TouchChatLastMessage(db *store.DB, chatID int64, at time.Time) error {
	stub := stubChat(chatID)
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_chats (chat_id, type, title, username, is_group, is_channel, last_message_at)
			VALUES (?, ?, '', '', ?, ?, ?)
			ON CONFLICT(chat_id) DO UPDATE SET last_message_at = excluded.last_message_at`),
		chatID, stub.Type, stub.IsGroup, stub.IsChannel, at.UTC(),
	)
	if err != nil {
		return fmt.Errorf("touch chat: %w", err)
	}
	return nil
}

func GetChat(db *store.DB, chatID int64) (*Chat, error) {
	var c Chat
	var lastMsg sql.NullTime
	err := db.QueryRow(
		db.Rebind(`SELECT chat_id, type, title, username, access_hash, is_group, is_channel, last_message_at
			FROM telegram_chats WHERE chat_id = ?`),
		chatID,
	).Scan(&c.ChatID, &c.Type, &c.Title, &c.Username, &c.AccessHash, &c.IsGroup, &c.IsChannel, &lastMsg)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get chat: %w", err)
	}
	if lastMsg.Valid {
		c.LastMessageAt = &lastMsg.Time
	}
	return &c, nil
}

const chatColumns = `chat_id, type, title, username, access_hash, is_group, is_channel, last_message_at`

func scanChats(rows *sql.Rows) ([]Chat, error) {
	chats := []Chat{}
	for rows.Next() {
		var c Chat
		var lastMsg sql.NullTime
		if err := rows.Scan(&c.ChatID, &c.Type, &c.Title, &c.Username, &c.AccessHash,
			&c.IsGroup, &c.IsChannel, &lastMsg); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		if lastMsg.Valid {
			c.LastMessageAt = &lastMsg.Time
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

func ListChats(db *store.DB) ([]Chat, error) {
	rows, err := db.Query("SELECT " + chatColumns + " FROM telegram_chats ORDER BY last_message_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()
	return scanChats(rows)
}

// FindChats matches chats by a fragment of their title or username. It is a
// search, not a send target: use ChatsByUsername or ChatsByTitle for that.
func FindChats(db *store.DB, query string, limit int) ([]Chat, error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := likePattern(query)
	rows, err := db.Query(
		db.Rebind(`SELECT `+chatColumns+` FROM telegram_chats
			WHERE LOWER(title) LIKE ? ESCAPE '\' OR LOWER(username) LIKE ? ESCAPE '\'
			ORDER BY last_message_at DESC LIMIT ?`),
		pattern, pattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("find chats: %w", err)
	}
	defer rows.Close()
	return scanChats(rows)
}

// ChatsByUsername returns chats whose username is exactly username, ignoring
// case. Usernames are unique on Telegram, so this is at most one row.
func ChatsByUsername(db *store.DB, username string) ([]Chat, error) {
	rows, err := db.Query(
		db.Rebind(`SELECT `+chatColumns+` FROM telegram_chats
			WHERE username != '' AND LOWER(username) = ?
			ORDER BY last_message_at DESC`),
		strings.ToLower(username),
	)
	if err != nil {
		return nil, fmt.Errorf("find chats by username: %w", err)
	}
	defer rows.Close()
	return scanChats(rows)
}

// ChatsByTitle returns chats whose title is exactly title, ignoring case.
// Titles are not unique, so callers still have to handle several matches.
func ChatsByTitle(db *store.DB, title string) ([]Chat, error) {
	rows, err := db.Query(
		db.Rebind(`SELECT `+chatColumns+` FROM telegram_chats
			WHERE title != '' AND LOWER(title) = ?
			ORDER BY last_message_at DESC`),
		strings.ToLower(title),
	)
	if err != nil {
		return nil, fmt.Errorf("find chats by title: %w", err)
	}
	defer rows.Close()
	return scanChats(rows)
}

func UpsertUser(db *store.DB, u *User) error {
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_users (user_id, username, first_name, last_name, phone, access_hash, is_bot, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET
				username = CASE WHEN excluded.username != '' THEN excluded.username ELSE telegram_users.username END,
				first_name = CASE WHEN excluded.first_name != '' THEN excluded.first_name ELSE telegram_users.first_name END,
				last_name = CASE WHEN excluded.last_name != '' THEN excluded.last_name ELSE telegram_users.last_name END,
				phone = CASE WHEN excluded.phone != '' THEN excluded.phone ELSE telegram_users.phone END,
				access_hash = CASE WHEN excluded.access_hash != 0 THEN excluded.access_hash ELSE telegram_users.access_hash END,
				is_bot = excluded.is_bot,
				updated_at = CURRENT_TIMESTAMP`),
		u.UserID, u.Username, u.FirstName, u.LastName, u.Phone, u.AccessHash, u.IsBot,
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

// SetUserAccessHash records an access hash without touching identity columns,
// so a refresh cannot flip is_bot or blank a name.
func SetUserAccessHash(db *store.DB, userID, accessHash int64) error {
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_users (user_id, username, first_name, last_name, phone, access_hash, updated_at)
			VALUES (?, '', '', '', '', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET
				access_hash = excluded.access_hash,
				updated_at = CURRENT_TIMESTAMP`),
		userID, accessHash,
	)
	if err != nil {
		return fmt.Errorf("set user access hash: %w", err)
	}
	return nil
}

func GetUser(db *store.DB, userID int64) (*User, error) {
	var u User
	err := db.QueryRow(
		db.Rebind(`SELECT user_id, username, first_name, last_name, phone, access_hash, is_bot
			FROM telegram_users WHERE user_id = ?`),
		userID,
	).Scan(&u.UserID, &u.Username, &u.FirstName, &u.LastName, &u.Phone, &u.AccessHash, &u.IsBot)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func ListUsers(db *store.DB, query string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 50
	}

	var rows *sql.Rows
	var err error
	if query != "" {
		pattern := likePattern(query)
		rows, err = db.Query(
			db.Rebind(`SELECT user_id, username, first_name, last_name, phone, access_hash, is_bot
				FROM telegram_users
				WHERE LOWER(username) LIKE ? ESCAPE '\' OR LOWER(first_name) LIKE ? ESCAPE '\'
					OR LOWER(last_name) LIKE ? ESCAPE '\' OR phone LIKE ? ESCAPE '\'
				ORDER BY first_name LIMIT ?`),
			pattern, pattern, pattern, pattern, limit,
		)
	} else {
		rows, err = db.Query(
			db.Rebind(`SELECT user_id, username, first_name, last_name, phone, access_hash, is_bot
				FROM telegram_users ORDER BY first_name LIMIT ?`),
			limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Username, &u.FirstName, &u.LastName,
			&u.Phone, &u.AccessHash, &u.IsBot); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// parseDay turns a YYYY-MM-DD filter into a timestamp. Comparing the raw
// string against a stored RFC3339 timestamp looks like it works but drops
// every message after midnight on the final day.
func parseDay(value string) (time.Time, error) {
	day, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", value)
	}
	return day.UTC(), nil
}

// likePattern builds a case-insensitive contains-pattern with the LIKE
// wildcards escaped, so searching "50% off" cannot match "discount 5000 off".
// Queries using it must add ESCAPE '\'.
func likePattern(s string) string {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + esc.Replace(strings.ToLower(s)) + "%"
}

const messageColumns = `message_id, chat_id, sender_id, sender_name, text, timestamp,
	media_type, reply_to_id, is_outgoing, edit_date`

func scanMessages(rows *sql.Rows) ([]Message, error) {
	messages := []Message{}
	for rows.Next() {
		var m Message
		var editDate sql.NullTime
		if err := rows.Scan(&m.MessageID, &m.ChatID, &m.SenderID, &m.SenderName,
			&m.Text, &m.Timestamp, &m.MediaType, &m.ReplyToID,
			&m.IsOutgoing, &editDate); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if editDate.Valid {
			m.EditDate = &editDate.Time
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func ListMessages(db *store.DB, opts ListOptions) ([]Message, error) {
	var conditions []string
	var args []any

	if opts.ChatID != 0 {
		conditions = append(conditions, "chat_id = ?")
		args = append(args, opts.ChatID)
	}
	if opts.After != "" {
		day, err := parseDay(opts.After)
		if err != nil {
			return nil, fmt.Errorf("after: %w", err)
		}
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, day)
	}
	if opts.Before != "" {
		day, err := parseDay(opts.Before)
		if err != nil {
			return nil, fmt.Errorf("before: %w", err)
		}
		conditions = append(conditions, "timestamp < ?")
		args = append(args, day.AddDate(0, 0, 1))
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
		"SELECT %s FROM telegram_messages %s ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		messageColumns, where)
	args = append(args, limit, opts.Offset)

	rows, err := db.Query(db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func SearchMessages(db *store.DB, query string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	pattern := likePattern(query)

	rows, err := db.Query(
		db.Rebind(fmt.Sprintf(
			`SELECT %s FROM telegram_messages WHERE LOWER(text) LIKE ? ESCAPE '\' ORDER BY timestamp DESC LIMIT ?`,
			messageColumns)),
		pattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func MessageExists(db *store.DB, messageID int, chatID int64) (bool, error) {
	var count int
	err := db.QueryRow(
		db.Rebind("SELECT COUNT(*) FROM telegram_messages WHERE message_id = ? AND chat_id = ?"),
		messageID, chatID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check message exists: %w", err)
	}
	return count > 0, nil
}

func CountMessages(db *store.DB, chatID int64) (int64, error) {
	query := "SELECT COUNT(*) FROM telegram_messages"
	var args []any
	if chatID != 0 {
		query += " WHERE chat_id = ?"
		args = append(args, chatID)
	}

	var count int64
	err := db.QueryRow(db.Rebind(query), args...).Scan(&count)
	return count, err
}

func LastSyncTime(db *store.DB) (*time.Time, error) {
	var raw sql.NullString
	err := db.QueryRow("SELECT MAX(synced_at) FROM telegram_messages").Scan(&raw)
	if err != nil {
		return nil, err
	}
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, raw.String); err == nil {
			return &t, nil
		}
	}
	return nil, nil
}

func GetSyncState(db *store.DB, chatID int64) (*SyncState, error) {
	var s SyncState
	var until sql.NullTime
	err := db.QueryRow(
		db.Rebind("SELECT chat_id, min_id, max_id, backfilled_until FROM telegram_sync_state WHERE chat_id = ?"),
		chatID,
	).Scan(&s.ChatID, &s.MinID, &s.MaxID, &until)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sync state: %w", err)
	}
	if until.Valid {
		s.BackfilledUntil = &until.Time
	}
	return &s, nil
}

func SaveSyncState(db *store.DB, s *SyncState) error {
	var until any
	if s.BackfilledUntil != nil {
		until = s.BackfilledUntil.UTC()
	}
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_sync_state (chat_id, min_id, max_id, backfilled_until, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(chat_id) DO UPDATE SET
				min_id = excluded.min_id,
				max_id = excluded.max_id,
				backfilled_until = excluded.backfilled_until,
				updated_at = CURRENT_TIMESTAMP`),
		s.ChatID, s.MinID, s.MaxID, until,
	)
	if err != nil {
		return fmt.Errorf("save sync state: %w", err)
	}
	return nil
}

// GetKV reads a telegram_updates_state value. Missing keys return ("", false, nil).
func GetKV(db *store.DB, key string) (string, bool, error) {
	var value string
	err := db.QueryRow(
		db.Rebind("SELECT value FROM telegram_updates_state WHERE key = ?"),
		key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get %q: %w", key, err)
	}
	return value, true, nil
}

func SetKV(db *store.DB, key, value string) error {
	_, err := db.Exec(
		db.Rebind(`INSERT INTO telegram_updates_state (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`),
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set %q: %w", key, err)
	}
	return nil
}

// SetKVBatch writes several keys in one statement, which both SQLite and
// Postgres apply atomically. The updates state is four keys that only mean
// anything together: a crash between separate writes would leave a state that
// reads back as valid with zeroed fields and silently breaks gap recovery.
func SetKVBatch(db *store.DB, pairs map[string]string) error {
	if len(pairs) == 0 {
		return nil
	}

	rows := make([]string, 0, len(pairs))
	args := make([]any, 0, len(pairs)*2)
	for key, value := range pairs {
		rows = append(rows, "(?, ?)")
		args = append(args, key, value)
	}

	_, err := db.Exec(
		db.Rebind("INSERT INTO telegram_updates_state (key, value) VALUES "+
			strings.Join(rows, ", ")+
			" ON CONFLICT(key) DO UPDATE SET value = excluded.value"),
		args...,
	)
	if err != nil {
		return fmt.Errorf("set %d keys: %w", len(pairs), err)
	}
	return nil
}

// ListKVPrefix returns all key/value pairs whose key starts with prefix.
func ListKVPrefix(db *store.DB, prefix string) (map[string]string, error) {
	rows, err := db.Query(
		db.Rebind("SELECT key, value FROM telegram_updates_state WHERE key LIKE ?"),
		prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", prefix, err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan kv: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

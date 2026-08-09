package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type sessionIndex struct {
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
	StartedAt string `json:"started_at"`
	UpdatedAt string `json:"updated_at"`
	Ended     bool   `json:"ended"`
}

type messageEntry struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) indexPath() string {
	return filepath.Join(s.dir, "sessions.jsonl")
}

func (s *Store) sessionPath(sessionID string) string {
	return filepath.Join(s.dir, "sessions", filepath.Base(sessionID)+".jsonl")
}

func validateSessionID(sessionID string) error {
	if !validSessionID.MatchString(sessionID) {
		return fmt.Errorf("invalid session ID: %q", sessionID)
	}
	return nil
}

func (s *Store) UpsertConversation(sessionID, cwd string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	entry := sessionIndex{
		SessionID: sessionID,
		CWD:       cwd,
		StartedAt: now,
		UpdatedAt: now,
	}
	return s.appendIndex(entry)
}

func (s *Store) SaveMessage(sessionID, role, content string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := messageEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}

	path := s.sessionPath(sessionID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(entry); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	return nil
}

func (s *Store) LoadRecentSession(cwd string, maxAge time.Duration) (*RecentSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadIndex()
	var best *RecentSession
	for _, entry := range idx {
		if entry.CWD != cwd || entry.Ended {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, entry.UpdatedAt)
		if err != nil {
			continue
		}
		if time.Since(ts) > maxAge {
			continue
		}
		if best == nil || ts.After(best.UpdatedAt) {
			best = &RecentSession{SessionID: entry.SessionID, UpdatedAt: ts}
		}
	}
	return best, nil
}

func (s *Store) LoadSessionMessages(sessionID string, limit int) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadSessionMessages(sessionID, limit)
}

func (s *Store) loadSessionMessages(sessionID string, limit int) ([]Message, error) {
	path := s.sessionPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var entry messageEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			slog.Warn("history: skipping malformed message line", "session", sessionID, "error", err)
			continue
		}
		m := Message{
			Role:    entry.Role,
			Content: entry.Content,
		}
		if ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
			m.Timestamp = ts
		}
		msgs = append(msgs, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session file: %w", err)
	}

	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

func (s *Store) EndSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadIndex()
	entry, ok := idx[sessionID]
	if !ok {
		entry = sessionIndex{
			SessionID: sessionID,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	entry.Ended = true
	return s.appendIndex(entry)
}

func (s *Store) CountConversations() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadIndex()
	return int64(len(idx)), nil
}

func (s *Store) LastCaptureTime() (*time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadIndex()
	var latest *time.Time
	for _, entry := range idx {
		ts, err := time.Parse(time.RFC3339Nano, entry.UpdatedAt)
		if err != nil {
			continue
		}
		if latest == nil || ts.After(*latest) {
			latest = &ts
		}
	}
	return latest, nil
}

func (s *Store) MessageCountForSession(sessionID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

// LoadRecentUserMessages loads user messages from the most recent N sessions.
func (s *Store) LoadRecentUserMessages(lastN int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadIndex()

	type indexedEntry struct {
		entry     sessionIndex
		updatedAt time.Time
	}
	var entries []indexedEntry
	for _, e := range idx {
		ts, err := time.Parse(time.RFC3339Nano, e.UpdatedAt)
		if err != nil {
			continue
		}
		entries = append(entries, indexedEntry{entry: e, updatedAt: ts})
	}

	// Sort by updated_at desc.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].updatedAt.After(entries[j].updatedAt)
	})

	if lastN > 0 && len(entries) > lastN {
		entries = entries[:lastN]
	}

	var messages []string
	for _, e := range entries {
		msgs, err := s.loadSessionMessages(e.entry.SessionID, 0)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			if m.Role == "user" {
				messages = append(messages, m.Content)
			}
		}
	}
	return messages, nil
}

// loadIndex reads sessions.jsonl and builds a map of session_id → latest entry.
// Must be called with s.mu held.
func (s *Store) loadIndex() map[string]sessionIndex {
	idx := make(map[string]sessionIndex)
	f, err := os.Open(s.indexPath())
	if err != nil {
		return idx
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var entry sessionIndex
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			slog.Warn("history: skipping malformed index line", "error", err)
			continue
		}
		idx[entry.SessionID] = entry
	}
	return idx
}

// appendIndex appends one index line to sessions.jsonl.
// Must be called with s.mu held.
func (s *Store) appendIndex(entry sessionIndex) error {
	f, err := os.OpenFile(s.indexPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(entry)
}

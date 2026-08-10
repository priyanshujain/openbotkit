package websearch

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/73ai/openbotkit/store"
)

func cacheKey(query, category, backend, region, timeLimit string, page int) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%d", query, category, backend, region, timeLimit, page)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func getSearchCache(db *store.DB, key string, ttl time.Duration) (*SearchResult, bool) {
	if db == nil {
		return nil, false
	}

	var query, resultsJSON string
	var createdAt time.Time
	q := db.Rebind("SELECT query, results, created_at FROM search_cache WHERE cache_key = ?")
	err := db.QueryRow(q, key).Scan(&query, &resultsJSON, &createdAt)
	if err != nil {
		return nil, false
	}

	if time.Since(createdAt) > ttl {
		delQ := db.Rebind("DELETE FROM search_cache WHERE cache_key = ?")
		db.Exec(delQ, key)
		return nil, false
	}

	var results []Result
	if err := json.Unmarshal([]byte(resultsJSON), &results); err != nil {
		return nil, false
	}

	return &SearchResult{
		Query:   query,
		Results: results,
		Metadata: SearchMetadata{
			Backends:     []string{"cache"},
			TotalResults: len(results),
			Cached:       true,
		},
	}, true
}

func putSearchCache(db *store.DB, key, query, category string, results []Result) {
	if db == nil {
		return
	}

	data, err := json.Marshal(results)
	if err != nil {
		return
	}

	q := db.Rebind(`INSERT INTO search_cache (cache_key, query, category, results) VALUES (?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET results = excluded.results, created_at = CURRENT_TIMESTAMP`)
	if _, err := db.Exec(q, key, query, category, string(data)); err != nil {
		slog.Warn("failed to write search cache", "error", err)
	}
}

func getFetchCache(db *store.DB, url, format string, ttl time.Duration) (*FetchResult, bool) {
	if db == nil {
		return nil, false
	}

	var title, content, contentType, storedFormat string
	var statusCode int
	var fetchedAt time.Time
	q := db.Rebind("SELECT title, content, content_type, format, status_code, fetched_at FROM fetch_cache WHERE url = ?")
	err := db.QueryRow(q, url).Scan(&title, &content, &contentType, &storedFormat, &statusCode, &fetchedAt)
	if err != nil {
		return nil, false
	}

	if time.Since(fetchedAt) > ttl {
		delQ := db.Rebind("DELETE FROM fetch_cache WHERE url = ?")
		db.Exec(delQ, url)
		return nil, false
	}

	if storedFormat != format {
		return nil, false
	}

	return &FetchResult{
		URL:         url,
		Title:       title,
		Content:     content,
		ContentType: contentType,
		StatusCode:  statusCode,
		Cached:      true,
	}, true
}

func putFetchCache(db *store.DB, result *FetchResult, format string) {
	if db == nil || result == nil {
		return
	}

	q := db.Rebind(`INSERT INTO fetch_cache (url, title, content, content_type, format, status_code) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET title = excluded.title, content = excluded.content,
		content_type = excluded.content_type, format = excluded.format, status_code = excluded.status_code, fetched_at = CURRENT_TIMESTAMP`)
	if _, err := db.Exec(q, result.URL, result.Title, result.Content, result.ContentType, format, result.StatusCode); err != nil {
		slog.Warn("failed to write fetch cache", "error", err)
	}
}

// SearchHistoryEntry is one line in the search history JSONL file.
type SearchHistoryEntry struct {
	Query       string `json:"query"`
	Category    string `json:"category"`
	ResultCount int    `json:"result_count"`
	Backends    string `json:"backends"`
	SearchMs    int64  `json:"search_ms"`
	CreatedAt   string `json:"created_at"`
}

func putSearchHistory(path string, query, category string, resultCount int, backends []string, searchMs int64) {
	if path == "" {
		return
	}

	entry := SearchHistoryEntry{
		Query:       query,
		Category:    category,
		ResultCount: resultCount,
		Backends:    strings.Join(backends, ","),
		SearchMs:    searchMs,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		slog.Warn("failed to open search history file", "error", err)
		return
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		slog.Warn("failed to write search history", "error", err)
	}
}

func LoadSearchHistory(path string, limit int) ([]SearchHistoryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open search history: %w", err)
	}
	defer f.Close()

	var all []SearchHistoryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e SearchHistoryEntry
		if err := json.Unmarshal(line, &e); err != nil {
			slog.Warn("websearch: skipping malformed history line", "error", err)
			continue
		}
		all = append(all, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan search history: %w", err)
	}

	// Return last N entries in reverse chronological order.
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}

func countSearchHistory(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("open search history: %w", err)
	}
	defer f.Close()

	var count int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

func (w *WebSearch) ClearCaches() error {
	return clearAllCaches(w.db)
}

func clearAllCaches(db *store.DB) error {
	if db == nil {
		return nil
	}

	if _, err := db.Exec("DELETE FROM search_cache"); err != nil {
		return fmt.Errorf("clear search cache: %w", err)
	}
	if _, err := db.Exec("DELETE FROM fetch_cache"); err != nil {
		return fmt.Errorf("clear fetch cache: %w", err)
	}
	return nil
}

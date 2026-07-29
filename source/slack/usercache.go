package slack

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/73ai/openbotkit/config"
)

// UserCache maps Slack IDs to display names, persisted per workspace.
//
// It resolves one ID at a time via users.info (Tier 4, ~100/min) rather than
// paging users.list (Tier 2), because a thread references a handful of people
// while a large workspace needs dozens of list calls.
type UserCache struct {
	workspace string
	api       API

	mu     sync.Mutex
	names  map[string]string
	loaded bool
	dirty  bool
}

type cachedUserNames struct {
	Workspace string            `json:"workspace"`
	Names     map[string]string `json:"names"`
}

func NewUserCache(workspace string, api API) *UserCache {
	return &UserCache{
		workspace: SanitizeWorkspaceName(workspace),
		api:       api,
		names:     make(map[string]string),
	}
}

// Refresh discards cached names so the next lookup re-fetches them. The cache
// file is overwritten on the next save rather than deleted.
func (c *UserCache) Refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.names = make(map[string]string)
	c.loaded = true
	c.dirty = true
}

// Seed records a name discovered outside the API, such as the username a bot
// message carries inline. Existing entries are not overwritten.
func (c *UserCache) Seed(id, name string) {
	if id == "" || name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	if _, ok := c.names[id]; ok {
		return
	}
	c.names[id] = name
	c.dirty = true
}

// Names resolves ids to display names, fetching only the ones not yet cached.
// IDs that cannot be resolved are simply absent from the result.
func (c *UserCache) Names(ctx context.Context, ids []string) (map[string]string, error) {
	c.mu.Lock()
	c.load()
	var missing []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := c.names[id]; !ok {
			missing = append(missing, id)
		}
	}
	c.mu.Unlock()

	for _, id := range missing {
		name, ok := c.fetchName(ctx, id)
		if !ok {
			continue
		}
		c.mu.Lock()
		c.names[id] = name
		c.dirty = true
		c.mu.Unlock()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		if name, ok := c.names[id]; ok {
			out[id] = name
		}
	}
	c.save()
	return out, nil
}

func (c *UserCache) fetchName(ctx context.Context, id string) (string, bool) {
	if strings.HasPrefix(id, "B") {
		bot, err := c.api.BotsInfo(ctx, id)
		if err != nil || bot == nil || bot.Name == "" {
			slog.Debug("slack user cache: bot lookup failed", "id", id, "error", err)
			return "", false
		}
		return bot.Name, true
	}

	user, err := c.api.UsersInfo(ctx, id)
	if err != nil || user == nil {
		slog.Debug("slack user cache: user lookup failed", "id", id, "error", err)
		return "", false
	}
	return displayName(user), true
}

func displayName(u *User) string {
	if u.Profile != nil {
		if u.Profile.DisplayName != "" {
			return u.Profile.DisplayName
		}
		if u.Profile.RealName != "" {
			return u.Profile.RealName
		}
	}
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.RealName != "" {
		return u.RealName
	}
	if u.Name != "" {
		return u.Name
	}
	return u.ID
}

// load reads the cache file once. Callers must hold c.mu.
func (c *UserCache) load() {
	if c.loaded {
		return
	}
	c.loaded = true

	data, err := os.ReadFile(c.path())
	if err != nil {
		return
	}
	var cached cachedUserNames
	if err := json.Unmarshal(data, &cached); err != nil {
		slog.Debug("slack user cache: unreadable cache file", "path", c.path(), "error", err)
		return
	}
	maps.Copy(c.names, cached.Names)
}

// save writes the cache file when there is something new. Callers must hold c.mu.
func (c *UserCache) save() {
	if !c.dirty || c.workspace == "" {
		return
	}
	if err := config.EnsureSourceDir("slack"); err != nil {
		slog.Warn("slack user cache: create dir failed", "error", err)
		return
	}
	data, err := json.MarshalIndent(cachedUserNames{Workspace: c.workspace, Names: c.names}, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(c.path(), data, 0600); err != nil {
		slog.Warn("slack user cache: write failed", "path", c.path(), "error", err)
		return
	}
	c.dirty = false
}

func (c *UserCache) path() string {
	return filepath.Join(config.SourceDir("slack"), "users-"+c.workspace+".json")
}

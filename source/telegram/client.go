package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"golang.org/x/time/rate"

	"github.com/73ai/openbotkit/provider"
)

const (
	APIIDRef   = "keychain:obk/telegram/api_id"
	APIHashRef = "keychain:obk/telegram/api_hash"

	envAPIID   = "TELEGRAM_API_ID"
	envAPIHash = "TELEGRAM_API_HASH"
)

// Credentials resolves the app's api_id/api_hash from the keyring, falling back
// to environment variables for headless runs. Each user registers their own at
// my.telegram.org: a shipped pair would collect API_ID_PUBLISHED_FLOOD for
// everyone and a single revocation would break every install.
func Credentials() (int, string, error) {
	rawID, err := provider.ResolveAPIKey(APIIDRef, envAPIID)
	if err != nil {
		return 0, "", fmt.Errorf("telegram api_id not configured — run 'obk setup telegram' (or set %s/%s)", envAPIID, envAPIHash)
	}
	hash, err := provider.ResolveAPIKey(APIHashRef, envAPIHash)
	if err != nil {
		return 0, "", fmt.Errorf("telegram api_hash not configured — run 'obk setup telegram' (or set %s/%s)", envAPIID, envAPIHash)
	}
	id, err := parseAPIID(rawID)
	if err != nil {
		return 0, "", err
	}
	return id, hash, nil
}

// HasCredentials reports whether an api_id and api_hash can be resolved at
// all, from the keyring or the environment. It is the single answer to "is
// Telegram configured": checking a config ref instead would call a working
// headless setup unconfigured and block login.
func HasCredentials() bool {
	_, _, err := Credentials()
	return err == nil
}

func parseAPIID(raw string) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("telegram api_id %q is not a number", raw)
	}
	return id, nil
}

// StoreCredentials writes api_id/api_hash into the OS keyring.
func StoreCredentials(apiID int, apiHash string) error {
	if err := provider.StoreCredential(APIIDRef, strconv.Itoa(apiID)); err != nil {
		return fmt.Errorf("store api_id: %w", err)
	}
	if err := provider.StoreCredential(APIHashRef, apiHash); err != nil {
		return fmt.Errorf("store api_hash: %w", err)
	}
	return nil
}

// Client wraps gotd's MTProto client with the session storage, update manager
// and rate limiting this source needs.
type Client struct {
	tg          *telegram.Client
	gaps        *updates.Manager
	dispatcher  tg.UpdateDispatcher
	sessionPath string
}

// NewClient builds a client against the session file at sessionPath. The
// dispatcher is wired through the updates manager so pts/qts gaps recover
// automatically; stateStorage may be nil to keep state in memory.
func NewClient(sessionPath string, apiID int, apiHash string, state *StateStorage) (*Client, error) {
	if sessionPath == "" {
		return nil, fmt.Errorf("session path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	dispatcher := tg.NewUpdateDispatcher()
	updatesCfg := updates.Config{Handler: dispatcher}
	if state != nil {
		updatesCfg.Storage = state
		updatesCfg.AccessHasher = state
		updatesCfg.UserAccessHasher = state
	}
	gaps := updates.New(updatesCfg)

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: sessionPath},
		UpdateHandler:  gaps,
		Device:         deviceConfig(),
		Middlewares: []telegram.Middleware{
			floodwait.NewSimpleWaiter().WithMaxRetries(5),
			ratelimit.New(rate.Every(100*time.Millisecond), 5),
		},
	})

	return &Client{
		tg:          client,
		gaps:        gaps,
		dispatcher:  dispatcher,
		sessionPath: sessionPath,
	}, nil
}

// TG exposes the underlying gotd client.
func (c *Client) TG() *telegram.Client { return c.tg }

// API is the raw MTProto escape hatch, the equivalent of WhatsApp's WM().
// Only valid inside Run.
func (c *Client) API() *tg.Client { return c.tg.API() }

// Gaps exposes the update manager, for live sync.
func (c *Client) Gaps() *updates.Manager { return c.gaps }

// Dispatcher exposes the update dispatcher so handlers can be registered
// before Run starts.
func (c *Client) Dispatcher() tg.UpdateDispatcher { return c.dispatcher }

// Run connects and holds the connection open until f returns or ctx is done.
func (c *Client) Run(ctx context.Context, f func(ctx context.Context) error) error {
	return c.tg.Run(ctx, f)
}

// HasSession reports whether a non-empty session file exists. This is the cheap
// local check; it does not prove the session is still valid server-side.
func (c *Client) HasSession() bool {
	return HasSession(c.sessionPath)
}

// HasSession reports whether sessionPath holds a non-empty session.
func HasSession(sessionPath string) bool {
	info, err := os.Stat(sessionPath)
	return err == nil && info.Size() > 0
}

// IsAuthenticated connects and asks the server whether the session is still
// authorised. Returns the signed-in user when it is.
func (c *Client) IsAuthenticated(ctx context.Context) (bool, *tg.User, error) {
	if !c.HasSession() {
		return false, nil, nil
	}

	var (
		ok   bool
		self *tg.User
	)
	err := c.tg.Run(ctx, func(ctx context.Context) error {
		status, err := c.tg.Auth().Status(ctx)
		if err != nil {
			return err
		}
		ok = status.Authorized
		self = status.User
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	return ok, self, nil
}

// Logout revokes the session server-side and only then removes the local
// session file. Dropping the file on a failed revocation would leave the
// device authorised on the account with nothing left to revoke it with.
func (c *Client) Logout(ctx context.Context) error {
	if !c.HasSession() {
		return fmt.Errorf("not authenticated")
	}
	err := c.tg.Run(ctx, func(ctx context.Context) error {
		_, err := c.tg.API().AuthLogOut(ctx)
		return err
	})
	if err != nil && !sessionAlreadyRevoked(err) {
		return fmt.Errorf("revoke session: %w (the local session was left in place; try again when you are online)", err)
	}
	if err := os.Remove(c.sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session: %w", err)
	}
	return nil
}

// sessionAlreadyRevoked reports whether the server considers the session dead
// already, in which case deleting the local file is the whole job.
func sessionAlreadyRevoked(err error) bool {
	return tgerr.Is(err, "AUTH_KEY_UNREGISTERED", "SESSION_REVOKED", "USER_DEACTIVATED")
}

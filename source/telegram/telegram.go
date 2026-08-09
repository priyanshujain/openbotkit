package telegram

import (
	"context"

	"github.com/73ai/openbotkit/source"
	"github.com/73ai/openbotkit/store"
)

type Telegram struct {
	cfg Config
}

func New(cfg Config) *Telegram {
	return &Telegram{cfg: cfg}
}

func (t *Telegram) Name() string {
	return "telegram"
}

// Status reports from local state only. Proving the session is still valid
// server-side needs a connection, which is too heavy for 'obk status'.
func (t *Telegram) Status(ctx context.Context, db *store.DB) (*source.Status, error) {
	st := &source.Status{Connected: HasSession(t.cfg.SessionPath)}
	if db == nil {
		return st, nil
	}

	if _, username, err := LoadSelf(db); err == nil && username != "" {
		st.Accounts = []string{"@" + username}
	}
	st.ItemCount, _ = CountMessages(db, 0)
	st.LastSyncedAt, _ = LastSyncTime(db)

	return st, nil
}

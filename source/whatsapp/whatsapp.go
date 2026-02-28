package whatsapp

import (
	"context"
	"fmt"

	"github.com/priyanshujain/openbotkit/source"
	"github.com/priyanshujain/openbotkit/store"
)

type WhatsApp struct {
	cfg Config
}

func New(cfg Config) *WhatsApp {
	return &WhatsApp{cfg: cfg}
}

func (w *WhatsApp) Name() string {
	return "whatsapp"
}

func (w *WhatsApp) Status(ctx context.Context, db *store.DB) (*source.Status, error) {
	client, err := NewClient(ctx, w.cfg.SessionDBPath)
	if err != nil {
		return &source.Status{Connected: false}, nil
	}
	defer client.Disconnect()

	authenticated := client.IsAuthenticated()

	var accounts []string
	if authenticated {
		accounts = []string{client.WM().Store.ID.User}
	}

	count, _ := CountMessages(db, "")
	lastSync, _ := LastSyncTime(db)

	return &source.Status{
		Connected:    authenticated,
		Accounts:     accounts,
		ItemCount:    count,
		LastSyncedAt: lastSync,
	}, nil
}

func (w *WhatsApp) Login(ctx context.Context) error {
	client, err := NewClient(ctx, w.cfg.SessionDBPath)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	if client.IsAuthenticated() {
		client.Disconnect()
		return fmt.Errorf("already authenticated; run 'obk whatsapp auth logout' first to re-authenticate")
	}

	return ServeQR(ctx, client, ":8085")
}

func (w *WhatsApp) Logout(ctx context.Context) error {
	client, err := NewClient(ctx, w.cfg.SessionDBPath)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer client.Disconnect()

	if !client.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	return client.WM().Logout(ctx)
}

func init() {
	source.Register(&WhatsApp{})
}

package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/73ai/openbotkit/source"
	"github.com/73ai/openbotkit/store"
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

	var dataDB *store.DB
	if w.cfg.DataDSN != "" {
		dataDB, err = store.Open(store.Config{Driver: "sqlite", DSN: w.cfg.DataDSN})
		if err != nil {
			slog.Warn("whatsapp login: could not open data db for history backfill", "error", err)
		} else {
			defer dataDB.Close()
			if err := Migrate(dataDB); err != nil {
				slog.Warn("whatsapp login: migrate data db", "error", err)
				dataDB = nil
			}
		}
	}

	return ServeQR(ctx, client, "127.0.0.1:8085", dataDB)
}

func (w *WhatsApp) Logout(ctx context.Context) error {
	client, err := NewClient(ctx, w.cfg.SessionDBPath)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	if !client.IsAuthenticated() {
		client.Close()
		return fmt.Errorf("not authenticated")
	}

	if err := client.Connect(ctx); err != nil {
		client.Close()
		return fmt.Errorf("connect: %w", err)
	}

	if err := client.WM().Logout(ctx); err != nil {
		client.Close()
		return err
	}

	// Give WhatsApp time to process the unlink before disconnecting.
	time.Sleep(3 * time.Second)
	client.Close()

	// Remove the session DB and WAL/SHM files so no stale state remains.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		os.Remove(w.cfg.SessionDBPath + suffix)
	}
	return nil
}

func init() {
	source.Register(&WhatsApp{})
}

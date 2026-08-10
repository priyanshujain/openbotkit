package daemon

import (
	"context"
	"log/slog"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/73ai/openbotkit/store"
)

// runTelegramSync holds an MTProto connection open until ctx is cancelled.
// Errors are sent on the returned channel (non-blocking).
func runTelegramSync(ctx context.Context, cfg *config.Config, notifier *SyncNotifier) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		if !config.IsSourceLinked("telegram") {
			slog.Info("telegram: not linked, skipping sync")
			return
		}

		sessionPath := cfg.TelegramSessionPath()
		if !tgsrc.HasSession(sessionPath) {
			slog.Warn("telegram: no session, skipping sync")
			return
		}

		apiID, apiHash, err := tgsrc.Credentials()
		if err != nil {
			slog.Error("telegram: credentials unavailable", "error", err)
			errCh <- err
			return
		}

		db, err := store.Open(store.Config{
			Driver: cfg.Telegram.Storage.Driver,
			DSN:    cfg.TelegramDataDSN(),
		})
		if err != nil {
			slog.Error("telegram: failed to open db", "error", err)
			errCh <- err
			return
		}
		defer db.Close()

		client, err := tgsrc.NewClient(sessionPath, apiID, apiHash, tgsrc.NewStateStorage(db))
		if err != nil {
			slog.Error("telegram: failed to create client", "error", err)
			errCh <- err
			return
		}

		opts := tgsrc.LiveOptions{}
		if notifier != nil {
			// Notify on rows actually written, not on a timer, so reactive
			// triggers do not fire on quiet accounts.
			opts.OnChange = func() { notifier.Notify("telegram") }
		}

		slog.Info("telegram: starting sync")
		if err := tgsrc.Live(ctx, client, db, opts); err != nil && ctx.Err() == nil {
			slog.Error("telegram: sync error", "error", err)
			errCh <- err
			return
		}

		slog.Info("telegram: sync stopped")
	}()

	return errCh
}

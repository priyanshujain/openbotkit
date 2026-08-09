package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"

	"github.com/73ai/openbotkit/channel"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/service/hooks"
	"github.com/73ai/openbotkit/store"
)

type Daemon struct {
	cfg            *config.Config
	river          *river.Client[*sql.Tx]
	jobsDB         *sql.DB
	scheduler      *Scheduler
	skipAppleNotes bool
	skipWhatsApp   bool
	skipTelegram   bool
	skipIMessage   bool
	skipContacts   bool
	skipScheduler  bool
}

type Option func(*Daemon)

func WithSkipAppleNotes() Option {
	return func(d *Daemon) { d.skipAppleNotes = true }
}

func WithSkipWhatsApp() Option {
	return func(d *Daemon) { d.skipWhatsApp = true }
}

func WithSkipTelegram() Option {
	return func(d *Daemon) { d.skipTelegram = true }
}

func WithSkipIMessage() Option {
	return func(d *Daemon) { d.skipIMessage = true }
}

func WithSkipContacts() Option {
	return func(d *Daemon) { d.skipContacts = true }
}

func WithSkipScheduler() Option {
	return func(d *Daemon) { d.skipScheduler = true }
}

func New(cfg *config.Config, opts ...Option) *Daemon {
	d := &Daemon{cfg: cfg}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Daemon) Run(ctx context.Context) error {
	if err := config.EnsureDir(); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

	lock, err := acquireLock()
	if err != nil {
		return err
	}
	defer releaseLock(lock)

	slog.Info("starting daemon")

	notifier := NewSyncNotifier()

	chanReg := channel.NewRegistry()
	hooksDB, err := d.openHooksDB()
	if err != nil {
		return fmt.Errorf("open hooks db: %w", err)
	}
	defer hooksDB.Close()

	client, db, err := newRiverClient(ctx, d.cfg, notifier, chanReg, hooksDB)
	if err != nil {
		return fmt.Errorf("init river: %w", err)
	}
	d.river = client
	d.jobsDB = db

	if err := d.river.Start(ctx); err != nil {
		d.jobsDB.Close()
		return fmt.Errorf("start river: %w", err)
	}
	slog.Info("river job queue started")

	hookListener := NewHookListener(d.cfg, d.river, d.jobsDB, notifier, hooksDB)
	go hookListener.Run(ctx)

	if !d.skipScheduler {
		d.scheduler = NewScheduler(d.cfg, d.river, d.jobsDB, notifier)
		if err := d.scheduler.Start(ctx); err != nil {
			slog.Error("scheduler start error", "error", err)
		}
	}

	var waErrChs []<-chan error
	if !d.skipWhatsApp {
		for _, acct := range d.cfg.WhatsAppAccountList() {
			if acct.Role == "source" || acct.Role == "both" {
				ch := runWhatsAppSyncForAccount(ctx, d.cfg, acct.Label, notifier)
				waErrChs = append(waErrChs, ch)
			}
		}
	}
	var anErrCh, imErrCh, ctErrCh, tgErrCh <-chan error
	if !d.skipTelegram {
		tgErrCh = runTelegramSync(ctx, d.cfg, notifier)
	}
	if !d.skipAppleNotes {
		anErrCh = runAppleNotesSync(ctx, d.cfg, notifier)
	}
	if !d.skipIMessage {
		imErrCh = runIMessageSync(ctx, d.cfg, notifier)
	}
	if !d.skipContacts {
		ctErrCh = runContactsSync(ctx, d.cfg)
	}

	// Block until context is cancelled (signal received).
	<-ctx.Done()
	slog.Info("shutting down daemon")

	// Drain sync errors.
	for _, waErrCh := range waErrChs {
		if err := <-waErrCh; err != nil {
			slog.Error("whatsapp error during shutdown", "error", err)
		}
	}
	if tgErrCh != nil {
		if err := <-tgErrCh; err != nil {
			slog.Error("telegram error during shutdown", "error", err)
		}
	}
	if anErrCh != nil {
		if err := <-anErrCh; err != nil {
			slog.Error("applenotes error during shutdown", "error", err)
		}
	}
	if imErrCh != nil {
		if err := <-imErrCh; err != nil {
			slog.Error("imessage error during shutdown", "error", err)
		}
	}
	if ctErrCh != nil {
		if err := <-ctErrCh; err != nil {
			slog.Error("contacts error during shutdown", "error", err)
		}
	}

	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	if err := d.river.Stop(context.Background()); err != nil {
		slog.Error("river stop error", "error", err)
	}
	d.jobsDB.Close()

	slog.Info("daemon stopped")
	return nil
}

func (d *Daemon) openHooksDB() (*store.DB, error) {
	dsn := d.cfg.SchedulerDataDSN()
	db, err := store.Open(store.SQLiteConfig(dsn))
	if err != nil {
		return nil, err
	}
	if err := hooks.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

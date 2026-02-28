package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/riverqueue/river"

	"github.com/priyanshujain/openbotkit/config"
)

type Daemon struct {
	cfg    *config.Config
	mode   Mode
	river  *river.Client[*sql.Tx]
	jobsDB *sql.DB
}

func New(cfg *config.Config, mode Mode) *Daemon {
	return &Daemon{cfg: cfg, mode: mode}
}

func (d *Daemon) Run(ctx context.Context) error {
	if err := config.EnsureDir(); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

	log.Printf("starting daemon in %s mode", d.mode)

	client, db, err := newRiverClient(ctx, d.cfg, d.mode)
	if err != nil {
		return fmt.Errorf("init river: %w", err)
	}
	d.river = client
	d.jobsDB = db

	if err := d.river.Start(ctx); err != nil {
		d.jobsDB.Close()
		return fmt.Errorf("start river: %w", err)
	}
	log.Println("river job queue started")

	waErrCh := runWhatsAppSync(ctx, d.cfg)

	// Block until context is cancelled (signal received).
	<-ctx.Done()
	log.Println("shutting down daemon")

	// Drain WhatsApp errors.
	if err := <-waErrCh; err != nil {
		log.Printf("whatsapp error during shutdown: %v", err)
	}

	if err := d.river.Stop(context.Background()); err != nil {
		log.Printf("river stop error: %v", err)
	}
	d.jobsDB.Close()

	log.Println("daemon stopped")
	return nil
}

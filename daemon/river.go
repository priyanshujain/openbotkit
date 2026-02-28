package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/daemon/jobs"
)

func newRiverClient(ctx context.Context, cfg *config.Config, mode Mode) (*river.Client[*sql.Tx], *sql.DB, error) {
	dsn := cfg.JobsDBDSN()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open jobs db: %w", err)
	}
	db.SetMaxOpenConns(1)

	driver := riversqlite.New(db)

	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.GmailSyncWorker{Cfg: cfg})
	river.AddWorker(workers, &jobs.ReminderWorker{})

	riverCfg := &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	}

	if mode == ModeStandalone {
		period, err := time.ParseDuration(cfg.Daemon.GmailSyncPeriod)
		if err != nil {
			period = 15 * time.Minute
		}

		riverCfg.PeriodicJobs = []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(period),
				func() (river.JobArgs, *river.InsertOpts) {
					return jobs.GmailSyncArgs{}, nil
				},
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		}
	}

	client, err := river.NewClient(driver, riverCfg)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create river client: %w", err)
	}

	return client, db, nil
}

package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/priyanshujain/openbotkit/config"
)

func TestDaemon_RunAndShutdown(t *testing.T) {
	cfg := config.Default()

	tmpDir := t.TempDir()
	cfg.Daemon.JobsStorage.DSN = tmpDir + "/test-jobs.db"
	// Point WhatsApp session DB to tmp so it doesn't conflict.
	cfg.WhatsApp.Storage.DSN = tmpDir + "/wa-data.db"

	d := New(cfg, ModeStandalone)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give the daemon time to start.
	time.Sleep(500 * time.Millisecond)

	// Signal shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Daemon.Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not shut down within 10s")
	}
}

func TestDaemon_WorkerMode(t *testing.T) {
	cfg := config.Default()

	tmpDir := t.TempDir()
	cfg.Daemon.JobsStorage.DSN = tmpDir + "/test-jobs.db"
	cfg.WhatsApp.Storage.DSN = tmpDir + "/wa-data.db"

	d := New(cfg, ModeWorker)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Daemon.Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not shut down within 10s")
	}
}

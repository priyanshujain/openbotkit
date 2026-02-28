package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/priyanshujain/openbotkit/config"
)

func TestNewRiverClient_Standalone(t *testing.T) {
	cfg := config.Default()
	cfg.Daemon.GmailSyncPeriod = "1m"

	tmpDir := t.TempDir()
	cfg.Daemon.JobsStorage.DSN = tmpDir + "/test-jobs.db"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, db, err := newRiverClient(ctx, cfg, ModeStandalone)
	if err != nil {
		t.Fatalf("newRiverClient failed: %v", err)
	}
	defer db.Close()

	if client == nil {
		t.Fatal("client is nil")
	}

	// Start and stop to verify it's functional.
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start failed: %v", err)
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("client.Stop failed: %v", err)
	}
}

func TestNewRiverClient_Worker(t *testing.T) {
	cfg := config.Default()

	tmpDir := t.TempDir()
	cfg.Daemon.JobsStorage.DSN = tmpDir + "/test-jobs.db"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, db, err := newRiverClient(ctx, cfg, ModeWorker)
	if err != nil {
		t.Fatalf("newRiverClient failed: %v", err)
	}
	defer db.Close()

	if client == nil {
		t.Fatal("client is nil")
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start failed: %v", err)
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("client.Stop failed: %v", err)
	}
}

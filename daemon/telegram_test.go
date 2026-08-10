package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/73ai/openbotkit/config"
)

func TestRunTelegramSync_ShutdownOnCancel(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", tmpDir)

	cfg := config.Default()
	cfg.Telegram.Storage.DSN = filepath.Join(tmpDir, "tg-test.db")

	ctx, cancel := context.WithCancel(context.Background())

	// The source is not linked, so sync returns quickly.
	errCh := runTelegramSync(ctx, cfg, nil)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runTelegramSync returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("telegram sync did not stop within 5s")
	}
}

// A linked source with no session file must be skipped rather than reported as
// an error: the daemon restarts on every login and should not crash-loop.
func TestRunTelegramSync_LinkedWithoutSession(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", tmpDir)

	cfg := config.Default()
	cfg.Telegram.Storage.DSN = filepath.Join(tmpDir, "tg-test.db")

	if err := config.LinkSource("telegram"); err != nil {
		t.Fatalf("link source: %v", err)
	}
	if _, err := os.Stat(cfg.TelegramSessionPath()); !os.IsNotExist(err) {
		t.Fatalf("expected no session file, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := runTelegramSync(ctx, cfg, nil)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("missing session should be skipped, got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("telegram sync did not return within 5s")
	}
}

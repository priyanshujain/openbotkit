package daemon

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/remote"
)

func TestBridgeMode_RequiresDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("test only runs on non-darwin")
	}

	cfg := config.Default()
	client := remote.NewClient("http://localhost:8443", "user", "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := RunBridge(ctx, cfg, client)
	if err == nil {
		t.Fatal("expected error on non-darwin")
	}
}

func TestDaemonOption_SkipAppleNotes(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, WithSkipAppleNotes())
	if !d.skipAppleNotes {
		t.Fatal("expected skipAppleNotes to be true")
	}
}

func TestDaemonOption_SkipWhatsApp(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, WithSkipWhatsApp())
	if !d.skipWhatsApp {
		t.Fatal("expected skipWhatsApp to be true")
	}
}

func TestDaemonOption_SkipTelegram(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, WithSkipTelegram())
	if !d.skipTelegram {
		t.Fatal("expected skipTelegram to be true")
	}
}

func TestDaemonOption_SkipIMessage(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, WithSkipIMessage())
	if !d.skipIMessage {
		t.Fatal("expected skipIMessage to be true")
	}
}

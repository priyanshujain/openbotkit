package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/testutil"
	"github.com/73ai/openbotkit/provider/gemini"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/service/memory"
)

func TestSession_MessageAndHistorySaved(t *testing.T) {
	key := testutil.RequireGeminiKey(t)

	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)

	cfg := config.Default()

	p := gemini.New(key)
	model := "gemini-2.5-flash"

	ms := &mockSender{}
	ch := NewChannel(ms, "owner@s.whatsapp.net")

	sm := NewSessionManager(cfg, ch, p, "gemini", model)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sm.handleMessage(ctx, "What is 2 + 2? Reply with just the number.")

	ms.mu.Lock()
	sentCount := len(ms.sent)
	ms.mu.Unlock()

	if sentCount == 0 {
		t.Fatal("expected sender to send a response")
	}

	ms.mu.Lock()
	lastMsg := ms.sent[sentCount-1]
	ms.mu.Unlock()

	if !strings.Contains(lastMsg.text, "4") {
		t.Errorf("expected response to contain '4', got: %q", lastMsg.text)
	}

	sm.mu.Lock()
	sid := sm.sessionID
	sm.mu.Unlock()
	hs := historysrc.NewStore(config.HistoryDir())
	msgs, loadErr := hs.LoadSessionMessages(sid, 100)
	if loadErr != nil {
		t.Fatalf("load history messages: %v", loadErr)
	}
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 history messages (user + assistant), got %d", len(msgs))
	}
}

func TestSession_MemoryInjectedIntoPrompt(t *testing.T) {
	key := testutil.RequireGeminiKey(t)

	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)

	cfg := config.Default()

	memDir := config.UserMemoryDir()
	memory.EnsureDir(memDir)
	ms := memory.NewStore(memDir)
	ms.Add("User's name is WhatsAppTestBot99", memory.CategoryIdentity, "manual", "")

	p := gemini.New(key)
	model := "gemini-2.5-flash"

	sender := &mockSender{}
	ch := NewChannel(sender, "owner@s.whatsapp.net")
	sm := NewSessionManager(cfg, ch, p, "gemini", model)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sm.handleMessage(ctx, "What is my name? Reply with just the name.")

	sender.mu.Lock()
	sentCount := len(sender.sent)
	sender.mu.Unlock()

	if sentCount == 0 {
		t.Fatal("expected sender to send a response")
	}

	sender.mu.Lock()
	lastMsg := sender.sent[sentCount-1]
	sender.mu.Unlock()

	if !strings.Contains(lastMsg.text, "WhatsAppTestBot99") {
		t.Errorf("expected response to contain 'WhatsAppTestBot99' (from memory), got: %q", lastMsg.text)
	}
}

func TestSession_KillCommand(t *testing.T) {
	key := testutil.RequireGeminiKey(t)

	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)

	cfg := config.Default()

	p := gemini.New(key)
	model := "gemini-2.5-flash"

	sender := &mockSender{}
	ch := NewChannel(sender, "owner@s.whatsapp.net")
	sm := NewSessionManager(cfg, ch, p, "gemini", model)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start a long-running message in a goroutine.
	done := make(chan struct{})
	go func() {
		sm.handleMessage(ctx, "Count from 1 to 1000, one number per line.")
		close(done)
	}()

	// Give the agent time to start.
	time.Sleep(2 * time.Second)

	// Send "kill" to trigger the kill path.
	sm.handleMessage(ctx, "kill")

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("agent did not stop within 30s after kill")
	}

	// Verify "Stopped." was sent.
	sender.mu.Lock()
	defer sender.mu.Unlock()
	found := false
	for _, msg := range sender.sent {
		if strings.Contains(msg.text, "Stopped") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Stopped.' message after kill")
	}
}

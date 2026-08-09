package telegram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/testutil"
	"github.com/73ai/openbotkit/provider/gemini"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/service/memory"
)

// TestSession_MessageAndHistorySaved verifies the full session lifecycle:
// message → agent with real Gemini API → response → history saved to DB.
func TestSession_MessageAndHistorySaved(t *testing.T) {
	key := testutil.RequireGeminiKey(t)

	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	// Create source dirs
	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)

	cfg := config.Default()

	// Create real Gemini provider
	p := gemini.New(key)
	model := "gemini-2.5-flash"

	bot := &mockBot{}
	ch := NewChannel(bot, 123)

	sm := NewSessionManager(cfg, ch, p, "gemini", model)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Send a simple message through the session manager
	sm.handleMessage(ctx, "What is 2 + 2? Reply with just the number.", 1)

	// Verify the bot sent a response
	bot.mu.Lock()
	sentCount := len(bot.sent)
	bot.mu.Unlock()

	if sentCount == 0 {
		t.Fatal("expected bot to send a response")
	}

	// Check the response contains "4"
	bot.mu.Lock()
	lastMsg := bot.sent[sentCount-1]
	bot.mu.Unlock()

	msg, ok := lastMsg.(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", lastMsg)
	}
	if !strings.Contains(msg.Text, "4") {
		t.Errorf("expected response to contain '4', got: %q", msg.Text)
	}

	// Verify history was saved.
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

// TestIntegration_SessionWithMemoryInjection tests that user memories are injected
// into the system prompt when the agent processes a message.
// TestSession_MemoryInjectedIntoPrompt verifies memories from the DB appear
// in the system prompt and the agent can reference them.
func TestSession_MemoryInjectedIntoPrompt(t *testing.T) {
	key := testutil.RequireGeminiKey(t)

	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)

	cfg := config.Default()

	// Seed memory store
	memDir := config.UserMemoryDir()
	memory.EnsureDir(memDir)
	ms := memory.NewStore(memDir)
	ms.Add("User's name is TestBot42", memory.CategoryIdentity, "manual", "")

	p := gemini.New(key)
	model := "gemini-2.5-flash"

	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := NewSessionManager(cfg, ch, p, "gemini", model)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Ask the agent something that requires memory
	sm.handleMessage(ctx, "What is my name? Reply with just the name.", 1)

	bot.mu.Lock()
	sentCount := len(bot.sent)
	bot.mu.Unlock()

	if sentCount == 0 {
		t.Fatal("expected bot to send a response")
	}

	bot.mu.Lock()
	lastMsg := bot.sent[sentCount-1]
	bot.mu.Unlock()

	msg, ok := lastMsg.(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", lastMsg)
	}

	if !strings.Contains(msg.Text, "TestBot42") {
		t.Errorf("expected response to contain 'TestBot42' (from memory), got: %q", msg.Text)
	}
}

// TestIntegration_SessionWithToolUse tests that the agent can use tools
// (like bash) when processing a Telegram message.
// TestSession_ToolUseViaBash verifies the agent can execute bash commands
// through the tool use loop within a Telegram session.
func TestSession_ToolUseViaBash(t *testing.T) {
	key := testutil.RequireGeminiKey(t)

	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)

	cfg := config.Default()

	p := gemini.New(key)
	model := "gemini-2.5-flash"

	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := NewSessionManager(cfg, ch, p, "gemini", model)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Ask something that requires tool use (bash)
	sm.handleMessage(ctx, "Run the command 'echo telegram-integration-ok' and tell me the output.", 1)

	bot.mu.Lock()
	sentCount := len(bot.sent)
	bot.mu.Unlock()

	if sentCount == 0 {
		t.Fatal("expected bot to send a response")
	}

	bot.mu.Lock()
	lastMsg := bot.sent[sentCount-1]
	bot.mu.Unlock()

	msg, ok := lastMsg.(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", lastMsg)
	}

	if !strings.Contains(msg.Text, "telegram-integration-ok") {
		t.Errorf("expected response to contain 'telegram-integration-ok', got: %q", msg.Text)
	}
}

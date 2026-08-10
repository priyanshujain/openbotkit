package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/73ai/openbotkit/agent/tools"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/service/memory"
)

type stubProvider struct{}

func (s *stubProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{
		Content:    []provider.ContentBlock{{Type: provider.ContentText, Text: "stub response"}},
		StopReason: provider.StopEndTurn,
	}, nil
}

func (s *stubProvider) StreamChat(_ context.Context, _ provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

func setupTestEnv(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)
	historysrc.EnsureDir(filepath.Join(dir, "history"))
	os.MkdirAll(filepath.Join(dir, "user_memory"), 0700)
	return config.Default()
}

func seedHistory(t *testing.T, sessionID string, msgs []historysrc.Message) {
	t.Helper()
	dir := config.HistoryDir()
	s := historysrc.NewStore(dir)
	if err := s.UpsertConversation(sessionID, "telegram"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	for _, m := range msgs {
		if err := s.SaveMessage(sessionID, m.Role, m.Content); err != nil {
			t.Fatalf("save message: %v", err)
		}
	}
}

func TestRestoreSession_RecentSession(t *testing.T) {
	cfg := setupTestEnv(t)
	seedHistory(t, "tg-abc", []historysrc.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you"},
		{Role: "assistant", Content: "fine"},
	})

	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.touchSession()

	if sm.sessionID != "tg-abc" {
		t.Fatalf("expected restored session tg-abc, got %q", sm.sessionID)
	}
	if len(sm.history) != 4 {
		t.Fatalf("expected 4 history messages, got %d", len(sm.history))
	}
	if sm.history[0].Content[0].Text != "hello" {
		t.Errorf("history[0] = %q, want 'hello'", sm.history[0].Content[0].Text)
	}
	if len(sm.messages) != 2 {
		t.Fatalf("expected 2 user messages, got %d", len(sm.messages))
	}
}

func TestRestoreSession_ExpiredSession(t *testing.T) {
	cfg := setupTestEnv(t)

	// Write an old session entry directly with a timestamp 1 hour ago.
	dir := config.HistoryDir()
	oldTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	line := fmt.Sprintf(`{"session_id":"tg-old","cwd":"telegram","started_at":"%s","updated_at":"%s","ended":false}`, oldTime, oldTime)
	os.WriteFile(filepath.Join(dir, "sessions.jsonl"), []byte(line+"\n"), 0600)
	msgLine := fmt.Sprintf(`{"role":"user","content":"old msg","timestamp":"%s"}`, oldTime)
	os.WriteFile(filepath.Join(dir, "sessions", "tg-old.jsonl"), []byte(msgLine+"\n"), 0600)

	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.touchSession()

	if sm.sessionID == "tg-old" {
		t.Fatal("should not restore expired session")
	}
	if sm.sessionID == "" {
		t.Fatal("should have generated new session ID")
	}
	if len(sm.history) != 0 {
		t.Fatalf("expected empty history, got %d", len(sm.history))
	}
}

func TestRestoreSession_EmptyDB(t *testing.T) {
	cfg := setupTestEnv(t)

	// No history seeded — just an empty directory.

	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.touchSession()

	if sm.sessionID == "" {
		t.Fatal("should have generated new session ID")
	}
	if len(sm.history) != 0 {
		t.Fatalf("expected empty history, got %d", len(sm.history))
	}
}

func sentTexts(bot *mockBot) []string {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	var texts []string
	for _, c := range bot.sent {
		if msg, ok := c.(tgbotapi.MessageConfig); ok {
			texts = append(texts, msg.Text)
		}
	}
	return texts
}

func TestStartCommand_ResetsSession(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.sessionID = "tg-abc"
	sm.history = []provider.Message{
		provider.NewTextMessage(provider.RoleUser, "hello"),
		provider.NewTextMessage(provider.RoleAssistant, "hi"),
	}
	sm.messages = []string{"hello"}

	sm.handleMessage(context.Background(), "/start", 0)

	if sm.sessionID != "" {
		t.Fatalf("expected cleared sessionID, got %q", sm.sessionID)
	}
	if sm.history != nil {
		t.Fatalf("expected nil history, got %d messages", len(sm.history))
	}

	texts := sentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Session reset") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected reset confirmation message, got: %v", texts)
	}
}

func TestStartCommand_WithSuffix(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}
	sm.sessionID = "tg-xyz"

	sm.handleMessage(context.Background(), "/start now", 0)

	if sm.sessionID != "" {
		t.Fatalf("expected cleared sessionID, got %q", sm.sessionID)
	}
	texts := sentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Session reset") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected reset confirmation, got: %v", texts)
	}
}

func TestNormalMessage_DoesNotReset(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
	}
	sm.sessionID = "tg-existing"
	sm.history = []provider.Message{
		provider.NewTextMessage(provider.RoleUser, "prior"),
	}
	sm.messages = []string{"prior"}

	sm.handleMessage(context.Background(), "hello", 0)

	// Session was NOT reset — it should still be "tg-existing"
	if sm.sessionID != "tg-existing" {
		t.Fatalf("session should not be reset, got %q", sm.sessionID)
	}
	texts := sentTexts(bot)
	for _, txt := range texts {
		if strings.Contains(txt, "Session reset") {
			t.Fatal("normal message should not trigger session reset")
		}
	}
	// Should have received a response from the stub provider
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "stub response") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stub response, got: %v", texts)
	}
}

func TestResolveContextWindow_FromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Models = &config.ModelsConfig{ContextWindow: 150000}
	sm := &SessionManager{cfg: cfg, model: "claude-opus-4-6"}

	if got := sm.resolveContextWindow(); got != 150000 {
		t.Fatalf("expected 150000 from config, got %d", got)
	}
}

func TestResolveContextWindow_FromModelLookup(t *testing.T) {
	cfg := config.Default()
	sm := &SessionManager{cfg: cfg, model: "gemini-2.5-flash"}

	if got := sm.resolveContextWindow(); got != 1048576 {
		t.Fatalf("expected 1048576 from model lookup, got %d", got)
	}
}

func TestResolveContextWindow_Fallback(t *testing.T) {
	cfg := config.Default()
	sm := &SessionManager{cfg: cfg, model: "unknown-model"}

	if got := sm.resolveContextWindow(); got != 200000 {
		t.Fatalf("expected 200000 fallback, got %d", got)
	}
}

func TestResolveCompactionThreshold_FromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Models = &config.ModelsConfig{CompactionThreshold: 0.25}
	sm := &SessionManager{cfg: cfg}

	if got := sm.resolveCompactionThreshold(); got != 0.25 {
		t.Fatalf("expected 0.25 from config, got %f", got)
	}
}

func TestResolveCompactionThreshold_Default(t *testing.T) {
	cfg := config.Default()
	sm := &SessionManager{cfg: cfg}

	if got := sm.resolveCompactionThreshold(); got != 0.30 {
		t.Fatalf("expected 0.30 default, got %f", got)
	}
}

// --- endSession tests ---

func TestEndSession_ClearsState(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.sessionID = "tg-end"
	sm.history = []provider.Message{
		provider.NewTextMessage(provider.RoleUser, "hello"),
	}
	sm.messages = []string{"hello"}
	sm.timer = time.AfterFunc(time.Hour, func() {})

	sm.endSession()

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.sessionID != "" {
		t.Fatalf("expected empty sessionID, got %q", sm.sessionID)
	}
	if sm.history != nil {
		t.Fatalf("expected nil history, got %d", len(sm.history))
	}
	if sm.messages != nil {
		t.Fatalf("expected nil messages, got %d", len(sm.messages))
	}
	if sm.timer != nil {
		t.Fatal("expected nil timer")
	}
}

func TestEndSession_NoopWhenEmpty(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	// Should not panic with empty state
	sm.endSession()

	if sm.sessionID != "" {
		t.Fatalf("expected empty sessionID, got %q", sm.sessionID)
	}
}

func TestEndSession_DoubleCallSafe(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}
	sm.sessionID = "tg-double"
	sm.messages = []string{"hi"}

	sm.endSession()
	sm.endSession() // second call should be a no-op

	if sm.sessionID != "" {
		t.Fatalf("expected empty sessionID after double end, got %q", sm.sessionID)
	}
}

// --- saveHistory tests ---

func TestSaveHistory_PersistsMessages(t *testing.T) {
	cfg := setupTestEnv(t)

	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.saveHistory("tg-save-test", "hello user", "hi assistant")

	// Verify messages were saved.
	s := historysrc.NewStore(config.HistoryDir())
	msgs, err := s.LoadSessionMessages("tg-save-test", 100)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello user" {
		t.Errorf("msg[0] = %q/%q", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi assistant" {
		t.Errorf("msg[1] = %q/%q", msgs[1].Role, msgs[1].Content)
	}
}

func TestSaveHistory_MultipleCallsSameSession(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.saveHistory("tg-multi", "msg1", "resp1")
	sm.saveHistory("tg-multi", "msg2", "resp2")

	s := historysrc.NewStore(config.HistoryDir())
	msgs, err := s.LoadSessionMessages("tg-multi", 100)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
}

// --- touchSession timer tests ---

func TestTouchSession_CreatesTimer(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.touchSession()

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.timer == nil {
		t.Fatal("expected timer to be set")
	}
	if sm.sessionID == "" {
		t.Fatal("expected sessionID to be set")
	}
}

func TestTouchSession_ResetsTimer(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch}

	sm.touchSession()
	sm.mu.Lock()
	firstID := sm.sessionID
	firstTimer := sm.timer
	sm.mu.Unlock()

	// Second touch should keep same session but reset timer
	sm.touchSession()
	sm.mu.Lock()
	secondID := sm.sessionID
	secondTimer := sm.timer
	sm.mu.Unlock()

	if firstID != secondID {
		t.Fatalf("sessionID changed: %q → %q", firstID, secondID)
	}
	if secondTimer == firstTimer {
		t.Fatal("timer should have been replaced")
	}
}

// --- handleMessage full path tests ---

func TestHandleMessage_UpdatesHistoryAndMessages(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
	}

	sm.handleMessage(context.Background(), "hello world", 0)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.messages) != 1 || sm.messages[0] != "hello world" {
		t.Fatalf("messages = %v, want [hello world]", sm.messages)
	}
	if len(sm.history) != 2 {
		t.Fatalf("history len = %d, want 2 (user + assistant)", len(sm.history))
	}
	if sm.history[0].Role != provider.RoleUser {
		t.Errorf("history[0].Role = %q, want user", sm.history[0].Role)
	}
	if sm.history[0].Content[0].Text != "hello world" {
		t.Errorf("history[0].Text = %q", sm.history[0].Content[0].Text)
	}
	if sm.history[1].Role != provider.RoleAssistant {
		t.Errorf("history[1].Role = %q, want assistant", sm.history[1].Role)
	}
	if sm.history[1].Content[0].Text != "stub response" {
		t.Errorf("history[1].Text = %q", sm.history[1].Content[0].Text)
	}
}

func TestHandleMessage_SavesHistoryToDB(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
	}

	sm.handleMessage(context.Background(), "test input", 0)

	sm.mu.Lock()
	sid := sm.sessionID
	sm.mu.Unlock()

	// Verify the history store was written to.
	s := historysrc.NewStore(config.HistoryDir())
	msgs, err := s.LoadSessionMessages(sid, 100)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in store, got %d", len(msgs))
	}
	if msgs[0].Content != "test input" {
		t.Errorf("user msg = %q", msgs[0].Content)
	}
	if msgs[1].Content != "stub response" {
		t.Errorf("assistant msg = %q", msgs[1].Content)
	}
}

func TestHandleMessage_MultiTurnAccumulates(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
	}

	sm.handleMessage(context.Background(), "first", 0)
	sm.handleMessage(context.Background(), "second", 0)
	sm.handleMessage(context.Background(), "third", 0)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(sm.messages))
	}
	// 3 turns × 2 messages (user+assistant) = 6
	if len(sm.history) != 6 {
		t.Fatalf("history len = %d, want 6", len(sm.history))
	}
}

// --- userMemoriesPrompt tests ---

func TestUserMemoriesPrompt_Empty(t *testing.T) {
	cfg := setupTestEnv(t)

	sm := &SessionManager{cfg: cfg}
	prompt := sm.userMemoriesPrompt()
	if prompt != "" {
		t.Fatalf("expected empty prompt, got %q", prompt)
	}
}

func TestUserMemoriesPrompt_WithMemories(t *testing.T) {
	cfg := setupTestEnv(t)

	dir := config.UserMemoryDir()
	memory.EnsureDir(dir)
	ms := memory.NewStore(dir)
	ms.Add("User prefers Go over Python", memory.CategoryPreference, "test", "")

	sm := &SessionManager{cfg: cfg}
	prompt := sm.userMemoriesPrompt()
	if !strings.Contains(prompt, "User prefers Go over Python") {
		t.Fatalf("expected memory in prompt, got %q", prompt)
	}
}

// --- newAgent wiring tests ---

func TestNewAgent_CreatesAgentWithOptions(t *testing.T) {
	cfg := setupTestEnv(t)

	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      NewChannel(&mockBot{}, 123),
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
		taskTracker:  nil,
	}
	// taskTracker is required by newAgent's tool registration
	sm.taskTracker = newTaskTracker()

	a, recorder, auditLogger, err := sm.newAgent(nil, nil)
	if err != nil {
		t.Fatalf("newAgent: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil agent")
	}
	if recorder != nil {
		defer recorder.Close()
	}
	if auditLogger != nil {
		defer auditLogger.Close()
	}
}

func TestNewAgent_WithHistory(t *testing.T) {
	cfg := setupTestEnv(t)

	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      NewChannel(&mockBot{}, 123),
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
		taskTracker:  newTaskTracker(),
	}

	history := []provider.Message{
		provider.NewTextMessage(provider.RoleUser, "prior msg"),
		provider.NewTextMessage(provider.RoleAssistant, "prior resp"),
	}

	a, recorder, auditLogger, err := sm.newAgent(history, nil)
	if err != nil {
		t.Fatalf("newAgent: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil agent")
	}
	if recorder != nil {
		defer recorder.Close()
	}
	if auditLogger != nil {
		defer auditLogger.Close()
	}
}

// --- Run loop tests ---

func TestRun_ExitsOnChannelClose(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
		taskTracker:  newTaskTracker(),
	}

	done := make(chan struct{})
	go func() {
		sm.Run(context.Background())
		close(done)
	}()

	// Close the channel to trigger EOF
	ch.Close()

	select {
	case <-done:
		// Run exited cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not exit after channel close")
	}
}

func TestRun_ProcessesMessages(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sp := &stubProvider{}
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     sp,
		model:        "test-model",
		fastProvider: sp,
		fastModel:    "test-model",
		nanoProvider: sp,
		nanoModel:    "test-model",
		taskTracker:  newTaskTracker(),
	}

	done := make(chan struct{})
	go func() {
		sm.Run(context.Background())
		close(done)
	}()

	ch.PushMessage("hello from run test", 0)

	// Wait briefly for the message to be processed
	time.Sleep(500 * time.Millisecond)

	// Close channel to end the Run loop
	ch.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not exit")
	}

	texts := sentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "stub response") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stub response from Run loop, got: %v", texts)
	}
}

// newTaskTracker creates a task tracker for tests.
func newTaskTracker() *tools.TaskTracker {
	return tools.NewTaskTracker()
}

// blockingProvider blocks on Chat() until released or context is cancelled.
type blockingProvider struct {
	called   chan struct{}
	released chan struct{}
	response *provider.ChatResponse
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{
		called:   make(chan struct{}),
		released: make(chan struct{}),
		response: &provider.ChatResponse{
			Content:    []provider.ContentBlock{{Type: provider.ContentText, Text: "blocking response"}},
			StopReason: provider.StopEndTurn,
		},
	}
}

func (p *blockingProvider) Chat(ctx context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	close(p.called)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.released:
		return p.response, nil
	}
}

func (p *blockingProvider) StreamChat(_ context.Context, _ provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestSessionManager_IsAgentRunning(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	bp := newBlockingProvider()
	sm := &SessionManager{
		cfg: cfg, channel: ch, provider: bp, model: "test",
		fastProvider: bp, fastModel: "test",
		nanoProvider: bp, nanoModel: "test",
		taskTracker: newTaskTracker(),
	}

	if sm.IsAgentRunning() {
		t.Fatal("should not be running initially")
	}

	done := make(chan struct{})
	go func() {
		sm.handleMessage(context.Background(), "test", 0)
		close(done)
	}()

	<-bp.called
	if !sm.IsAgentRunning() {
		t.Fatal("should be running during agent execution")
	}

	close(bp.released)
	<-done
	if sm.IsAgentRunning() {
		t.Fatal("should not be running after agent completes")
	}
}

func TestSessionManager_Kill(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	bp := newBlockingProvider()
	sm := &SessionManager{
		cfg: cfg, channel: ch, provider: bp, model: "test",
		fastProvider: bp, fastModel: "test",
		nanoProvider: bp, nanoModel: "test",
		taskTracker: newTaskTracker(),
	}

	done := make(chan struct{})
	go func() {
		sm.handleMessage(context.Background(), "test input", 0)
		close(done)
	}()

	<-bp.called
	if !sm.Kill() {
		t.Fatal("Kill should return true when agent is running")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleMessage should have returned after kill")
	}

	texts := sentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Stopped.") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Stopped.' message, got: %v", texts)
	}
}

func TestSessionManager_KillNotRunning(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	sm := &SessionManager{cfg: cfg, channel: ch, taskTracker: newTaskTracker()}

	if sm.Kill() {
		t.Fatal("Kill should return false when no agent is running")
	}
}

func TestSessionManager_HandleMessageKilled(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	bp := newBlockingProvider()
	sm := &SessionManager{
		cfg: cfg, channel: ch, provider: bp, model: "test",
		fastProvider: bp, fastModel: "test",
		nanoProvider: bp, nanoModel: "test",
		taskTracker: newTaskTracker(),
	}

	done := make(chan struct{})
	go func() {
		sm.handleMessage(context.Background(), "hello", 0)
		close(done)
	}()

	<-bp.called
	sm.Kill()
	<-done

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(sm.history))
	}
	if sm.history[1].Content[0].Text != "(interrupted)" {
		t.Errorf("history[1].Text = %q, want (interrupted)", sm.history[1].Content[0].Text)
	}
}

func TestSessionManager_KillDuringApproval(t *testing.T) {
	bot := &mockBot{notify: make(chan struct{}, 1)}
	ch := NewChannel(bot, 123)

	approvalDone := make(chan bool, 1)
	go func() {
		approved, _ := ch.RequestApproval("risky action")
		approvalDone <- approved
	}()

	select {
	case <-bot.notify:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for approval message")
	}

	ch.CancelPendingApproval()

	select {
	case approved := <-approvalDone:
		if approved {
			t.Fatal("expected approval to be denied after cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for approval result")
	}
}

func TestSessionManager_RunningDelegateTasks(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	tracker := newTaskTracker()
	tracker.Start("t1", "research Go", tools.AgentClaude)
	tracker.Start("t2", "write docs", tools.AgentGemini)
	tracker.Complete("t2", "done")

	sm := &SessionManager{cfg: cfg, channel: ch, taskTracker: tracker}

	tasks := sm.RunningDelegateTasks()
	if len(tasks) != 1 {
		t.Fatalf("got %d running tasks, want 1", len(tasks))
	}
	if tasks[0].ID != "t1" || tasks[0].Task != "research Go" {
		t.Errorf("task = %+v", tasks[0])
	}
}

func TestSessionManager_KillDelegateTask(t *testing.T) {
	cfg := setupTestEnv(t)
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	tracker := newTaskTracker()
	tracker.Start("t1", "research", tools.AgentClaude)

	ctx, cancel := context.WithCancel(context.Background())
	tracker.RegisterCancel("t1", cancel)

	sm := &SessionManager{cfg: cfg, channel: ch, taskTracker: tracker}

	if !sm.KillDelegateTask("t1") {
		t.Fatal("expected true")
	}
	if ctx.Err() != context.Canceled {
		t.Error("context should be cancelled")
	}
	if sm.KillDelegateTask("nonexistent") {
		t.Fatal("expected false for nonexistent task")
	}
}

func TestAuthRedirectURL_WithCallback(t *testing.T) {
	cfg := config.Default()
	cfg.Integrations = &config.IntegrationsConfig{
		GWS: &config.GWSConfig{
			CallbackURL: "https://example.ngrok-free.app/auth/google/callback",
		},
	}
	sm := &SessionManager{cfg: cfg}
	got := sm.authRedirectURL()
	want := "https://example.ngrok-free.app/auth/redirect"
	if got != want {
		t.Fatalf("authRedirectURL() = %q, want %q", got, want)
	}
}

func TestAuthRedirectURL_WithoutCallback(t *testing.T) {
	cfg := config.Default()
	sm := &SessionManager{cfg: cfg}
	got := sm.authRedirectURL()
	if got != "" {
		t.Fatalf("authRedirectURL() = %q, want empty", got)
	}
}

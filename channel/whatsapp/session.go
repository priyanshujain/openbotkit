package whatsapp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/73ai/openbotkit/agent"
	"github.com/73ai/openbotkit/agent/audit"
	"github.com/73ai/openbotkit/agent/tools"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/service/learnings"
	"github.com/73ai/openbotkit/service/memory"
	"github.com/73ai/openbotkit/oauth/google"
	"github.com/73ai/openbotkit/provider"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/service/scheduler"
	slacksrc "github.com/73ai/openbotkit/source/slack"
	usagesrc "github.com/73ai/openbotkit/service/usage"
)

const sessionTimeout = 15 * time.Minute

type SessionManager struct {
	cfg          *config.Config
	channel      *Channel
	provider     provider.Provider
	providerName string
	model        string

	interactor  tools.Interactor
	scopeWaiter *google.ScopeWaiter
	tokenBridge *tools.TokenBridge
	googleAuth  *google.Google
	account     string
	manifest    *skills.Manifest

	taskTracker *tools.TaskTracker

	webSearch    tools.WebSearcher
	fastProvider provider.Provider
	fastModel    string
	nanoProvider provider.Provider
	nanoModel    string

	accountLabel string
	ownerJID     string

	mu           sync.Mutex
	sessionID    string
	timer        *time.Timer
	messages     []string
	history      []provider.Message
	agentRunning bool
	runCancel    context.CancelFunc
}

type SessionManagerDeps struct {
	Interactor   tools.Interactor
	ScopeWaiter  *google.ScopeWaiter
	TokenBridge  *tools.TokenBridge
	GoogleAuth   *google.Google
	Account      string
	AccountLabel string
	OwnerJID     string
}

func NewSessionManager(cfg *config.Config, ch *Channel, p provider.Provider, providerName, model string, deps ...SessionManagerDeps) *SessionManager {
	sm := &SessionManager{
		cfg:          cfg,
		channel:      ch,
		provider:     p,
		providerName: providerName,
		model:        model,
		taskTracker:  openTaskTracker(cfg),
	}
	if len(deps) > 0 {
		d := deps[0]
		sm.interactor = d.Interactor
		sm.scopeWaiter = d.ScopeWaiter
		sm.tokenBridge = d.TokenBridge
		sm.googleAuth = d.GoogleAuth
		sm.account = d.Account
		sm.accountLabel = d.AccountLabel
		sm.ownerJID = d.OwnerJID
	}
	if sm.gwsEnabled() {
		sm.manifest, _ = skills.LoadManifest()
	}
	sm.initWebSearch()
	return sm
}

func (sm *SessionManager) Run(ctx context.Context) {
	for {
		text, err := sm.channel.Receive()
		if err == io.EOF {
			sm.endSession()
			return
		}
		if err != nil {
			slog.Error("whatsapp session: receive error", "error", err)
			continue
		}
		sm.handleMessage(ctx, text)
	}
}

func (sm *SessionManager) handleMessage(ctx context.Context, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "stop" || lower == "kill" {
		if sm.Kill() {
			return
		}
		sm.channel.Send("Nothing running to stop.")
		return
	}

	sm.touchSession()

	sm.mu.Lock()
	priorHistory := make([]provider.Message, len(sm.history))
	copy(priorHistory, sm.history)
	sm.mu.Unlock()

	a, recorder, auditLogger, err := sm.newAgent(priorHistory, nil)
	if err != nil {
		slog.Error("whatsapp session: create agent", "error", err)
		sm.channel.Send(fmt.Sprintf("Error: %v", err))
		return
	}
	if recorder != nil {
		defer recorder.Close()
	}
	if auditLogger != nil {
		defer auditLogger.Close()
	}

	agentCtx, cancel := context.WithCancel(ctx)
	sm.mu.Lock()
	sm.agentRunning = true
	sm.runCancel = cancel
	sm.mu.Unlock()

	response, err := a.Run(agentCtx, text)

	killed := agentCtx.Err() != nil && ctx.Err() == nil

	sm.mu.Lock()
	sm.agentRunning = false
	sm.runCancel = nil
	sm.mu.Unlock()
	cancel()

	if killed {
		sm.mu.Lock()
		sid := sm.sessionID
		sm.messages = append(sm.messages, text)
		sm.history = append(sm.history,
			provider.NewTextMessage(provider.RoleUser, text),
			provider.NewTextMessage(provider.RoleAssistant, "(interrupted)"),
		)
		sm.mu.Unlock()
		sm.saveHistory(sid, text, "(interrupted)")
		sm.channel.Send("Stopped.")
		return
	}

	if err != nil {
		slog.Error("whatsapp session: agent error", "error", err)
		sm.channel.Send(fmt.Sprintf("Error: %v", err))
		return
	}

	if strings.TrimSpace(response) == "" {
		slog.Warn("whatsapp session: empty response from agent")
		response = "I wasn't able to generate a response. Could you try again?"
	}

	if err := sm.channel.Send(response); err != nil {
		slog.Error("whatsapp session: send response", "error", err)
	}

	sm.mu.Lock()
	sid := sm.sessionID
	sm.messages = append(sm.messages, text)
	sm.history = append(sm.history,
		provider.NewTextMessage(provider.RoleUser, text),
		provider.NewTextMessage(provider.RoleAssistant, response),
	)
	sm.mu.Unlock()

	sm.saveHistory(sid, text, response)
}

func (sm *SessionManager) Kill() bool {
	sm.mu.Lock()
	cancel := sm.runCancel
	running := sm.agentRunning
	sm.mu.Unlock()
	if !running || cancel == nil {
		return false
	}
	sm.channel.CancelPendingApproval()
	cancel()
	return true
}

func (sm *SessionManager) restoreSession() bool {
	histStore := sm.openHistoryStore()

	recent, err := histStore.LoadRecentSession("whatsapp", sessionTimeout)
	if err != nil {
		slog.Warn("whatsapp session: load recent session", "error", err)
		return false
	}
	if recent == nil {
		return false
	}

	msgs, err := histStore.LoadSessionMessages(recent.SessionID, 100)
	if err != nil {
		slog.Warn("whatsapp session: load session messages", "error", err)
		return false
	}
	if len(msgs) == 0 {
		return false
	}

	sm.sessionID = recent.SessionID
	sm.messages = nil
	sm.history = nil
	for _, m := range msgs {
		role := provider.RoleUser
		if m.Role == "assistant" {
			role = provider.RoleAssistant
		}
		sm.history = append(sm.history, provider.NewTextMessage(role, m.Content))
		if m.Role == "user" {
			sm.messages = append(sm.messages, m.Content)
		}
	}
	return true
}

func (sm *SessionManager) touchSession() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.sessionID == "" {
		if !sm.restoreSession() {
			sm.sessionID = generateSessionID()
			sm.messages = nil
		}
		if err := config.EnsureScratchDir(sm.sessionID); err != nil {
			slog.Warn("scratch dir creation failed", "error", err)
		}
	}

	if sm.timer != nil {
		sm.timer.Stop()
	}
	sm.timer = time.AfterFunc(sessionTimeout, func() {
		sm.endSession()
	})
}

func (sm *SessionManager) endSession() {
	sm.mu.Lock()
	if sm.sessionID == "" {
		sm.mu.Unlock()
		return
	}
	if sm.timer != nil {
		sm.timer.Stop()
		sm.timer = nil
	}
	messages := sm.messages
	sid := sm.sessionID
	sm.sessionID = ""
	sm.messages = nil
	sm.history = nil
	sm.mu.Unlock()

	config.CleanScratch(sid)

	histStore := sm.openHistoryStore()
	histStore.EndSession(sid)

	if len(messages) > 0 {
		go sm.extractMemories(context.Background(), messages)
	}
}

func (sm *SessionManager) extractMemories(ctx context.Context, messages []string) {
	if sm.cfg.Models == nil || sm.cfg.Models.Default == "" {
		return
	}

	dir := config.UserMemoryDir()
	if err := memory.EnsureDir(dir); err != nil {
		slog.Error("whatsapp session: ensure memory dir", "error", err)
		return
	}
	ms := memory.NewStore(dir)

	extractLLM, reconcileLLM, err := sm.buildMemoryLLMs()
	if err != nil {
		slog.Error("whatsapp session: build LLM for extraction", "error", err)
		return
	}

	candidates, err := memory.Extract(ctx, extractLLM, messages)
	if err != nil {
		slog.Error("whatsapp session: extract memories", "error", err)
		return
	}

	if len(candidates) == 0 {
		return
	}

	result, err := memory.Reconcile(ctx, ms, reconcileLLM, candidates)
	if err != nil {
		slog.Error("whatsapp session: reconcile memories", "error", err)
		return
	}

	slog.Info("whatsapp session: memory extraction complete",
		"added", result.Added, "updated", result.Updated,
		"deleted", result.Deleted, "skipped", result.Skipped)
}

func (sm *SessionManager) buildMemoryLLMs() (extract memory.LLM, reconcile memory.LLM, err error) {
	registry, err := provider.NewRegistry(sm.cfg.Models)
	if err != nil {
		return nil, nil, fmt.Errorf("create provider registry: %w", err)
	}
	router := provider.NewRouter(registry, sm.cfg.Models)
	extractLLM := &memory.RouterLLM{Router: router, Tier: provider.TierFast}
	reconcileLLM := &memory.RouterLLM{Router: router, Tier: provider.TierNano}
	return extractLLM, reconcileLLM, nil
}

func (sm *SessionManager) resolveContextWindow() int {
	if sm.cfg.Models != nil && sm.cfg.Models.ContextWindow > 0 {
		return sm.cfg.Models.ContextWindow
	}
	if w := provider.DefaultContextWindow(sm.model); w > 0 {
		return w
	}
	return 200000
}

func (sm *SessionManager) resolveCompactionThreshold() float64 {
	if sm.cfg.Models != nil && sm.cfg.Models.CompactionThreshold > 0 {
		return sm.cfg.Models.CompactionThreshold
	}
	return 0.30
}

func (sm *SessionManager) authRedirectURL() string {
	cbURL := sm.cfg.GWSCallbackURL()
	if cbURL == "" {
		return ""
	}
	return strings.TrimSuffix(cbURL, "/auth/google/callback") + "/auth/redirect"
}

func (sm *SessionManager) gwsEnabled() bool {
	return sm.cfg.Integrations != nil && sm.cfg.Integrations.GWS != nil && sm.cfg.Integrations.GWS.Enabled
}

func (sm *SessionManager) newAgent(history []provider.Message, onToolStart func(string)) (*agent.Agent, *usagesrc.Recorder, *audit.Logger, error) {
	toolReg := tools.NewStandardRegistry(sm.interactor, nil)

	sm.mu.Lock()
	sid := sm.sessionID
	sm.mu.Unlock()
	if sid != "" {
		toolReg.SetScratchDir(config.ScratchDir(sid))
	}

	al := sm.openAuditLogger()
	if al != nil {
		toolReg.SetAudit(al, "whatsapp")
	}

	if sm.gwsEnabled() && sm.interactor != nil {
		toolReg.Register(tools.NewGWSExecuteTool(tools.GWSToolConfig{
			Interactor:      sm.interactor,
			ScopeChecker:    &tools.GoogleScopeChecker{TokenDBPath: sm.cfg.GoogleTokenDBPath()},
			Bridge:          sm.tokenBridge,
			ScopeWaiter:     sm.scopeWaiter,
			Google:          sm.googleAuth,
			Account:         sm.account,
			Manifest:        sm.manifest,
			Runner:          tools.NewGWSRunner(),
			AuthRedirectURL: sm.authRedirectURL(),
		}))
	}

	sm.registerSlackTools(toolReg)
	sm.registerDelegateTool(toolReg)
	sm.registerScheduleTools(toolReg)
	sm.registerLearningsTools(toolReg)
	sm.registerWebTools(toolReg)

	scratchDir := config.ScratchDir(sid)
	workspaceDir := sm.cfg.ResolvedWorkspaceDir()

	subagentWebDeps := tools.WebToolDeps{
		WS:       sm.webSearch,
		Provider: sm.fastProvider,
		Model:    sm.fastModel,
	}
	subagentLearningsDeps := tools.LearningsDeps{
		Store: learnings.New(config.LearningsDir()),
	}
	slackWS := ""
	if sm.cfg.Slack != nil {
		slackWS = sm.cfg.Slack.DefaultWorkspace
	}
	toolReg.Register(tools.BuildSubagentTool(tools.SubagentToolConfig{
		Provider:       sm.provider,
		Model:          sm.model,
		WebDeps:        &subagentWebDeps,
		LearningsDeps:  &subagentLearningsDeps,
		ScratchDir:     scratchDir,
		WorkspaceDir:   workspaceDir,
		SlackWorkspace: slackWS,
	}))

	identity := "You are a personal AI assistant communicating via WhatsApp.\n"
	extras := `
Talk like a helpful human, not a robot. Be casual, warm, and direct.
- Answer the question first. Don't restate what the user already knows (like today's date).
- Keep it short — one or two sentences when possible.
- Summarize by default, but include specifics when the user asks for details.
- Use natural language, not structured output. Say "you're free after 2" not "you have availability from 14:00-17:00".
- Do not use markdown formatting — WhatsApp does not render it. Use plain text only.
` + sm.userMemoriesPrompt()
	blocks := tools.BuildSystemBlocks(identity, toolReg, extras)

	opts := []agent.Option{agent.WithSystemBlocks(blocks)}
	if len(history) > 0 {
		opts = append(opts, agent.WithHistory(history))
	}
	opts = append(opts, agent.WithContextWindow(sm.resolveContextWindow()))
	opts = append(opts, agent.WithCompactionThreshold(sm.resolveCompactionThreshold()))
	opts = append(opts, agent.WithSummarizer(&agent.LLMSummarizer{
		Provider: sm.fastProvider,
		Model:    sm.fastModel,
	}))
	recorder := sm.openUsageRecorder()
	if recorder != nil {
		opts = append(opts, agent.WithUsageRecorder(recorder))
	}
	opts = append(opts, agent.WithOnIntermediateText(func(text string) {
		sm.channel.Send(text)
	}))
	var executor agent.ToolExecutor = toolReg
	if onToolStart != nil {
		executor = &notifyingExecutor{delegate: toolReg, onToolStart: onToolStart}
	}
	return agent.New(sm.provider, sm.model, executor, opts...), recorder, al, nil
}

func (sm *SessionManager) registerDelegateTool(reg *tools.Registry) {
	agents := tools.DetectAgents()
	if len(agents) == 0 || sm.interactor == nil {
		return
	}
	sm.mu.Lock()
	delegateSID := sm.sessionID
	sm.mu.Unlock()
	reg.Register(tools.NewDelegateTaskTool(tools.DelegateTaskConfig{
		Interactor: sm.interactor,
		Agents:     agents,
		Tracker:    sm.taskTracker,
		ScratchDir: config.ScratchDir(delegateSID),
	}))
	reg.Register(tools.NewCheckTaskTool(sm.taskTracker))
}

func (sm *SessionManager) registerScheduleTools(reg *tools.Registry) {
	meta := scheduler.ChannelMeta{}
	if sm.ownerJID != "" && sm.accountLabel != "" {
		meta.WAOwnerJID = sm.ownerJID
		meta.WAAccountLabel = sm.accountLabel
	}
	deps := tools.ScheduleToolDeps{
		Cfg:         sm.cfg,
		Channel:     "whatsapp",
		ChannelMeta: meta,
	}
	reg.Register(tools.NewCreateScheduleTool(deps))
	reg.Register(tools.NewListSchedulesTool(deps))
	reg.Register(tools.NewDeleteScheduleTool(deps))
}

func (sm *SessionManager) registerLearningsTools(reg *tools.Registry) {
	st := learnings.New(config.LearningsDir())

	var baseURL string
	if sm.cfg.Integrations != nil && sm.cfg.Integrations.GWS != nil && sm.cfg.Integrations.GWS.NgrokDomain != "" {
		baseURL = "https://" + sm.cfg.Integrations.GWS.NgrokDomain
	}

	deps := tools.LearningsDeps{
		Store:   st,
		BaseURL: baseURL,
	}
	reg.Register(tools.NewLearningSaveTool(deps))
	reg.Register(tools.NewLearningReadTool(deps))
	reg.Register(tools.NewLearningSearchTool(deps))

	extractDeps := tools.LearningsExtractDeps{
		LearningsDeps: deps,
		Provider:      sm.provider,
		Model:         sm.model,
	}
	reg.Register(tools.NewLearningExtractTool(extractDeps))
}

func (sm *SessionManager) initWebSearch() {
	ws, _ := tools.NewWebSearchInstance(tools.WebSearchSetup{
		WSConfig: sm.cfg.WebSearch,
		DSN:      sm.cfg.WebSearchDataDSN(),
	})
	sm.webSearch = ws

	reg, err := provider.NewRegistry(sm.cfg.Models)
	if err == nil {
		sm.fastProvider, sm.fastModel = tools.ResolveFastProvider(
			sm.cfg.Models, reg, sm.provider, sm.model,
		)
		sm.nanoProvider, sm.nanoModel = tools.ResolveNanoProvider(
			sm.cfg.Models, reg, sm.provider, sm.model,
		)
	} else {
		sm.fastProvider = sm.provider
		sm.fastModel = sm.model
		sm.nanoProvider = sm.provider
		sm.nanoModel = sm.model
	}
}

func (sm *SessionManager) registerWebTools(reg *tools.Registry) {
	deps := tools.WebToolDeps{
		WS:       sm.webSearch,
		Provider: sm.fastProvider,
		Model:    sm.fastModel,
	}
	reg.Register(tools.NewWebSearchTool(deps))
	reg.Register(tools.NewWebFetchTool(deps))
}

func (sm *SessionManager) registerSlackTools(reg *tools.Registry) {
	if sm.cfg.Slack == nil || sm.cfg.Slack.DefaultWorkspace == "" || sm.interactor == nil {
		return
	}
	creds, err := slacksrc.LoadCredentials(sm.cfg.Slack.DefaultWorkspace)
	if err != nil {
		return
	}
	client := slacksrc.NewClient(creds.Token, creds.Cookie)
	deps := tools.SlackToolDeps{
		Client:     client,
		Interactor: sm.interactor,
		Workspace:  sm.cfg.Slack.DefaultWorkspace,
		ScratchDir: config.ScratchDir(sm.sessionID),
	}
	reg.Register(tools.NewSlackSearchTool(deps))
	reg.Register(tools.NewSlackReadChannelTool(deps))
	reg.Register(tools.NewSlackReadThreadTool(deps))
	reg.Register(tools.NewSlackMediaDownloadTool(deps))
	reg.Register(tools.NewSlackSendTool(deps))
	reg.Register(tools.NewSlackEditTool(deps))
	reg.Register(tools.NewSlackReactTool(deps))
}

func (sm *SessionManager) userMemoriesPrompt() string {
	dir := config.UserMemoryDir()
	memory.EnsureDir(dir)
	ms := memory.NewStore(dir)
	memories, err := ms.List()
	if err != nil || len(memories) == 0 {
		return ""
	}
	return "\n" + memory.FormatForPrompt(memories)
}

func (sm *SessionManager) saveHistory(sessionID, userMsg, assistantMsg string) {
	histStore := sm.openHistoryStore()

	if err := histStore.UpsertConversation(sessionID, "whatsapp"); err != nil {
		slog.Error("whatsapp session: create conversation", "error", err)
		return
	}

	if err := histStore.SaveMessage(sessionID, "user", userMsg); err != nil {
		slog.Error("whatsapp session: save user message", "error", err)
	}
	if err := histStore.SaveMessage(sessionID, "assistant", assistantMsg); err != nil {
		slog.Error("whatsapp session: save assistant message", "error", err)
	}
}

func (sm *SessionManager) openUsageRecorder() *usagesrc.Recorder {
	path := config.UsageJSONLPath()
	if err := usagesrc.EnsureDir(path); err != nil {
		return nil
	}

	sm.mu.Lock()
	sid := sm.sessionID
	sm.mu.Unlock()

	return usagesrc.NewRecorder(path, sm.providerName, "whatsapp", sid)
}

func (sm *SessionManager) openHistoryStore() *historysrc.Store {
	dir := config.HistoryDir()
	historysrc.EnsureDir(dir)
	return historysrc.NewStore(dir)
}

func (sm *SessionManager) openAuditLogger() *audit.Logger {
	return audit.OpenDefault(config.AuditJSONLPath())
}

func openTaskTracker(cfg *config.Config) *tools.TaskTracker {
	if err := config.EnsureSourceDir("tasks"); err != nil {
		slog.Warn("tasks: ensure dir failed", "error", err)
		return tools.NewTaskTracker()
	}
	return tools.OpenPersistentTaskTracker(cfg.Tasks.Storage.Driver, cfg.TasksDataDSN())
}

// notifyingExecutor wraps a ToolExecutor to call onToolStart before each execution.
type notifyingExecutor struct {
	delegate    agent.ToolExecutor
	onToolStart func(string)
}

func (n *notifyingExecutor) Execute(ctx context.Context, call provider.ToolCall) (string, error) {
	if n.onToolStart != nil {
		n.onToolStart(call.Name)
	}
	return n.delegate.Execute(ctx, call)
}

func (n *notifyingExecutor) ToolSchemas() []provider.Tool {
	return n.delegate.ToolSchemas()
}

func generateSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("wa-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("wa-%x", b[:])
}

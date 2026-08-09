package cli

import (
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/73ai/openbotkit/agent"
	"github.com/73ai/openbotkit/agent/audit"
	"github.com/73ai/openbotkit/agent/tools"
	clicli "github.com/73ai/openbotkit/channel/cli"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/service/learnings"
	usagesrc "github.com/73ai/openbotkit/service/usage"
	slacksrc "github.com/73ai/openbotkit/source/slack"
	"github.com/73ai/openbotkit/store"

	// Register provider factories.
	_ "github.com/73ai/openbotkit/provider/anthropic"
	_ "github.com/73ai/openbotkit/provider/gemini"
	_ "github.com/73ai/openbotkit/provider/groq"
	_ "github.com/73ai/openbotkit/provider/openai"
	_ "github.com/73ai/openbotkit/provider/openrouter"
	_ "github.com/73ai/openbotkit/provider/zai"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:     "chat",
	Short:   "Start an interactive chat with the AI assistant",
	Example: `  obk chat`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := cfg.RequireSetup(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		registry, err := provider.NewRegistry(cfg.Models)
		if err != nil {
			return fmt.Errorf("create provider registry: %w", err)
		}

		// Resolve the default model's provider and model name.
		providerName, modelName, err := provider.ParseModelSpec(cfg.Models.Default)
		if err != nil {
			return fmt.Errorf("parse model spec: %w", err)
		}
		p, ok := registry.Get(providerName)
		if !ok {
			return fmt.Errorf("provider %q not found", providerName)
		}

		sessionID := generateSessionID()

		// Open history store for saving conversation.
		histStore, err := openHistoryStore(sessionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: history will not be saved: %v\n", err)
		}

		ch := clicli.New(os.Stdin, os.Stdout)

		// Set up audit logging.
		auditLogger := openAuditLogger()
		if auditLogger != nil {
			defer auditLogger.Close()
		}

		// Build tool registry with CLI approval gate.
		inter := NewCLIInteractor(ch)
		approvalRules := tools.NewApprovalRuleSet()
		toolReg := tools.NewStandardRegistry(inter, approvalRules)
		if err := config.EnsureScratchDir(sessionID); err != nil {
			slog.Warn("scratch dir creation failed", "error", err)
		}
		toolReg.SetScratchDir(config.ScratchDir(sessionID))
		defer config.CleanScratch(sessionID)
		if auditLogger != nil {
			toolReg.SetAudit(auditLogger, "cli")
		}
		scratchDir := config.ScratchDir(sessionID)
		workspaceDir := cfg.ResolvedWorkspaceDir()

		subagentWebDeps := registerWebDeps(cfg, registry, p, modelName)
		subagentLearningsDeps := tools.LearningsDeps{
			Store: learnings.New(config.LearningsDir()),
		}
		slackWS := ""
		if cfg.Slack != nil {
			slackWS = cfg.Slack.DefaultWorkspace
		}
		toolReg.Register(tools.BuildSubagentTool(tools.SubagentToolConfig{
			Provider:       p,
			Model:          modelName,
			WebDeps:        &subagentWebDeps,
			LearningsDeps:  &subagentLearningsDeps,
			ScratchDir:     scratchDir,
			WorkspaceDir:   workspaceDir,
			SlackWorkspace: slackWS,
		}))

		// Register delegate_task if external AI CLIs are available.
		tracker := openTaskTracker(cfg)
		defer tracker.Close()
		registerDelegateTool(toolReg, ch, tracker)

		// Register Slack tools if configured.
		registerSlackTools(cfg, toolReg, ch, scratchDir)

		// Register learnings tools.
		registerLearningsTools(toolReg)

		// Register web search/fetch tools.
		wsDB := registerWebTools(cfg, toolReg, registry, p, modelName)
		if wsDB != nil {
			defer wsDB.Close()
		}

		// Set up usage recording.
		usageRecorder := openUsageRecorder(cfg, providerName, "cli", sessionID)
		if usageRecorder != nil {
			defer usageRecorder.Close()
		}

		// Build system prompt with structured blocks for cache optimization.
		identity := "You are a personal AI assistant powered by OpenBotKit.\n"
		blocks := tools.BuildSystemBlocks(identity, toolReg)

		var agentOpts []agent.Option
		agentOpts = append(agentOpts, agent.WithSystemBlocks(blocks))
		if usageRecorder != nil {
			agentOpts = append(agentOpts, agent.WithUsageRecorder(usageRecorder))
		}
		a := agent.New(p, modelName, toolReg, agentOpts...)

		fmt.Println("OpenBotKit Chat (Ctrl+D to exit)")
		fmt.Println()

		for {
			input, err := ch.Receive()
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return nil
			}
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}
			if input == "" {
				continue
			}

			response, err := a.Run(cmd.Context(), input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}

			if histStore != nil {
				histStore.SaveMessage(sessionID, "user", input)
				histStore.SaveMessage(sessionID, "assistant", response)
			}

			ch.Send(response)
			fmt.Println()
		}
	},
}

func openHistoryStore(sessionID string) (*historysrc.Store, error) {
	dir := config.HistoryDir()
	if err := historysrc.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("ensure history dir: %w", err)
	}
	s := historysrc.NewStore(dir)
	cwd, _ := os.Getwd()
	if err := s.UpsertConversation(sessionID, cwd); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return s, nil
}

func openUsageRecorder(cfg *config.Config, providerName, channel, sessionID string) *usagesrc.Recorder {
	path := config.UsageJSONLPath()
	if err := usagesrc.EnsureDir(path); err != nil {
		return nil
	}
	return usagesrc.NewRecorder(path, providerName, channel, sessionID)
}

func generateSessionID() string {
	var b [16]byte
	rand.Read(b[:])
	return fmt.Sprintf("obk-chat-%x", b[:])
}

func openAuditLogger() *audit.Logger {
	return audit.OpenDefault(config.AuditJSONLPath())
}

func registerSlackTools(cfg *config.Config, reg *tools.Registry, ch *clicli.Channel, scratchDir string) {
	if cfg.Slack == nil || cfg.Slack.DefaultWorkspace == "" {
		return
	}
	creds, err := slacksrc.LoadCredentials(cfg.Slack.DefaultWorkspace)
	if err != nil {
		slog.Debug("slack tools not loaded: no credentials", "workspace", cfg.Slack.DefaultWorkspace)
		return
	}
	client := slacksrc.NewClient(creds.Token, creds.Cookie)
	inter := NewCLIInteractor(ch)
	deps := tools.SlackToolDeps{
		Client:     client,
		Interactor: inter,
		Workspace:  cfg.Slack.DefaultWorkspace,
		ScratchDir: scratchDir,
	}

	reg.Register(tools.NewSlackSearchTool(deps))
	reg.Register(tools.NewSlackReadChannelTool(deps))
	reg.Register(tools.NewSlackReadThreadTool(deps))
	reg.Register(tools.NewSlackMediaDownloadTool(deps))
	reg.Register(tools.NewSlackSendTool(deps))
	reg.Register(tools.NewSlackEditTool(deps))
	reg.Register(tools.NewSlackReactTool(deps))
}

func openTaskTracker(cfg *config.Config) *tools.TaskTracker {
	if err := config.EnsureSourceDir("tasks"); err != nil {
		slog.Warn("tasks: ensure dir failed", "error", err)
		return tools.NewTaskTracker()
	}
	return tools.OpenPersistentTaskTracker(cfg.Tasks.Storage.Driver, cfg.TasksDataDSN())
}

func registerDelegateTool(reg *tools.Registry, ch *clicli.Channel, tracker *tools.TaskTracker) {
	agents := tools.DetectAgents()
	if len(agents) == 0 {
		return
	}
	inter := NewCLIInteractor(ch)
	reg.Register(tools.NewDelegateTaskTool(tools.DelegateTaskConfig{
		Interactor: inter,
		Agents:     agents,
		Tracker:    tracker,
	}))
	reg.Register(tools.NewCheckTaskTool(tracker))
}

func registerLearningsTools(reg *tools.Registry) {
	st := learnings.New(config.LearningsDir())
	deps := tools.LearningsDeps{Store: st}
	reg.Register(tools.NewLearningSaveTool(deps))
	reg.Register(tools.NewLearningReadTool(deps))
	reg.Register(tools.NewLearningSearchTool(deps))
}

// registerWebTools adds web_search and web_fetch tools. Returns an optional
// DB handle that the caller must close when done.
func registerWebTools(cfg *config.Config, reg *tools.Registry, provRegistry *provider.Registry, defaultP provider.Provider, defaultModel string) *store.DB {
	ws, wsDB := tools.NewWebSearchInstance(tools.WebSearchSetup{
		WSConfig: cfg.WebSearch,
		DSN:      cfg.WebSearchDataDSN(),
	})
	fastP, fastModel := tools.ResolveFastProvider(cfg.Models, provRegistry, defaultP, defaultModel)
	deps := tools.WebToolDeps{WS: ws, Provider: fastP, Model: fastModel}
	reg.Register(tools.NewWebSearchTool(deps))
	reg.Register(tools.NewWebFetchTool(deps))
	return wsDB
}

func registerWebDeps(cfg *config.Config, provRegistry *provider.Registry, defaultP provider.Provider, defaultModel string) tools.WebToolDeps {
	ws, _ := tools.NewWebSearchInstance(tools.WebSearchSetup{
		WSConfig: cfg.WebSearch,
		DSN:      cfg.WebSearchDataDSN(),
	})
	fastP, fastModel := tools.ResolveFastProvider(cfg.Models, provRegistry, defaultP, defaultModel)
	return tools.WebToolDeps{WS: ws, Provider: fastP, Model: fastModel}
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

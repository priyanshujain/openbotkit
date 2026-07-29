package tools

import (
	"errors"
	"os"

	"github.com/73ai/openbotkit/provider"
	"github.com/73ai/openbotkit/source/slack"
)

// denyInteractor implements Interactor for sub-agent contexts where
// user interaction is not available. Matches Claude Code's background
// subagent model: guarded tools fail, subagent continues.
type denyInteractor struct{}

func (d *denyInteractor) Notify(_ string) error        { return nil }
func (d *denyInteractor) NotifyLink(_, _ string) error { return nil }
func (d *denyInteractor) RequestApproval(_ string) (bool, error) {
	return false, errors.New("not permitted in sub-agent context")
}

// SubagentRegistryDeps provides optional dependencies for building
// an enriched subagent tool registry.
type SubagentRegistryDeps struct {
	ScratchDir     string
	WebDeps        *WebToolDeps
	LearningsDeps  *LearningsDeps
	ScheduleDeps   *ScheduleToolDeps
	SlackClient    slack.API
	SlackWorkspace string
	Agents         []AgentInfo
}

// NewSubagentRegistry creates a tool registry for subagent contexts.
// All tools are registered so the subagent sees them. Safe tools work
// normally; guarded tools get a denyInteractor so they fail via the
// existing GuardedAction error path. The subagent tool itself is
// excluded to prevent recursion.
func NewSubagentRegistry(deps SubagentRegistryDeps) *Registry {
	deny := &denyInteractor{}

	r := NewStandardRegistry(nil, nil)
	if deps.ScratchDir != "" {
		r.SetScratchDir(deps.ScratchDir)
	}

	// Safe tools — no guard, always work.
	if deps.WebDeps != nil {
		r.Register(NewWebSearchTool(*deps.WebDeps))
		r.Register(NewWebFetchTool(*deps.WebDeps))
	}
	if deps.LearningsDeps != nil {
		r.Register(NewLearningSaveTool(*deps.LearningsDeps))
		r.Register(NewLearningReadTool(*deps.LearningsDeps))
		r.Register(NewLearningSearchTool(*deps.LearningsDeps))
	}
	if deps.ScheduleDeps != nil {
		r.Register(NewCreateScheduleTool(*deps.ScheduleDeps))
		r.Register(NewListSchedulesTool(*deps.ScheduleDeps))
		r.Register(NewDeleteScheduleTool(*deps.ScheduleDeps))
	}

	// Guarded tools — denyInteractor makes them fail via GuardedAction.
	if deps.SlackClient != nil {
		readDeps := SlackToolDeps{
			Client:     deps.SlackClient,
			Workspace:  deps.SlackWorkspace,
			ScratchDir: deps.ScratchDir,
		}
		r.Register(NewSlackSearchTool(readDeps))
		r.Register(NewSlackReadChannelTool(readDeps))
		r.Register(NewSlackReadThreadTool(readDeps))
		r.Register(NewSlackMediaDownloadTool(readDeps))
		writeDeps := SlackToolDeps{Client: deps.SlackClient, Interactor: deny}
		r.Register(NewSlackSendTool(writeDeps))
		r.Register(NewSlackEditTool(writeDeps))
		r.Register(NewSlackReactTool(writeDeps))
	}
	if len(deps.Agents) > 0 {
		r.Register(NewDelegateTaskTool(DelegateTaskConfig{
			Interactor: deny,
			Agents:     deps.Agents,
			ScratchDir: deps.ScratchDir,
		}))
	}

	// NOT registered: subagent (no recursion)
	return r
}

// SubagentToolConfig holds everything needed to create a subagent tool.
// Use BuildSubagentTool to avoid duplicating setup across channels.
type SubagentToolConfig struct {
	Provider       provider.Provider
	Model          string
	WebDeps        *WebToolDeps
	LearningsDeps  *LearningsDeps
	ScheduleDeps   *ScheduleToolDeps
	ScratchDir     string
	WorkspaceDir   string
	SlackWorkspace string // if non-empty, load Slack credentials
}

// BuildSubagentTool creates a fully-configured SubagentTool with enriched
// tools, optional Slack, detected agents, and workspace extras.
func BuildSubagentTool(cfg SubagentToolConfig) *SubagentTool {
	if cfg.WorkspaceDir != "" {
		os.MkdirAll(cfg.WorkspaceDir, 0700)
	}

	deps := SubagentRegistryDeps{
		ScratchDir:    cfg.ScratchDir,
		WebDeps:       cfg.WebDeps,
		LearningsDeps: cfg.LearningsDeps,
		ScheduleDeps:  cfg.ScheduleDeps,
	}
	if cfg.SlackWorkspace != "" {
		if creds, err := slack.LoadCredentials(cfg.SlackWorkspace); err == nil {
			deps.SlackClient = slack.NewClient(creds.Token, creds.Cookie)
			deps.SlackWorkspace = cfg.SlackWorkspace
		}
	}
	deps.Agents = DetectAgents()

	return NewSubagentTool(SubagentConfig{
		Provider: cfg.Provider,
		Model:    cfg.Model,
		ToolFactory: func() *Registry {
			return NewSubagentRegistry(deps)
		},
		System: "You are a focused sub-agent. Complete the given task and return a concise result.",
		Extras: []string{"\nWorkspace directory: " + cfg.WorkspaceDir + "\n"},
	})
}

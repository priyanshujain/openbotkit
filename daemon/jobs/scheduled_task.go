package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/73ai/openbotkit/agent"
	"github.com/73ai/openbotkit/agent/audit"
	"github.com/73ai/openbotkit/agent/tools"
	"github.com/73ai/openbotkit/channel"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	"github.com/73ai/openbotkit/service/scheduler"
	"github.com/73ai/openbotkit/service/tasks"
	"github.com/73ai/openbotkit/store"
)

type ScheduledTaskArgs struct {
	ScheduleID   int64   `json:"schedule_id"`
	Task         string  `json:"task"`
	Channel      string  `json:"channel"`
	ChannelMeta  string  `json:"channel_meta"`
	ModelTier    string  `json:"model_tier,omitempty"`
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty"`
}

func (ScheduledTaskArgs) Kind() string { return "scheduled_task" }

// PusherFactory builds a Pusher for delivering results. If nil, the default
// channel.NewPusher is used.
type PusherFactory func(channelType string, meta scheduler.ChannelMeta) (channel.Pusher, error)

// AgentRunner executes a task prompt and returns the result text.
// If nil, the default LLM-based agent is used.
type AgentRunner func(ctx context.Context, task string) (string, error)

type ScheduledTaskWorker struct {
	river.WorkerDefaults[ScheduledTaskArgs]
	Cfg          *config.Config
	MakePusher   PusherFactory
	RunAgentFunc AgentRunner
	TasksDB      *store.DB // optional: for recording task results
}

func (w *ScheduledTaskWorker) Work(ctx context.Context, job *river.Job[ScheduledTaskArgs]) error {
	slog.Info("running scheduled task", "schedule_id", job.Args.ScheduleID, "attempt", job.Attempt)
	taskID := fmt.Sprintf("sched-%d-%d", job.Args.ScheduleID, time.Now().UnixMilli())
	w.recordTaskStart(taskID, job.Args.Task)

	var meta scheduler.ChannelMeta
	if err := json.Unmarshal([]byte(job.Args.ChannelMeta), &meta); err != nil {
		return fmt.Errorf("parse channel meta: %w", err)
	}

	var result string
	var err error
	if w.RunAgentFunc != nil {
		result, err = w.RunAgentFunc(ctx, job.Args.Task)
	} else {
		result, err = w.runAgentWithBudget(ctx, job.Args.Task, job.Args.ModelTier, job.Args.MaxBudgetUSD)
	}
	if err != nil {
		slog.Error("scheduled task agent failed", "schedule_id", job.Args.ScheduleID, "error", err)
		w.updateLastRun(job.Args.ScheduleID, err.Error())
		w.recordTaskFailed(taskID, err.Error())

		apiErr := provider.ClassifyError(err)
		if apiErr.Kind == provider.ErrorAuth || apiErr.Kind == provider.ErrorContextWindow {
			w.notifyFailure(ctx, job.Args.Channel, meta, job.Args.ScheduleID, err)
			return river.JobCancel(err)
		}

		if job.Attempt >= job.MaxAttempts {
			w.notifyFailure(ctx, job.Args.Channel, meta, job.Args.ScheduleID, err)
			return nil
		}
		return err
	}

	pusher, err := w.newPusher(job.Args.Channel, meta)
	if err != nil {
		slog.Error("scheduled task: create pusher", "schedule_id", job.Args.ScheduleID, "error", err)
		w.updateLastRun(job.Args.ScheduleID, fmt.Sprintf("create pusher: %v", err))
		return fmt.Errorf("create pusher: %w", err)
	}
	if err := pusher.Push(ctx, result); err != nil {
		slog.Error("scheduled task: push result", "schedule_id", job.Args.ScheduleID, "error", err)
		w.updateLastRun(job.Args.ScheduleID, fmt.Sprintf("push: %v", err))
		return fmt.Errorf("push result: %w", err)
	}

	w.updateLastRun(job.Args.ScheduleID, "")
	w.recordTaskCompleted(taskID, result)
	w.maybeMarkCompleted(job.Args.ScheduleID)

	slog.Info("scheduled task complete", "schedule_id", job.Args.ScheduleID)
	return nil
}

func (w *ScheduledTaskWorker) NextRetry(job *river.Job[ScheduledTaskArgs]) time.Time {
	if len(job.Errors) == 0 {
		return time.Now().Add(15 * time.Minute)
	}
	// Auth and context-window errors are cancelled in Work() via
	// river.JobCancel, so NextRetry is only called for retryable errors.
	lastErr := job.Errors[len(job.Errors)-1]
	apiErr := provider.ClassifyError(fmt.Errorf("%s", lastErr.Error))
	if apiErr.Kind == provider.ErrorRetryable && apiErr.StatusCode == 429 {
		return time.Now().Add(30 * time.Minute)
	}
	if apiErr.Kind == provider.ErrorRetryable {
		return time.Now().Add(10 * time.Minute) // 5xx
	}
	return time.Now().Add(15 * time.Minute)
}

func (w *ScheduledTaskWorker) runAgent(ctx context.Context, task string) (string, error) {
	return w.runAgentWithBudget(ctx, task, "", 0)
}

func (w *ScheduledTaskWorker) runAgentWithBudget(ctx context.Context, task string, modelTier string, maxBudget float64) (string, error) {
	if w.Cfg == nil || w.Cfg.Models == nil || w.Cfg.Models.Default == "" {
		return "", fmt.Errorf("no LLM model configured")
	}

	registry, err := provider.NewRegistry(w.Cfg.Models)
	if err != nil {
		return "", fmt.Errorf("create provider registry: %w", err)
	}

	router := provider.NewRouter(registry, w.Cfg.Models)
	tier := provider.TierFast
	if modelTier != "" {
		tier = provider.ModelTier(modelTier)
	}
	p, modelName, err := router.Resolve(tier)
	if err != nil {
		return "", fmt.Errorf("resolve model tier %q: %w", tier, err)
	}

	toolReg := tools.NewScheduledTaskRegistry()
	sessionID := fmt.Sprintf("sched-%d", time.Now().UnixMilli())
	if err := config.EnsureScratchDir(sessionID); err != nil {
		slog.Warn("scratch dir creation failed", "error", err)
	}
	toolReg.SetScratchDir(config.ScratchDir(sessionID))
	defer config.CleanScratch(sessionID)

	al := openAuditLogger()
	if al != nil {
		defer al.Close()
		toolReg.SetAudit(al, "scheduled")
	}

	identity := "You are a scheduled task agent. Execute the task and return a concise result.\n"
	blocks := tools.BuildSystemBlocks(identity, toolReg)

	opts := []agent.Option{agent.WithSystemBlocks(blocks)}
	if maxBudget > 0 {
		bt := agent.NewBudgetTracker(maxBudget, nil)
		opts = append(opts, agent.WithUsageRecorder(bt), agent.WithBudgetChecker(bt))
	}

	a := agent.New(p, modelName, toolReg, opts...)
	return a.Run(ctx, task)
}

func (w *ScheduledTaskWorker) updateLastRun(scheduleID int64, lastErr string) {
	db, err := store.Open(store.Config{
		Driver: w.Cfg.Scheduler.Storage.Driver,
		DSN:    w.Cfg.SchedulerDataDSN(),
	})
	if err != nil {
		slog.Error("scheduled task: open scheduler db", "error", err)
		return
	}
	defer db.Close()

	if err := scheduler.UpdateLastRun(db, scheduleID, time.Now().UTC(), lastErr); err != nil {
		slog.Error("scheduled task: update last run", "error", err)
	}
}

func (w *ScheduledTaskWorker) maybeMarkCompleted(scheduleID int64) {
	db, err := store.Open(store.Config{
		Driver: w.Cfg.Scheduler.Storage.Driver,
		DSN:    w.Cfg.SchedulerDataDSN(),
	})
	if err != nil {
		return
	}
	defer db.Close()

	s, err := scheduler.Get(db, scheduleID)
	if err != nil {
		return
	}
	if s.Type == scheduler.OneShot {
		_ = scheduler.MarkCompleted(db, scheduleID, time.Now().UTC())
	}
}

func (w *ScheduledTaskWorker) newPusher(channelType string, meta scheduler.ChannelMeta) (channel.Pusher, error) {
	if w.MakePusher != nil {
		return w.MakePusher(channelType, meta)
	}
	return channel.NewPusher(channelType, meta)
}

func (w *ScheduledTaskWorker) notifyFailure(ctx context.Context, ch string, meta scheduler.ChannelMeta, scheduleID int64, taskErr error) {
	pusher, err := w.newPusher(ch, meta)
	if err != nil {
		slog.Error("scheduled task: create failure pusher", "error", err)
		return
	}
	msg := fmt.Sprintf("Scheduled task #%d failed after retries: %v", scheduleID, taskErr)
	if err := pusher.Push(ctx, msg); err != nil {
		slog.Error("scheduled task: push failure notification", "error", err)
	}
}

func (w *ScheduledTaskWorker) recordTaskStart(taskID, task string) {
	if w.TasksDB == nil {
		return
	}
	if err := tasks.Insert(w.TasksDB, &tasks.TaskRecord{
		ID: taskID, Task: task, Agent: "scheduled",
		Status: "running", StartedAt: time.Now().UTC(),
	}); err != nil {
		slog.Warn("tasks: record start failed", "id", taskID, "error", err)
	}
}

func (w *ScheduledTaskWorker) recordTaskCompleted(taskID, output string) {
	if w.TasksDB == nil {
		return
	}
	if err := tasks.SetCompleted(w.TasksDB, taskID, output); err != nil {
		slog.Warn("tasks: record completed failed", "id", taskID, "error", err)
	}
}

func (w *ScheduledTaskWorker) recordTaskFailed(taskID, errMsg string) {
	if w.TasksDB == nil {
		return
	}
	if err := tasks.SetFailed(w.TasksDB, taskID, errMsg); err != nil {
		slog.Warn("tasks: record failed failed", "id", taskID, "error", err)
	}
}

func openAuditLogger() *audit.Logger {
	return audit.OpenDefault(config.AuditJSONLPath())
}

var _ river.Worker[ScheduledTaskArgs] = (*ScheduledTaskWorker)(nil)

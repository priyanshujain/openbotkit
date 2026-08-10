package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/service/scheduler"
	"github.com/73ai/openbotkit/store"
)

type ScheduleToolDeps struct {
	Cfg         *config.Config
	Channel     string
	ChannelMeta scheduler.ChannelMeta
}

func (d ScheduleToolDeps) openDB() (*store.DB, error) {
	if err := config.EnsureSourceDir("scheduler"); err != nil {
		return nil, fmt.Errorf("ensure scheduler dir: %w", err)
	}
	db, err := store.Open(store.Config{
		Driver: d.Cfg.Scheduler.Storage.Driver,
		DSN:    d.Cfg.SchedulerDataDSN(),
	})
	if err != nil {
		return nil, fmt.Errorf("open scheduler db: %w", err)
	}
	if err := scheduler.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate scheduler db: %w", err)
	}
	return db, nil
}

// CreateScheduleTool

type CreateScheduleTool struct {
	deps ScheduleToolDeps
}

func NewCreateScheduleTool(deps ScheduleToolDeps) *CreateScheduleTool {
	return &CreateScheduleTool{deps: deps}
}

func (t *CreateScheduleTool) Name() string { return "create_schedule" }
func (t *CreateScheduleTool) Description() string {
	return "Create a recurring, one-shot, or reactive scheduled task"
}
func (t *CreateScheduleTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"type": {
				"type": "string",
				"enum": ["recurring", "one_shot", "reactive"],
				"description": "Schedule type"
			},
			"cron_expr": {
				"type": "string",
				"description": "5-field UTC cron expression (for recurring)"
			},
			"scheduled_at": {
				"type": "string",
				"description": "UTC ISO 8601 datetime (for one_shot)"
			},
			"task": {
				"type": "string",
				"description": "Self-contained prompt for the scheduled agent"
			},
			"timezone": {
				"type": "string",
				"description": "User's timezone (e.g. America/New_York)"
			},
			"description": {
				"type": "string",
				"description": "Human-readable description of the schedule"
			},
			"model_tier": {
				"type": "string",
				"enum": ["fast", "default", "complex", "nano"],
				"description": "Model tier for the task (default: fast)"
			},
			"max_budget_usd": {
				"type": "number",
				"description": "Maximum cost budget in USD (0 = unlimited)"
			},
			"trigger_source": {
				"type": "string",
				"enum": ["gmail", "whatsapp", "telegram", "imessage", "applenotes"],
				"description": "Data source to watch (for reactive type)"
			},
			"trigger_query": {
				"type": "string",
				"description": "SQL WHERE clause to match new rows (for reactive type)"
			}
		},
		"required": ["type", "task", "timezone"]
	}`)
}

type createScheduleInput struct {
	Type          string  `json:"type"`
	CronExpr      string  `json:"cron_expr"`
	ScheduledAt   string  `json:"scheduled_at"`
	Task          string  `json:"task"`
	Timezone      string  `json:"timezone"`
	Description   string  `json:"description"`
	ModelTier     string  `json:"model_tier"`
	MaxBudgetUSD  float64 `json:"max_budget_usd"`
	TriggerSource string  `json:"trigger_source"`
	TriggerQuery  string  `json:"trigger_query"`
}

func (t *CreateScheduleTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var in createScheduleInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if in.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	loc, err := time.LoadLocation(in.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone %q: %w", in.Timezone, err)
	}

	modelTier := in.ModelTier
	if modelTier == "" {
		modelTier = "fast"
	}

	s := &scheduler.Schedule{
		Type:         scheduler.ScheduleType(in.Type),
		Task:         in.Task,
		Channel:      t.deps.Channel,
		ChannelMeta:  t.deps.ChannelMeta,
		Timezone:     in.Timezone,
		Description:  in.Description,
		ModelTier:    modelTier,
		MaxBudgetUSD: in.MaxBudgetUSD,
	}

	switch s.Type {
	case scheduler.Recurring:
		if in.CronExpr == "" {
			return "", fmt.Errorf("cron_expr is required for recurring schedules")
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		cronSched, err := parser.Parse(in.CronExpr)
		if err != nil {
			return "", fmt.Errorf("invalid cron expression: %w", err)
		}
		now := time.Now().UTC()
		first := cronSched.Next(now)
		second := cronSched.Next(first)
		if second.Sub(first) < time.Hour {
			return "", fmt.Errorf("recurring schedules must run at most once per hour")
		}
		s.CronExpr = in.CronExpr

	case scheduler.OneShot:
		if in.ScheduledAt == "" {
			return "", fmt.Errorf("scheduled_at is required for one-shot schedules")
		}
		scheduledAt, err := time.Parse(time.RFC3339, in.ScheduledAt)
		if err != nil {
			return "", fmt.Errorf("invalid scheduled_at (expected RFC3339): %w", err)
		}
		if scheduledAt.Before(time.Now().UTC()) {
			return "", fmt.Errorf("scheduled_at must be in the future")
		}
		s.ScheduledAt = &scheduledAt

	case scheduler.Reactive:
		if in.TriggerSource == "" {
			return "", fmt.Errorf("trigger_source is required for reactive schedules")
		}
		if err := scheduler.ValidateTriggerSource(in.TriggerSource); err != nil {
			return "", err
		}
		if err := scheduler.ValidateTriggerQuery(in.TriggerSource, in.TriggerQuery); err != nil {
			return "", err
		}
		s.TriggerSource = in.TriggerSource
		s.TriggerQuery = in.TriggerQuery

	default:
		return "", fmt.Errorf("type must be 'recurring', 'one_shot', or 'reactive'")
	}

	db, err := t.deps.openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	id, err := scheduler.Create(db, s)
	if err != nil {
		return "", fmt.Errorf("create schedule: %w", err)
	}

	if s.Type == scheduler.Reactive {
		return fmt.Sprintf("Reactive schedule created (ID: %d). Watching %s for: %s. Description: %s.", id, in.TriggerSource, in.TriggerQuery, in.Description), nil
	}
	nextRun := t.formatNextRun(s, loc)
	return fmt.Sprintf("Schedule created (ID: %d). Description: %s. Next run: %s (in your timezone).", id, in.Description, nextRun), nil
}

func (t *CreateScheduleTool) formatNextRun(s *scheduler.Schedule, loc *time.Location) string {
	if s.Type == scheduler.OneShot && s.ScheduledAt != nil {
		return s.ScheduledAt.In(loc).Format("2006-01-02 15:04 MST")
	}
	if s.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if sched, err := parser.Parse(s.CronExpr); err == nil {
			next := sched.Next(time.Now().UTC())
			return next.In(loc).Format("2006-01-02 15:04 MST")
		}
	}
	return "unknown"
}

// ListSchedulesTool

type ListSchedulesTool struct {
	deps ScheduleToolDeps
}

func NewListSchedulesTool(deps ScheduleToolDeps) *ListSchedulesTool {
	return &ListSchedulesTool{deps: deps}
}

func (t *ListSchedulesTool) Name() string        { return "list_schedules" }
func (t *ListSchedulesTool) Description() string { return "List all scheduled tasks" }
func (t *ListSchedulesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type": "object", "properties": {}}`)
}

func (t *ListSchedulesTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	db, err := t.deps.openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	schedules, err := scheduler.List(db)
	if err != nil {
		return "", fmt.Errorf("list schedules: %w", err)
	}

	if len(schedules) == 0 {
		return "No scheduled tasks found.", nil
	}

	var b strings.Builder
	for _, s := range schedules {
		loc, _ := time.LoadLocation(s.Timezone)
		if loc == nil {
			loc = time.UTC
		}

		status := "enabled"
		if !s.Enabled {
			status = "disabled"
		}

		fmt.Fprintf(&b, "ID: %d | %s | %s | %s | tier=%s", s.ID, s.Type, status, s.Description, s.ModelTier)
		if s.MaxBudgetUSD > 0 {
			fmt.Fprintf(&b, " | budget=$%.2f", s.MaxBudgetUSD)
		}
		fmt.Fprintln(&b)
		if s.Type == scheduler.Recurring {
			fmt.Fprintf(&b, "  Cron: %s (UTC)\n", s.CronExpr)
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			if sched, err := parser.Parse(s.CronExpr); err == nil {
				next := sched.Next(time.Now().UTC())
				fmt.Fprintf(&b, "  Next run: %s\n", next.In(loc).Format("2006-01-02 15:04 MST"))
			}
		}
		if s.Type == scheduler.OneShot && s.ScheduledAt != nil {
			fmt.Fprintf(&b, "  Scheduled at: %s\n", s.ScheduledAt.In(loc).Format("2006-01-02 15:04 MST"))
		}
		if s.Type == scheduler.Reactive {
			fmt.Fprintf(&b, "  Source: %s | Query: %s\n", s.TriggerSource, s.TriggerQuery)
		}
		if s.LastRunAt != nil {
			fmt.Fprintf(&b, "  Last run: %s\n", s.LastRunAt.In(loc).Format("2006-01-02 15:04 MST"))
		}
		if s.LastError != "" {
			fmt.Fprintf(&b, "  Last error: %s\n", s.LastError)
		}
	}

	return b.String(), nil
}

// DeleteScheduleTool

type DeleteScheduleTool struct {
	deps ScheduleToolDeps
}

func NewDeleteScheduleTool(deps ScheduleToolDeps) *DeleteScheduleTool {
	return &DeleteScheduleTool{deps: deps}
}

func (t *DeleteScheduleTool) Name() string        { return "delete_schedule" }
func (t *DeleteScheduleTool) Description() string { return "Delete a scheduled task by ID" }
func (t *DeleteScheduleTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "integer",
				"description": "Schedule ID to delete"
			}
		},
		"required": ["id"]
	}`)
}

type deleteScheduleInput struct {
	ID int64 `json:"id"`
}

func (t *DeleteScheduleTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var in deleteScheduleInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if in.ID == 0 {
		return "", fmt.Errorf("id is required")
	}

	db, err := t.deps.openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	if err := scheduler.Delete(db, in.ID); err != nil {
		return "", fmt.Errorf("delete schedule: %w", err)
	}

	return fmt.Sprintf("Schedule %d deleted.", in.ID), nil
}

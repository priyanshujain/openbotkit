package spectest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/73ai/openbotkit/channel"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/daemon"
	"github.com/73ai/openbotkit/daemon/jobs"
	"github.com/73ai/openbotkit/service/scheduler"
	"github.com/73ai/openbotkit/store"
)

func TestSpec_ReactiveEmailTrigger(t *testing.T) {
	fx := NewLocalFixture(t)

	// Seed emails — only the boss email should trigger.
	fx.GivenEmails(t, []Email{
		{From: "newsletter@news.com", To: "me@example.com",
			Subject: "Weekly Digest", Body: "Here are this week's top stories..."},
		{From: "boss@acme.com", To: "me@example.com",
			Subject: "Q1 Planning", Body: "Please review the Q1 targets before our meeting tomorrow."},
		{From: "support@vendor.io", To: "me@example.com",
			Subject: "Ticket #1234 resolved", Body: "Your support ticket has been resolved."},
	})

	dir := fx.dir
	schedDBPath := filepath.Join(dir, "scheduler", "data.db")
	jobsDBPath := filepath.Join(dir, "jobs.db")
	gmailDBPath := filepath.Join(dir, "gmail", "data.db")

	// Create scheduler DB with reactive schedule.
	sdb, err := store.Open(store.SQLiteConfig(schedDBPath))
	if err != nil {
		t.Fatalf("open sched db: %v", err)
	}
	if err := scheduler.Migrate(sdb); err != nil {
		t.Fatalf("migrate sched: %v", err)
	}

	meta := scheduler.ChannelMeta{BotToken: "test", OwnerID: 1}

	scheduleID, err := scheduler.Create(sdb, &scheduler.Schedule{
		Type:          scheduler.Reactive,
		TriggerSource: "gmail",
		TriggerQuery:  "from_addr LIKE '%@acme.com%'",
		Task:          "Summarize this email in one sentence.",
		Channel:       "test",
		ChannelMeta:   meta,
		Timezone:      "UTC",
		Description:   "Summarize emails from Acme Corp",
		ModelTier:     "fast",
		MaxBudgetUSD:  1.0,
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	sdb.Close()

	cfg := &config.Config{
		Scheduler: &config.SchedulerConfig{
			Storage: config.StorageConfig{Driver: "sqlite", DSN: schedDBPath},
		},
	}

	// Set up River.
	jobsDB, err := sql.Open("sqlite", jobsDBPath)
	if err != nil {
		t.Fatalf("open jobs db: %v", err)
	}
	defer jobsDB.Close()
	jobsDB.SetMaxOpenConns(1)

	driver := riversqlite.New(jobsDB)
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if _, err := migrator.Migrate(context.Background(), rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	// Set up worker with real LLM and mock pusher.
	pusher := &capturePusher{}
	worker := &jobs.ScheduledTaskWorker{
		Cfg:          cfg,
		RunAgentFunc: makeAgentRunner(fx.Provider, fx.Model),
		MakePusher: func(_ string, _ scheduler.ChannelMeta) (channel.Pusher, error) {
			return pusher, nil
		},
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, worker)

	riverClient, err := river.NewClient(driver, &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
		Workers: workers,
	})
	if err != nil {
		t.Fatalf("create river client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := riverClient.Start(ctx); err != nil {
		t.Fatalf("start river: %v", err)
	}
	defer riverClient.Stop(context.Background())

	// Execute reactive trigger check.
	notifier := daemon.NewSyncNotifier()
	sched := daemon.NewScheduler(cfg, riverClient, jobsDB, notifier)

	gmailDB, err := store.Open(store.SQLiteConfig(gmailDBPath))
	if err != nil {
		t.Fatalf("open gmail db: %v", err)
	}
	defer gmailDB.Close()

	if err := sched.CheckReactiveTriggersForTest(ctx, "gmail", gmailDB); err != nil {
		t.Fatalf("reactive check: %v", err)
	}

	// Wait for the pusher to receive a message.
	deadline := time.After(90 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for reactive task result")
		case <-ticker.C:
			msgs := pusher.Messages()
			if len(msgs) == 0 {
				continue
			}
			t.Logf("pushed message: %s", msgs[0])

			// Verify schedule watermark was updated.
			sdb2, err := store.Open(store.SQLiteConfig(schedDBPath))
			if err != nil {
				t.Fatalf("reopen sched db: %v", err)
			}
			defer sdb2.Close()

			s, err := scheduler.Get(sdb2, scheduleID)
			if err != nil {
				t.Fatalf("get schedule: %v", err)
			}
			if s.LastTriggerID == 0 {
				t.Error("expected last_trigger_id to be updated (watermark)")
			}
			if s.LastRunAt == nil {
				t.Error("expected last_run_at to be set")
			}
			if s.LastError != "" {
				t.Errorf("expected no last_error, got %q", s.LastError)
			}

			// Verify re-running trigger doesn't re-fire (watermark prevents it).
			if err := sched.CheckReactiveTriggersForTest(ctx, "gmail", gmailDB); err != nil {
				t.Fatalf("second reactive check: %v", err)
			}
			time.Sleep(2 * time.Second)
			msgs2 := pusher.Messages()
			if len(msgs2) > 1 {
				t.Errorf("expected no additional messages after watermark update, got %d total", len(msgs2))
			}

			return
		}
	}
}

// TestSpec_ReactiveNoMatchDoesNotFire verifies no job is enqueued when nothing matches.
func TestSpec_ReactiveNoMatchDoesNotFire(t *testing.T) {
	fx := NewLocalFixture(t)

	fx.GivenEmails(t, []Email{
		{From: "newsletter@news.com", To: "me@example.com",
			Subject: "Weekly Digest", Body: "Here are this week's top stories..."},
	})

	dir := fx.dir
	schedDBPath := filepath.Join(dir, "scheduler", "data.db")
	jobsDBPath := filepath.Join(dir, "jobs.db")
	gmailDBPath := filepath.Join(dir, "gmail", "data.db")

	sdb, err := store.Open(store.SQLiteConfig(schedDBPath))
	if err != nil {
		t.Fatalf("open sched db: %v", err)
	}
	if err := scheduler.Migrate(sdb); err != nil {
		t.Fatalf("migrate sched: %v", err)
	}

	meta := scheduler.ChannelMeta{BotToken: "test", OwnerID: 1}

	_, err = scheduler.Create(sdb, &scheduler.Schedule{
		Type:          scheduler.Reactive,
		TriggerSource: "gmail",
		TriggerQuery:  "from_addr LIKE '%@doesnotexist.com%'",
		Task:          "Should not fire",
		Channel:       "test",
		ChannelMeta:   meta,
		Timezone:      "UTC",
		Description:   "No-match test",
		ModelTier:     "fast",
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	sdb.Close()

	cfg := &config.Config{
		Scheduler: &config.SchedulerConfig{
			Storage: config.StorageConfig{Driver: "sqlite", DSN: schedDBPath},
		},
	}

	jobsDB, err := sql.Open("sqlite", jobsDBPath)
	if err != nil {
		t.Fatalf("open jobs db: %v", err)
	}
	defer jobsDB.Close()
	jobsDB.SetMaxOpenConns(1)

	driver := riversqlite.New(jobsDB)
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if _, err := migrator.Migrate(context.Background(), rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pusher := &capturePusher{}
	worker := &jobs.ScheduledTaskWorker{
		Cfg: cfg,
		MakePusher: func(_ string, _ scheduler.ChannelMeta) (channel.Pusher, error) {
			return pusher, nil
		},
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, worker)

	riverClient, err := river.NewClient(driver, &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
		Workers: workers,
	})
	if err != nil {
		t.Fatalf("create river client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := riverClient.Start(ctx); err != nil {
		t.Fatalf("start river: %v", err)
	}
	defer riverClient.Stop(context.Background())

	notifier := daemon.NewSyncNotifier()
	sched := daemon.NewScheduler(cfg, riverClient, jobsDB, notifier)

	gmailDB, err := store.Open(store.SQLiteConfig(gmailDBPath))
	if err != nil {
		t.Fatalf("open gmail db: %v", err)
	}
	defer gmailDB.Close()

	if err := sched.CheckReactiveTriggersForTest(ctx, "gmail", gmailDB); err != nil {
		t.Fatalf("reactive check: %v", err)
	}

	// Wait and verify no message was pushed.
	time.Sleep(3 * time.Second)
	msgs := pusher.Messages()
	if len(msgs) != 0 {
		t.Errorf("expected no messages, got %d", len(msgs))
	}
}

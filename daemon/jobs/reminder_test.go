package jobs

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestReminderArgs_Kind(t *testing.T) {
	args := ReminderArgs{Message: "test reminder"}
	if args.Kind() != "reminder" {
		t.Errorf("Kind() = %q, want %q", args.Kind(), "reminder")
	}
}

func TestReminderWorker_Work(t *testing.T) {
	w := &ReminderWorker{}
	job := &river.Job[ReminderArgs]{
		JobRow: &rivertype.JobRow{},
		Args:   ReminderArgs{Message: "test reminder"},
	}

	err := w.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("ReminderWorker.Work returned error: %v", err)
	}
}

func TestGmailSyncArgs_Kind(t *testing.T) {
	args := GmailSyncArgs{}
	if args.Kind() != "gmail_sync" {
		t.Errorf("Kind() = %q, want %q", args.Kind(), "gmail_sync")
	}
}

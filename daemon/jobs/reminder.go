package jobs

import (
	"context"
	"log"

	"github.com/riverqueue/river"
)

type ReminderArgs struct {
	Message string `json:"message"`
}

func (ReminderArgs) Kind() string { return "reminder" }

type ReminderWorker struct {
	river.WorkerDefaults[ReminderArgs]
}

func (w *ReminderWorker) Work(ctx context.Context, job *river.Job[ReminderArgs]) error {
	log.Printf("reminder: %s", job.Args.Message)
	return nil
}

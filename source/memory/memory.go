package memory

import (
	"context"

	"github.com/priyanshujain/openbotkit/source"
	"github.com/priyanshujain/openbotkit/store"
)

type Memory struct {
	cfg Config
}

func New(cfg Config) *Memory {
	return &Memory{cfg: cfg}
}

func (m *Memory) Name() string {
	return "memory"
}

func (m *Memory) Status(ctx context.Context, db *store.DB) (*source.Status, error) {
	if db == nil {
		return &source.Status{Connected: false}, nil
	}

	count, _ := CountConversations(db)
	lastCapture, _ := LastCaptureTime(db)

	return &source.Status{
		Connected:    count > 0,
		ItemCount:    count,
		LastSyncedAt: lastCapture,
	}, nil
}

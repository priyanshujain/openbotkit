package telegram_read

import (
	"context"
	"testing"
	"time"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/73ai/openbotkit/spectest"
	"github.com/73ai/openbotkit/store"
	"github.com/73ai/openbotkit/usecase"
)

func skipUnlessTelegramConnected(t *testing.T) {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("could not load config: %v", err)
	}
	if !tgsrc.HasSession(cfg.TelegramSessionPath()) {
		t.Skip("Telegram not connected: run 'obk telegram auth login' first")
	}

	db, err := store.Open(store.Config{
		Driver: cfg.Telegram.Storage.Driver,
		DSN:    cfg.TelegramDataDSN(),
	})
	if err != nil {
		t.Skipf("could not open telegram db: %v", err)
	}
	defer db.Close()

	count, err := tgsrc.CountMessages(db, 0)
	if err != nil {
		t.Skipf("could not count telegram messages: %v", err)
	}
	if count == 0 {
		t.Skip("no Telegram messages synced: run 'obk telegram sync' first")
	}
}

func TestUseCase_TelegramListChats(t *testing.T) {
	fx := usecase.NewFixture(t)
	skipUnlessTelegramConnected(t)

	skill := fx.LoadSkillContent(t, "telegram-read")
	a := fx.Agent(t, skill)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	result, err := a.Run(ctx, "Show me my 5 most recently active Telegram chats")
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}

	spectest.AssertNotEmpty(t, result)
	fx.AssertJudge(t, "Show me my 5 most recently active Telegram chats", result,
		"The response should list actual Telegram chats by name or title. It should not say it cannot access Telegram.")
}

func TestUseCase_TelegramSearchMessages(t *testing.T) {
	fx := usecase.NewFixture(t)
	skipUnlessTelegramConnected(t)

	skill := fx.LoadSkillContent(t, "telegram-read")
	a := fx.Agent(t, skill)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	result, err := a.Run(ctx, "Show me the 10 most recent Telegram messages I have received, with who sent each one")
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}

	spectest.AssertNotEmpty(t, result)
	fx.AssertJudge(t, "Show me the 10 most recent Telegram messages I have received, with who sent each one", result,
		"The response should contain actual Telegram message text along with sender names or chat names. It should not say it cannot access Telegram.")
}

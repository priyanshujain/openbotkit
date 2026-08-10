package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"

	"github.com/73ai/openbotkit/store"
)

// Fetcher is the seam between backfill and the network, so sync logic is
// testable without MTProto.
type Fetcher interface {
	Dialogs(ctx context.Context, fn func(context.Context, dialogs.Elem) error) error
	History(ctx context.Context, peer tg.InputPeerClass, offsetID int, fn func(context.Context, messages.Elem) error) error
}

type apiFetcher struct {
	raw *tg.Client
}

// NewAPIFetcher returns a Fetcher backed by a live MTProto client.
func NewAPIFetcher(raw *tg.Client) Fetcher {
	return &apiFetcher{raw: raw}
}

func (f *apiFetcher) Dialogs(ctx context.Context, fn func(context.Context, dialogs.Elem) error) error {
	return query.GetDialogs(f.raw).ForEach(ctx, fn)
}

func (f *apiFetcher) History(ctx context.Context, peer tg.InputPeerClass, offsetID int, fn func(context.Context, messages.Elem) error) error {
	b := query.Messages(f.raw).GetHistory(peer).BatchSize(100)
	if offsetID > 0 {
		b = b.OffsetID(offsetID)
	}
	return b.ForEach(ctx, fn)
}

// errStopChat unwinds a history iteration once we have walked far enough back.
var errStopChat = errors.New("telegram: stop iterating chat")

// SyncDialogs records every dialog plus the users and chats they reference,
// without touching message history. Backfill and login both need this first.
func SyncDialogs(ctx context.Context, f Fetcher, db *store.DB) (int, error) {
	count := 0
	err := f.Dialogs(ctx, func(ctx context.Context, elem dialogs.Elem) error {
		if err := saveEntities(db, elem.Entities); err != nil {
			return err
		}
		chat, ok := chatFromDialog(elem)
		if !ok {
			return nil
		}
		if err := UpsertChat(db, chat); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("iterate dialogs: %w", err)
	}
	return count, nil
}

// Backfill walks history for every dialog, oldest-bound by opts.Since. Progress
// is recorded per chat so an interrupted run resumes instead of restarting.
func Backfill(ctx context.Context, f Fetcher, db *store.DB, opts BackfillOptions) (*BackfillResult, error) {
	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	result := &BackfillResult{}

	selfID, _, err := LoadSelf(db)
	if err != nil {
		slog.Warn("telegram: could not read the stored account", "error", err)
	}

	var targets []backfillTarget

	err = f.Dialogs(ctx, func(ctx context.Context, elem dialogs.Elem) error {
		if err := saveEntities(db, elem.Entities); err != nil {
			return err
		}
		chat, ok := chatFromDialog(elem)
		if !ok {
			return nil
		}
		if err := UpsertChat(db, chat); err != nil {
			return err
		}
		result.Chats++
		t := backfillTarget{peer: elem.Peer, chat: chat}
		if elem.Last != nil {
			t.lastID = elem.Last.GetID()
		}
		targets = append(targets, t)
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("iterate dialogs: %w", err)
	}

	for _, t := range targets {
		n, err := backfillChat(ctx, f, db, t, selfID, opts)
		result.Messages += n
		if err != nil {
			slog.Error("telegram: backfill chat", "chat_id", t.chat.ChatID, "error", err)
			result.Errors++
		}
	}

	return result, nil
}

type backfillTarget struct {
	peer tg.InputPeerClass
	chat *Chat
	// lastID is the chat's newest message as the dialog listing reports it,
	// which tells backfill whether a catch-up pass is worth an RPC.
	lastID int
}

// backfillChat walks a chat's history in two passes: newest-first down to the
// watermark a previous run left, then older history from where that run
// stopped. getHistory only walks backwards, so without the catch-up pass
// anything newer than the watermark would never be reachable again.
func backfillChat(ctx context.Context, f Fetcher, db *store.DB, t backfillTarget, selfID int64, opts BackfillOptions) (int, error) {
	state, err := GetSyncState(db, t.chat.ChatID)
	if err != nil {
		return 0, fmt.Errorf("get sync state: %w", err)
	}
	if state == nil || opts.Full {
		state = &SyncState{ChatID: t.chat.ChatID}
	}

	saved := 0
	var oldest time.Time

	// walk stores messages from offsetID backwards. stopAt ends the catch-up
	// pass at the watermark; trackFloor records how far back history now
	// reaches, which only the older-history pass extends.
	walk := func(offsetID, stopAt int, trackFloor bool) error {
		err := f.History(ctx, t.peer, offsetID, func(ctx context.Context, elem messages.Elem) error {
			if err := saveEntities(db, elem.Entities); err != nil {
				return err
			}

			msg, ok := messageFromTG(elem.Msg, t.chat.ChatID, selfID)
			if !ok {
				return nil
			}
			if stopAt > 0 && msg.MessageID <= stopAt {
				return errStopChat
			}
			if !opts.Since.IsZero() && msg.Timestamp.Before(opts.Since) {
				return errStopChat
			}
			msg.SenderName = resolveSenderName(db, msg.SenderID)

			if err := SaveMessage(db, msg); err != nil {
				return fmt.Errorf("save message %d: %w", msg.MessageID, err)
			}
			saved++
			if trackFloor {
				oldest = msg.Timestamp
			}

			if state.MinID == 0 || msg.MessageID < state.MinID {
				state.MinID = msg.MessageID
			}
			if msg.MessageID > state.MaxID {
				state.MaxID = msg.MessageID
			}

			if opts.PerChatLimit > 0 && saved >= opts.PerChatLimit {
				return errStopChat
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopChat) {
			return fmt.Errorf("iterate history: %w", err)
		}
		return nil
	}

	resumeFrom, watermark := state.MinID, state.MaxID

	var iterErr error
	// The dialog listing already told us the newest message, so a chat with
	// nothing new costs no extra request.
	if watermark > 0 && t.lastID > watermark {
		iterErr = walk(0, watermark, false)
	}
	if iterErr == nil && (opts.PerChatLimit <= 0 || saved < opts.PerChatLimit) {
		iterErr = walk(resumeFrom, 0, true)
	}

	if saved > 0 {
		if !oldest.IsZero() {
			state.BackfilledUntil = &oldest
		}
		if err := SaveSyncState(db, state); err != nil {
			return saved, fmt.Errorf("save sync state: %w", err)
		}
	}
	return saved, iterErr
}

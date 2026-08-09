package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"

	"github.com/73ai/openbotkit/store"
)

// LiveOptions configures the continuous update stream.
type LiveOptions struct {
	// OnChange fires after rows are actually written, so the daemon can notify
	// reactive triggers on real data rather than on a blind ticker.
	OnChange func()
}

// Live holds a connection open and writes updates straight through to the
// store, until ctx is cancelled. Gap recovery is handled by gotd's update
// manager, backed by our StateStorage.
func Live(ctx context.Context, client *Client, db *store.DB, opts LiveOptions) error {
	if err := Migrate(db); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	registerUpdateHandlers(client.Dispatcher(), db, opts.OnChange)

	return client.Run(ctx, func(ctx context.Context) error {
		self, err := client.TG().Self(ctx)
		if err != nil {
			return fmt.Errorf("get self: %w", err)
		}
		if err := SaveSelf(db, self.ID, self.Username); err != nil {
			slog.Warn("telegram: could not record account", "error", err)
		}
		if err := UpsertUser(db, userFromTG(self)); err != nil {
			slog.Warn("telegram: could not store account user", "error", err)
		}

		return client.Gaps().Run(ctx, client.API(), self.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				slog.Info("telegram: live sync started", "user_id", self.ID)
			},
		})
	})
}

func registerUpdateHandlers(d tg.UpdateDispatcher, db *store.DB, onChange func()) {
	notify := func(changed bool) {
		if changed && onChange != nil {
			onChange()
		}
	}

	d.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		notify(storeUpdateMessage(db, e, u.Message))
		return nil
	})

	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		notify(storeUpdateMessage(db, e, u.Message))
		return nil
	})

	d.OnEditMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditMessage) error {
		notify(storeUpdateMessage(db, e, u.Message))
		return nil
	})

	d.OnEditChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditChannelMessage) error {
		notify(storeUpdateMessage(db, e, u.Message))
		return nil
	})

	d.OnDeleteMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteMessages) error {
		if err := DeleteMessagesNonChannel(db, u.Messages); err != nil {
			slog.Error("telegram: delete messages", "error", err)
			return nil
		}
		notify(len(u.Messages) > 0)
		return nil
	})

	d.OnDeleteChannelMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteChannelMessages) error {
		if err := DeleteMessages(db, ChatIDFromChannel(u.ChannelID), u.Messages); err != nil {
			slog.Error("telegram: delete channel messages", "error", err)
			return nil
		}
		notify(len(u.Messages) > 0)
		return nil
	})
}

// storeUpdateMessage writes an update's message and the entities it references.
// It reports whether a row was written.
func storeUpdateMessage(db *store.DB, e tg.Entities, raw tg.MessageClass) bool {
	ents := peer.EntitiesFromUpdate(e)
	if err := saveEntities(db, ents); err != nil {
		slog.Error("telegram: save entities", "error", err)
	}

	notEmpty, ok := raw.AsNotEmpty()
	if !ok {
		return false
	}
	chatID, ok := ChatIDFromPeer(notEmpty.GetPeerID())
	if !ok {
		return false
	}

	msg, ok := messageFromTG(notEmpty, chatID)
	if !ok {
		return false
	}
	msg.SenderName = resolveSenderName(db, msg.SenderID)

	if err := SaveMessage(db, msg); err != nil {
		slog.Error("telegram: save message", "chat_id", chatID, "message_id", msg.MessageID, "error", err)
		return false
	}

	chat := chatFromEntities(chatID, ents)
	chat.LastMessageAt = &msg.Timestamp
	if err := UpsertChat(db, chat); err != nil {
		slog.Error("telegram: upsert chat", "chat_id", chatID, "error", err)
	}
	return true
}

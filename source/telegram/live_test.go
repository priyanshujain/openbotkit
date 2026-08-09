package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/gotd/td/tg"
)

func TestLiveNewMessageIsStored(t *testing.T) {
	db := testDB(t)
	changes := 0
	d := tg.NewUpdateDispatcher()
	registerUpdateHandlers(d, db, func() { changes++ })

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, AccessHash: 1111, FirstName: "Ann", LastName: "Lee"}
	msg := &tg.Message{ID: 5, Date: int(now.Unix()), Message: "hello there"}
	msg.PeerID = &tg.PeerUser{UserID: ann.ID}
	msg.SetFromID(&tg.PeerUser{UserID: ann.ID})

	err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateNewMessage{Message: msg}},
		Users:   []tg.UserClass{ann},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	msgs, err := ListMessages(db, ListOptions{ChatID: ChatIDFromUser(11)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 stored message, got %d", len(msgs))
	}
	if msgs[0].Text != "hello there" {
		t.Fatalf("text = %q", msgs[0].Text)
	}
	if msgs[0].SenderName != "Ann Lee" {
		t.Fatalf("sender_name = %q", msgs[0].SenderName)
	}

	chat, err := GetChat(db, ChatIDFromUser(11))
	if err != nil || chat == nil {
		t.Fatalf("chat missing: %v", err)
	}
	if chat.AccessHash != 1111 {
		t.Fatalf("access_hash = %d", chat.AccessHash)
	}
	if chat.LastMessageAt == nil {
		t.Fatal("last_message_at not advanced by the new message")
	}

	// The notifier fires on real data, not on a ticker.
	if changes != 1 {
		t.Fatalf("change callback fired %d times, want 1", changes)
	}
}

func TestLiveChannelMessageIsStored(t *testing.T) {
	db := testDB(t)
	d := tg.NewUpdateDispatcher()
	registerUpdateHandlers(d, db, nil)

	now := time.Now().UTC().Truncate(time.Second)
	channel := &tg.Channel{ID: 41, AccessHash: 4141, Title: "Eng"}
	msg := &tg.Message{ID: 9, Date: int(now.Unix()), Message: "shipped"}
	msg.PeerID = &tg.PeerChannel{ChannelID: channel.ID}

	err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateNewChannelMessage{Message: msg}},
		Chats:   []tg.ChatClass{channel},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	chatID := ChatIDFromChannel(41)
	if n, _ := CountMessages(db, chatID); n != 1 {
		t.Fatalf("stored %d channel messages, want 1", n)
	}
	chat, _ := GetChat(db, chatID)
	if chat == nil || chat.Title != "Eng" || chat.AccessHash != 4141 {
		t.Fatalf("channel chat = %+v", chat)
	}
}

func TestLiveEditUpdatesExistingRow(t *testing.T) {
	db := testDB(t)
	d := tg.NewUpdateDispatcher()
	registerUpdateHandlers(d, db, nil)

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, FirstName: "Ann"}

	orig := &tg.Message{ID: 5, Date: int(now.Unix()), Message: "typo"}
	orig.PeerID = &tg.PeerUser{UserID: ann.ID}
	if err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateNewMessage{Message: orig}},
		Users:   []tg.UserClass{ann},
	}); err != nil {
		t.Fatalf("handle new: %v", err)
	}

	edited := &tg.Message{ID: 5, Date: int(now.Unix()), Message: "fixed"}
	edited.PeerID = &tg.PeerUser{UserID: ann.ID}
	edited.SetEditDate(int(now.Add(time.Minute).Unix()))
	if err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateEditMessage{Message: edited}},
		Users:   []tg.UserClass{ann},
	}); err != nil {
		t.Fatalf("handle edit: %v", err)
	}

	msgs, err := ListMessages(db, ListOptions{ChatID: ChatIDFromUser(11)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("edit should update in place, got %d rows", len(msgs))
	}
	if msgs[0].Text != "fixed" {
		t.Fatalf("text = %q, want \"fixed\"", msgs[0].Text)
	}
	if msgs[0].EditDate == nil {
		t.Fatal("edit_date not recorded")
	}
}

func TestLiveDeleteRemovesRows(t *testing.T) {
	db := testDB(t)
	d := tg.NewUpdateDispatcher()
	registerUpdateHandlers(d, db, nil)

	now := time.Now().UTC()
	userChat := ChatIDFromUser(11)
	channelChat := ChatIDFromChannel(41)
	for _, m := range []*Message{
		{MessageID: 5, ChatID: userChat, Text: "dm", Timestamp: now},
		{MessageID: 5, ChatID: channelChat, Text: "post", Timestamp: now},
	} {
		if err := SaveMessage(db, m); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateDeleteMessages{Messages: []int{5}}},
	}); err != nil {
		t.Fatalf("handle delete: %v", err)
	}
	if exists, _ := MessageExists(db, 5, userChat); exists {
		t.Fatal("dm should be deleted")
	}
	if exists, _ := MessageExists(db, 5, channelChat); !exists {
		t.Fatal("channel post has its own ID sequence and must survive")
	}

	if err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateDeleteChannelMessages{ChannelID: 41, Messages: []int{5}}},
	}); err != nil {
		t.Fatalf("handle channel delete: %v", err)
	}
	if exists, _ := MessageExists(db, 5, channelChat); exists {
		t.Fatal("channel post should be deleted")
	}
}

func TestLiveIgnoresServiceMessages(t *testing.T) {
	db := testDB(t)
	changes := 0
	d := tg.NewUpdateDispatcher()
	registerUpdateHandlers(d, db, func() { changes++ })

	svc := &tg.MessageService{ID: 3, Date: int(time.Now().Unix())}
	svc.PeerID = &tg.PeerUser{UserID: 11}

	if err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{&tg.UpdateNewMessage{Message: svc}},
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if n, _ := CountMessages(db, 0); n != 0 {
		t.Fatalf("service messages should not be stored, got %d rows", n)
	}
	if changes != 0 {
		t.Fatal("service messages should not trigger the change callback")
	}
}

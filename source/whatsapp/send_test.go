package whatsapp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func TestSendText_InvalidJID(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tmpDir := t.TempDir()
	ctx := context.Background()
	client, err := NewClient(ctx, filepath.Join(tmpDir, "session.db"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = SendText(ctx, client, db, SendInput{
		ChatJID: "",
		Text:    "hello",
	})
	if err == nil {
		t.Fatal("expected error for empty JID")
	}
}

func TestSendText_SentMessageDBFields(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Simulate the DB operations that SendText performs after a successful send.
	chatJID := "1234567890@s.whatsapp.net"
	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	msg := &Message{
		MessageID: "sent-msg-001",
		ChatJID:   chatJID,
		SenderJID: "myphone@s.whatsapp.net",
		Text:      "Hello from test",
		Timestamp: ts,
		IsGroup:   false,
		IsFromMe:  true,
	}
	if err := SaveMessage(db, msg); err != nil {
		t.Fatalf("save sent message: %v", err)
	}
	UpsertChat(db, chatJID, "", false)

	// Verify message was saved.
	exists, err := MessageExists(db, "sent-msg-001", chatJID)
	if err != nil {
		t.Fatalf("check exists: %v", err)
	}
	if !exists {
		t.Fatal("sent message not found in database")
	}

	// Verify IsFromMe is set.
	messages, err := ListMessages(db, ListOptions{ChatJID: chatJID, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if !messages[0].IsFromMe {
		t.Fatal("expected IsFromMe=true for sent message")
	}
	if messages[0].Text != "Hello from test" {
		t.Fatalf("expected text 'Hello from test', got %q", messages[0].Text)
	}
	if messages[0].SenderJID != "myphone@s.whatsapp.net" {
		t.Fatalf("expected sender JID 'myphone@s.whatsapp.net', got %q", messages[0].SenderJID)
	}

	// Verify chat was upserted.
	chats, err := ListChats(db)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].JID != chatJID {
		t.Fatalf("expected chat JID %q, got %q", chatJID, chats[0].JID)
	}
}

func TestSendText_GroupDetection(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ts := time.Now()

	// Individual chat.
	individual := &Message{
		MessageID: "ind-001",
		ChatJID:   "1234@s.whatsapp.net",
		Text:      "individual msg",
		Timestamp: ts,
		IsGroup:   false,
		IsFromMe:  true,
	}
	SaveMessage(db, individual)

	// Group chat.
	group := &Message{
		MessageID: "grp-001",
		ChatJID:   "120363001234567890@g.us",
		Text:      "group msg",
		Timestamp: ts,
		IsGroup:   true,
		IsFromMe:  true,
	}
	SaveMessage(db, group)

	// Verify individual.
	msgs, _ := ListMessages(db, ListOptions{ChatJID: "1234@s.whatsapp.net", Limit: 10})
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	if msgs[0].IsGroup {
		t.Fatal("expected IsGroup=false for individual chat")
	}

	// Verify group.
	msgs, _ = ListMessages(db, ListOptions{ChatJID: "120363001234567890@g.us", Limit: 10})
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	if !msgs[0].IsGroup {
		t.Fatal("expected IsGroup=true for group chat")
	}
}

func TestSendText_DuplicateSendUpserts(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	chatJID := "1234@s.whatsapp.net"
	msg := &Message{
		MessageID: "dup-001",
		ChatJID:   chatJID,
		Text:      "first send",
		Timestamp: time.Now(),
		IsFromMe:  true,
	}

	if err := SaveMessage(db, msg); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Re-save with same message ID (simulates retry).
	msg.Text = "retried send"
	if err := SaveMessage(db, msg); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	count, _ := CountMessages(db, chatJID)
	if count != 1 {
		t.Fatalf("expected 1 message after upsert, got %d", count)
	}

	msgs, _ := ListMessages(db, ListOptions{ChatJID: chatJID, Limit: 10})
	if msgs[0].Text != "retried send" {
		t.Fatalf("expected upserted text 'retried send', got %q", msgs[0].Text)
	}
}

func TestSendText_JIDParsing(t *testing.T) {
	tests := []struct {
		name    string
		jid     string
		wantErr bool
	}{
		{"valid individual", "1234567890@s.whatsapp.net", false},
		{"valid group", "120363001234567890@g.us", false},
		{"empty string", "", false}, // whatsmeow accepts empty, returns empty JID
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := types.ParseJID(tt.jid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseJID(%q) error = %v, wantErr = %v", tt.jid, err, tt.wantErr)
			}
		})
	}
}

func TestSendText_ChatUpsertPreservesName(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	chatJID := "1234@s.whatsapp.net"

	// First, the chat exists with a name from sync.
	UpsertChat(db, chatJID, "Alice", false)

	// SendText upserts with empty name (it doesn't know the contact name).
	UpsertChat(db, chatJID, "", false)

	chats, _ := ListChats(db)
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].Name != "Alice" {
		t.Fatalf("expected name 'Alice' preserved, got %q", chats[0].Name)
	}
}

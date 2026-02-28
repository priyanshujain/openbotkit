package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"google.golang.org/protobuf/proto"
)

func TestParseHistoryMessage(t *testing.T) {
	webMsg := &waWeb.WebMessageInfo{
		Key: &waCommon.MessageKey{
			RemoteJID: proto.String("123@s.whatsapp.net"),
			FromMe:    proto.Bool(false),
			ID:        proto.String("hist-msg-001"),
		},
		Message: &waE2E.Message{
			Conversation: proto.String("history message text"),
		},
		MessageTimestamp: proto.Uint64(uint64(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC).Unix())),
		PushName:         proto.String("Bob"),
	}

	msg := parseHistoryMessage(webMsg, "123@s.whatsapp.net", false)
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if msg.MessageID != "hist-msg-001" {
		t.Fatalf("expected 'hist-msg-001', got %q", msg.MessageID)
	}
	if msg.ChatJID != "123@s.whatsapp.net" {
		t.Fatalf("expected chat jid, got %q", msg.ChatJID)
	}
	if msg.Text != "history message text" {
		t.Fatalf("expected text, got %q", msg.Text)
	}
	if msg.SenderName != "Bob" {
		t.Fatalf("expected 'Bob', got %q", msg.SenderName)
	}
	if msg.IsFromMe {
		t.Fatal("expected IsFromMe=false")
	}
}

func TestParseHistoryMessageGroupWithParticipant(t *testing.T) {
	webMsg := &waWeb.WebMessageInfo{
		Key: &waCommon.MessageKey{
			RemoteJID:   proto.String("group@g.us"),
			FromMe:      proto.Bool(false),
			ID:          proto.String("grp-msg-001"),
			Participant: proto.String("456@s.whatsapp.net"),
		},
		Message: &waE2E.Message{
			Conversation: proto.String("group message"),
		},
		MessageTimestamp: proto.Uint64(1700000000),
	}

	msg := parseHistoryMessage(webMsg, "group@g.us", true)
	if msg == nil {
		t.Fatal("expected non-nil")
	}
	if msg.SenderJID != "456@s.whatsapp.net" {
		t.Fatalf("expected participant JID as sender, got %q", msg.SenderJID)
	}
	if !msg.IsGroup {
		t.Fatal("expected IsGroup=true")
	}
}

func TestParseHistoryMessageNilMessage(t *testing.T) {
	msg := parseHistoryMessage(nil, "chat", false)
	if msg != nil {
		t.Fatal("expected nil for nil WebMessageInfo")
	}

	webMsg := &waWeb.WebMessageInfo{
		Key: &waCommon.MessageKey{
			ID: proto.String("test"),
		},
	}
	msg = parseHistoryMessage(webMsg, "chat", false)
	if msg != nil {
		t.Fatal("expected nil for nil inner message")
	}
}

func TestParseHistoryMessageEmptyContent(t *testing.T) {
	webMsg := &waWeb.WebMessageInfo{
		Key: &waCommon.MessageKey{
			RemoteJID: proto.String("123@s.whatsapp.net"),
			ID:        proto.String("empty-msg"),
		},
		Message:          &waE2E.Message{},
		MessageTimestamp: proto.Uint64(1700000000),
	}

	msg := parseHistoryMessage(webMsg, "123@s.whatsapp.net", false)
	if msg != nil {
		t.Fatal("expected nil for empty message content")
	}
}

func TestFullSyncFlowIntegration(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Simulate history sync: save messages and chats.
	chatJID := "test-chat@s.whatsapp.net"
	UpsertChat(db, chatJID, "Test Chat", false)

	now := time.Now()
	messages := []*Message{
		{MessageID: "m1", ChatJID: chatJID, SenderJID: "sender1@s.whatsapp.net", SenderName: "Alice", Text: "Hello from history", Timestamp: now.Add(-2 * time.Hour)},
		{MessageID: "m2", ChatJID: chatJID, SenderJID: "sender2@s.whatsapp.net", SenderName: "Bob", Text: "Reply from history", Timestamp: now.Add(-1 * time.Hour)},
		{MessageID: "m3", ChatJID: chatJID, SenderJID: "sender1@s.whatsapp.net", SenderName: "Alice", Text: "Another message", Timestamp: now},
	}
	for _, m := range messages {
		if err := SaveMessage(db, m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	// Verify count.
	count, err := CountMessages(db, chatJID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 messages, got %d", count)
	}

	// Verify list with chat filter.
	listed, err := ListMessages(db, ListOptions{ChatJID: chatJID, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3, got %d", len(listed))
	}
	// Should be ordered by timestamp DESC.
	if listed[0].MessageID != "m3" {
		t.Fatalf("expected m3 first (most recent), got %q", listed[0].MessageID)
	}

	// Verify search.
	results, err := SearchMessages(db, "reply", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].MessageID != "m2" {
		t.Fatalf("expected m2, got %q", results[0].MessageID)
	}

	// Verify chats.
	chats, err := ListChats(db)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].Name != "Test Chat" {
		t.Fatalf("expected 'Test Chat', got %q", chats[0].Name)
	}

	// Verify last sync time.
	lastSync, err := LastSyncTime(db)
	if err != nil {
		t.Fatalf("last sync: %v", err)
	}
	if lastSync == nil {
		t.Fatal("expected non-nil last sync time")
	}

	// Verify upsert doesn't duplicate.
	if err := SaveMessage(db, messages[0]); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	count, _ = CountMessages(db, chatJID)
	if count != 3 {
		t.Fatalf("expected 3 after re-save, got %d", count)
	}
}

package whatsapp

import (
	"testing"
	"time"
)

func TestSaveMessage(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	msg := &Message{
		MessageID:  "msg-001",
		ChatJID:    "123@s.whatsapp.net",
		SenderJID:  "456@s.whatsapp.net",
		SenderName: "Alice",
		Text:       "Hello world",
		Timestamp:  time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
		IsGroup:    false,
		IsFromMe:   false,
	}

	if err := SaveMessage(db, msg); err != nil {
		t.Fatalf("save message: %v", err)
	}

	// Verify it was inserted.
	exists, err := MessageExists(db, "msg-001", "123@s.whatsapp.net")
	if err != nil {
		t.Fatalf("message exists: %v", err)
	}
	if !exists {
		t.Fatal("expected message to exist")
	}
}

func TestSaveMessageUpsert(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	msg := &Message{
		MessageID:  "msg-001",
		ChatJID:    "123@s.whatsapp.net",
		SenderJID:  "456@s.whatsapp.net",
		SenderName: "Alice",
		Text:       "original text",
		Timestamp:  time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
	}

	if err := SaveMessage(db, msg); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Upsert with updated text.
	msg.Text = "updated text"
	if err := SaveMessage(db, msg); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Verify only one row exists.
	count, err := CountMessages(db, "123@s.whatsapp.net")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message, got %d", count)
	}

	// Verify the text was updated.
	messages, err := ListMessages(db, ListOptions{ChatJID: "123@s.whatsapp.net", Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1, got %d", len(messages))
	}
	if messages[0].Text != "updated text" {
		t.Fatalf("expected 'updated text', got %q", messages[0].Text)
	}
}

func TestUpsertChat(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := UpsertChat(db, "123@g.us", "Test Group", true); err != nil {
		t.Fatalf("upsert chat: %v", err)
	}

	chats, err := ListChats(db)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].JID != "123@g.us" {
		t.Fatalf("expected JID '123@g.us', got %q", chats[0].JID)
	}
	if chats[0].Name != "Test Group" {
		t.Fatalf("expected name 'Test Group', got %q", chats[0].Name)
	}
	if !chats[0].IsGroup {
		t.Fatal("expected is_group=true")
	}

	// Upsert again — should not duplicate.
	if err := UpsertChat(db, "123@g.us", "Renamed Group", true); err != nil {
		t.Fatalf("upsert chat again: %v", err)
	}
	chats, err = ListChats(db)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat after upsert, got %d", len(chats))
	}
	if chats[0].Name != "Renamed Group" {
		t.Fatalf("expected name 'Renamed Group', got %q", chats[0].Name)
	}
}

func TestUpsertChatPreservesNameWhenEmpty(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := UpsertChat(db, "123@g.us", "Test Group", true); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Upsert with empty name should preserve existing.
	if err := UpsertChat(db, "123@g.us", "", true); err != nil {
		t.Fatalf("upsert empty name: %v", err)
	}
	chats, err := ListChats(db)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if chats[0].Name != "Test Group" {
		t.Fatalf("expected preserved name 'Test Group', got %q", chats[0].Name)
	}
}

func TestMessageExists(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	exists, err := MessageExists(db, "nonexistent", "chat1")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if exists {
		t.Fatal("expected false for nonexistent message")
	}

	msg := &Message{
		MessageID: "msg-001",
		ChatJID:   "chat1",
		Timestamp: time.Now(),
	}
	SaveMessage(db, msg)

	exists, err = MessageExists(db, "msg-001", "chat1")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !exists {
		t.Fatal("expected true for existing message")
	}
}

func TestListMessagesWithFilters(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	msgs := []*Message{
		{MessageID: "m1", ChatJID: "chat-a", Text: "hello", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{MessageID: "m2", ChatJID: "chat-a", Text: "world", Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{MessageID: "m3", ChatJID: "chat-b", Text: "foo", Timestamp: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, m := range msgs {
		if err := SaveMessage(db, m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	// Filter by chat.
	results, err := ListMessages(db, ListOptions{ChatJID: "chat-a", Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}

	// Filter by after date.
	results, err = ListMessages(db, ListOptions{After: "2026-01-15", Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 after Jan 15, got %d", len(results))
	}

	// Limit.
	results, err = ListMessages(db, ListOptions{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 with limit, got %d", len(results))
	}
}

func TestSearchMessages(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	msgs := []*Message{
		{MessageID: "m1", ChatJID: "c1", Text: "hello world", Timestamp: time.Now()},
		{MessageID: "m2", ChatJID: "c1", Text: "goodbye moon", Timestamp: time.Now()},
		{MessageID: "m3", ChatJID: "c1", Text: "Hello Again", Timestamp: time.Now()},
	}
	for _, m := range msgs {
		SaveMessage(db, m)
	}

	// Case-insensitive search.
	results, err := SearchMessages(db, "hello", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 matches for 'hello', got %d", len(results))
	}

	// No match.
	results, err = SearchMessages(db, "nonexistent", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(results))
	}
}

func TestCountMessages(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Empty.
	count, err := CountMessages(db, "")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	SaveMessage(db, &Message{MessageID: "m1", ChatJID: "c1", Timestamp: time.Now()})
	SaveMessage(db, &Message{MessageID: "m2", ChatJID: "c2", Timestamp: time.Now()})

	// Total count.
	count, err = CountMessages(db, "")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}

	// Filtered count.
	count, err = CountMessages(db, "c1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestLastSyncTime(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// No messages — should return nil.
	ts, err := LastSyncTime(db)
	if err != nil {
		t.Fatalf("last sync: %v", err)
	}
	if ts != nil {
		t.Fatalf("expected nil, got %v", ts)
	}

	// Add a message and check.
	SaveMessage(db, &Message{MessageID: "m1", ChatJID: "c1", Timestamp: time.Now()})

	ts, err = LastSyncTime(db)
	if err != nil {
		t.Fatalf("last sync: %v", err)
	}
	if ts == nil {
		t.Fatal("expected non-nil timestamp after saving a message")
	}
}

func TestListChatsEmpty(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	chats, err := ListChats(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(chats) != 0 {
		t.Fatalf("expected 0 chats, got %d", len(chats))
	}
}

func TestListChatsDefaultLimit(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Default limit should be 50 when 0 is passed.
	results, err := ListMessages(db, ListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Just verifying it doesn't error — result is empty.
	_ = results
}

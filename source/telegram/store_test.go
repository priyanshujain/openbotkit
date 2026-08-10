package telegram

import (
	"testing"
	"time"
)

func TestSaveMessageAndList(t *testing.T) {
	db := testDB(t)

	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	msgs := []*Message{
		{MessageID: 1, ChatID: 100, SenderID: 5, SenderName: "Ann", Text: "hello", Timestamp: base},
		{MessageID: 2, ChatID: 100, SenderID: 5, SenderName: "Ann", Text: "world", Timestamp: base.Add(time.Minute)},
		{MessageID: 1, ChatID: -200, SenderID: 6, SenderName: "Bob", Text: "group msg", Timestamp: base.Add(2 * time.Minute)},
	}
	for _, m := range msgs {
		if err := SaveMessage(db, m); err != nil {
			t.Fatalf("save %d/%d: %v", m.ChatID, m.MessageID, err)
		}
	}

	all, err := ListMessages(db, ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(all))
	}
	if all[0].Text != "group msg" {
		t.Fatalf("expected newest first, got %q", all[0].Text)
	}

	inChat, err := ListMessages(db, ListOptions{ChatID: 100})
	if err != nil {
		t.Fatalf("list by chat: %v", err)
	}
	if len(inChat) != 2 {
		t.Fatalf("expected 2 messages in chat 100, got %d", len(inChat))
	}
}

func TestSaveMessageUpsertsOnEdit(t *testing.T) {
	db := testDB(t)

	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := SaveMessage(db, &Message{MessageID: 1, ChatID: 100, Text: "before", Timestamp: ts, IsOutgoing: true}); err != nil {
		t.Fatalf("save: %v", err)
	}

	edited := ts.Add(time.Hour)
	if err := SaveMessage(db, &Message{MessageID: 1, ChatID: 100, Text: "after", Timestamp: ts, EditDate: &edited}); err != nil {
		t.Fatalf("resave: %v", err)
	}

	msgs, err := ListMessages(db, ListOptions{ChatID: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected upsert not insert, got %d rows", len(msgs))
	}
	if msgs[0].Text != "after" {
		t.Fatalf("text = %q, want \"after\"", msgs[0].Text)
	}
	if msgs[0].EditDate == nil {
		t.Fatal("edit_date not stored")
	}
	if !msgs[0].IsOutgoing {
		t.Fatal("is_outgoing should survive an edit upsert")
	}
}

func TestListMessagesDateFilters(t *testing.T) {
	db := testDB(t)

	for i, day := range []int{1, 5, 10} {
		ts := time.Date(2026, 3, day, 12, 0, 0, 0, time.UTC)
		if err := SaveMessage(db, &Message{MessageID: i + 1, ChatID: 100, Text: "m", Timestamp: ts}); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	got, err := ListMessages(db, ListOptions{After: "2026-03-03", Before: "2026-03-07"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message in range, got %d", len(got))
	}
	if got[0].MessageID != 2 {
		t.Fatalf("message_id = %d, want 2", got[0].MessageID)
	}
}

// --before is a date, and a date means the whole day. Comparing it against a
// stored timestamp as a string silently drops everything after midnight.
func TestListMessagesBeforeIncludesTheWholeDay(t *testing.T) {
	db := testDB(t)

	ts := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	if err := SaveMessage(db, &Message{MessageID: 1, ChatID: 100, Text: "noon", Timestamp: ts}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := ListMessages(db, ListOptions{Before: "2026-02-01"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the midday message to be within --before 2026-02-01, got %d", len(got))
	}

	got, err = ListMessages(db, ListOptions{After: "2026-02-01", Before: "2026-02-01"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("a single-day range should return that day, got %d", len(got))
	}

	got, err = ListMessages(db, ListOptions{Before: "2026-01-31"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("--before must stay exclusive of later days, got %d", len(got))
	}
}

func TestListMessagesRejectsBadDates(t *testing.T) {
	db := testDB(t)

	if _, err := ListMessages(db, ListOptions{After: "yesterday"}); err == nil {
		t.Fatal("expected an error for an unparseable --after")
	}
	if _, err := ListMessages(db, ListOptions{Before: "01/02/2026"}); err == nil {
		t.Fatal("expected an error for an unparseable --before")
	}
}

func TestSearchMessages(t *testing.T) {
	db := testDB(t)

	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	for i, text := range []string{"Lunch tomorrow?", "no thanks", "LUNCH is ready"} {
		if err := SaveMessage(db, &Message{MessageID: i + 1, ChatID: 100, Text: text, Timestamp: ts}); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	got, err := SearchMessages(db, "lunch", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 case-insensitive matches, got %d", len(got))
	}
}

func TestCountAndMessageExists(t *testing.T) {
	db := testDB(t)

	ts := time.Now().UTC()
	if err := SaveMessage(db, &Message{MessageID: 1, ChatID: 100, Text: "a", Timestamp: ts}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := SaveMessage(db, &Message{MessageID: 1, ChatID: 200, Text: "b", Timestamp: ts}); err != nil {
		t.Fatalf("save: %v", err)
	}

	total, err := CountMessages(db, 0)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	inChat, err := CountMessages(db, 100)
	if err != nil {
		t.Fatalf("count chat: %v", err)
	}
	if inChat != 1 {
		t.Fatalf("chat count = %d, want 1", inChat)
	}

	exists, err := MessageExists(db, 1, 100)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists {
		t.Fatal("expected message to exist")
	}

	exists, err = MessageExists(db, 99, 100)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists {
		t.Fatal("expected missing message to report false")
	}
}

func TestDeleteMessagesNonChannelSkipsChannels(t *testing.T) {
	db := testDB(t)

	ts := time.Now().UTC()
	userChat := ChatIDFromUser(500)
	channelChat := ChatIDFromChannel(500)

	if err := SaveMessage(db, &Message{MessageID: 42, ChatID: userChat, Text: "dm", Timestamp: ts}); err != nil {
		t.Fatalf("save dm: %v", err)
	}
	if err := SaveMessage(db, &Message{MessageID: 42, ChatID: channelChat, Text: "channel", Timestamp: ts}); err != nil {
		t.Fatalf("save channel: %v", err)
	}

	if err := DeleteMessagesNonChannel(db, []int{42}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if exists, _ := MessageExists(db, 42, userChat); exists {
		t.Fatal("dm message should be deleted")
	}
	if exists, _ := MessageExists(db, 42, channelChat); !exists {
		t.Fatal("channel message shares the ID but has its own sequence; it must survive")
	}
}

func TestUpsertChatPreservesKnownFields(t *testing.T) {
	db := testDB(t)

	last := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	chat := &Chat{
		ChatID: ChatIDFromChannel(9), Type: PeerChannel, Title: "Eng",
		Username: "eng", AccessHash: 4242, IsChannel: true, LastMessageAt: &last,
	}
	if err := UpsertChat(db, chat); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// A later partial upsert (e.g. from the access hasher) must not blank the title.
	if err := UpsertChat(db, &Chat{ChatID: chat.ChatID, Type: PeerChannel, AccessHash: 5151, IsChannel: true}); err != nil {
		t.Fatalf("partial upsert: %v", err)
	}

	got, err := GetChat(db, chat.ChatID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("chat not found")
	}
	if got.Title != "Eng" {
		t.Fatalf("title = %q, want Eng", got.Title)
	}
	if got.AccessHash != 5151 {
		t.Fatalf("access_hash = %d, want 5151", got.AccessHash)
	}
	if got.LastMessageAt == nil {
		t.Fatal("last_message_at cleared by partial upsert")
	}
}

// The update manager refreshes access hashes with nothing but an ID, so the
// refresh must not reset the flags that say what kind of chat this is.
func TestSetChatAccessHashKeepsIdentity(t *testing.T) {
	db := testDB(t)

	last := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	supergroup := &Chat{
		ChatID: ChatIDFromChannel(9), Type: PeerChannel, Title: "My Supergroup",
		Username: "mysuper", AccessHash: 4242, IsGroup: true, IsChannel: true,
		LastMessageAt: &last,
	}
	if err := UpsertChat(db, supergroup); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := SetChatAccessHash(db, supergroup.ChatID, 5151); err != nil {
		t.Fatalf("set access hash: %v", err)
	}

	got, err := GetChat(db, supergroup.ChatID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.AccessHash != 5151 {
		t.Fatalf("access_hash = %d, want 5151", got.AccessHash)
	}
	if !got.IsGroup || !got.IsChannel {
		t.Fatalf("a supergroup must keep both flags after a hash refresh: %+v", got)
	}
	if got.Title != "My Supergroup" || got.Username != "mysuper" {
		t.Fatalf("chat = %+v", got)
	}
	if got.LastMessageAt == nil {
		t.Fatal("last_message_at cleared by a hash refresh")
	}
}

// An unknown channel still needs a row, or the hash is lost and sending fails.
func TestSetChatAccessHashCreatesMissingRow(t *testing.T) {
	db := testDB(t)

	chatID := ChatIDFromChannel(77)
	if err := SetChatAccessHash(db, chatID, 999); err != nil {
		t.Fatalf("set access hash: %v", err)
	}

	got, err := GetChat(db, chatID)
	if err != nil || got == nil {
		t.Fatalf("chat row missing: %v", err)
	}
	if got.AccessHash != 999 || got.Type != PeerChannel || !got.IsChannel {
		t.Fatalf("chat = %+v", got)
	}
}

func TestSetUserAccessHashKeepsIdentity(t *testing.T) {
	db := testDB(t)

	bot := &User{UserID: 7, Username: "somebot", FirstName: "Some Bot", AccessHash: 111, IsBot: true}
	if err := UpsertUser(db, bot); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := SetUserAccessHash(db, bot.UserID, 222); err != nil {
		t.Fatalf("set access hash: %v", err)
	}

	got, err := GetUser(db, bot.UserID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.AccessHash != 222 {
		t.Fatalf("access_hash = %d, want 222", got.AccessHash)
	}
	if !got.IsBot {
		t.Fatalf("is_bot reset by a hash refresh: %+v", got)
	}
	if got.Username != "somebot" || got.FirstName != "Some Bot" {
		t.Fatalf("user = %+v", got)
	}
}

func TestTouchChatLastMessageKeepsIdentity(t *testing.T) {
	db := testDB(t)

	last := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	chat := &Chat{
		ChatID: ChatIDFromChannel(9), Type: PeerChannel, Title: "My Supergroup",
		AccessHash: 4242, IsGroup: true, IsChannel: true, LastMessageAt: &last,
	}
	if err := UpsertChat(db, chat); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	newer := last.Add(time.Hour)
	if err := TouchChatLastMessage(db, chat.ChatID, newer); err != nil {
		t.Fatalf("touch: %v", err)
	}

	got, err := GetChat(db, chat.ChatID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if !got.IsGroup || !got.IsChannel || got.Title != "My Supergroup" || got.AccessHash != 4242 {
		t.Fatalf("chat = %+v", got)
	}
	if got.LastMessageAt == nil || !got.LastMessageAt.Equal(newer) {
		t.Fatalf("last_message_at = %v, want %v", got.LastMessageAt, newer)
	}
}

func TestGetChatMissing(t *testing.T) {
	db := testDB(t)
	got, err := GetChat(db, 12345)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing chat, got %+v", got)
	}
}

func TestFindChats(t *testing.T) {
	db := testDB(t)

	now := time.Now().UTC()
	chats := []*Chat{
		{ChatID: 1, Type: PeerUser, Title: "Alice Smith", LastMessageAt: &now},
		{ChatID: -2, Type: PeerGroup, Title: "Family", IsGroup: true, LastMessageAt: &now},
		{ChatID: 3, Type: PeerUser, Title: "Bob", Username: "alicefan", LastMessageAt: &now},
	}
	for _, c := range chats {
		if err := UpsertChat(db, c); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	got, err := FindChats(db, "alice", 10)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected title and username matches, got %d", len(got))
	}
}

func TestUpsertUserAndDisplayName(t *testing.T) {
	db := testDB(t)

	u := &User{UserID: 7, Username: "ann", FirstName: "Ann", LastName: "Lee", AccessHash: 111}
	if err := UpsertUser(db, u); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Access-hash-only upsert must not wipe the name.
	if err := UpsertUser(db, &User{UserID: 7, AccessHash: 222}); err != nil {
		t.Fatalf("partial upsert: %v", err)
	}

	got, err := GetUser(db, 7)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("user not found")
	}
	if got.FirstName != "Ann" || got.LastName != "Lee" {
		t.Fatalf("name = %q %q", got.FirstName, got.LastName)
	}
	if got.AccessHash != 222 {
		t.Fatalf("access_hash = %d, want 222", got.AccessHash)
	}
	if got.DisplayName() != "Ann Lee" {
		t.Fatalf("display name = %q", got.DisplayName())
	}

	anon := User{UserID: 8, Username: "ghost"}
	if anon.DisplayName() != "@ghost" {
		t.Fatalf("username fallback = %q", anon.DisplayName())
	}
}

func TestListUsersFilter(t *testing.T) {
	db := testDB(t)

	users := []*User{
		{UserID: 1, Username: "ann", FirstName: "Ann"},
		{UserID: 2, Username: "bob", FirstName: "Bob"},
		{UserID: 3, Username: "carol", FirstName: "Carol", Phone: "919876543210"},
	}
	for _, u := range users {
		if err := UpsertUser(db, u); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	all, err := ListUsers(db, "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 users, got %d", len(all))
	}

	byName, err := ListUsers(db, "bob", 10)
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(byName) != 1 || byName[0].UserID != 2 {
		t.Fatalf("expected only Bob, got %+v", byName)
	}

	byPhone, err := ListUsers(db, "9876", 10)
	if err != nil {
		t.Fatalf("list by phone: %v", err)
	}
	if len(byPhone) != 1 || byPhone[0].UserID != 3 {
		t.Fatalf("expected only Carol, got %+v", byPhone)
	}
}

func TestSyncStateRoundTrip(t *testing.T) {
	db := testDB(t)

	if got, err := GetSyncState(db, 100); err != nil || got != nil {
		t.Fatalf("expected no state, got %+v (err %v)", got, err)
	}

	until := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := SaveSyncState(db, &SyncState{ChatID: 100, MinID: 5, MaxID: 90, BackfilledUntil: &until}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := SaveSyncState(db, &SyncState{ChatID: 100, MinID: 2, MaxID: 95, BackfilledUntil: &until}); err != nil {
		t.Fatalf("resave: %v", err)
	}

	got, err := GetSyncState(db, 100)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.MinID != 2 || got.MaxID != 95 {
		t.Fatalf("state = %+v", got)
	}
	if got.BackfilledUntil == nil {
		t.Fatal("backfilled_until not stored")
	}
}

func TestKVRoundTrip(t *testing.T) {
	db := testDB(t)

	if _, found, err := GetKV(db, "missing"); err != nil || found {
		t.Fatalf("expected miss, found=%v err=%v", found, err)
	}

	if err := SetKV(db, "state:1:pts", "10"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := SetKV(db, "state:1:pts", "11"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	value, found, err := GetKV(db, "state:1:pts")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if value != "11" {
		t.Fatalf("value = %q, want 11", value)
	}

	if err := SetKV(db, "channel_pts:1:500", "7"); err != nil {
		t.Fatalf("set channel: %v", err)
	}
	if err := SetKV(db, "channel_pts:1:600", "8"); err != nil {
		t.Fatalf("set channel: %v", err)
	}
	if err := SetKV(db, "channel_pts:2:700", "9"); err != nil {
		t.Fatalf("set channel: %v", err)
	}

	got, err := ListKVPrefix(db, "channel_pts:1:")
	if err != nil {
		t.Fatalf("list prefix: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 keys for user 1, got %d (%v)", len(got), got)
	}
}

func TestLastSyncTime(t *testing.T) {
	db := testDB(t)

	if got, err := LastSyncTime(db); err != nil || got != nil {
		t.Fatalf("expected nil on empty table, got %v (err %v)", got, err)
	}

	if err := SaveMessage(db, &Message{MessageID: 1, ChatID: 100, Text: "a", Timestamp: time.Now().UTC()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LastSyncTime(db)
	if err != nil {
		t.Fatalf("last sync: %v", err)
	}
	if got == nil {
		t.Fatal("expected a sync time after inserting a message")
	}
}

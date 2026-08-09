package telegram

import (
	"testing"

	"github.com/73ai/openbotkit/store"
)

func testDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(store.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestMigrate(t *testing.T) {
	db := testDB(t)

	tables := []string{
		"telegram_messages",
		"telegram_chats",
		"telegram_users",
		"telegram_sync_state",
		"telegram_updates_state",
	}
	for _, table := range tables {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate should be idempotent: %v", err)
	}
}

// Telegram message IDs are per-chat, not global, so the unique key is the pair.
func TestMigrateUniqueConstraint(t *testing.T) {
	db := testDB(t)

	_, err := db.Exec(`INSERT INTO telegram_messages (message_id, chat_id, text) VALUES (7, 100, 'hello')`)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	_, err = db.Exec(`INSERT INTO telegram_messages (message_id, chat_id, text) VALUES (7, 100, 'world')`)
	if err == nil {
		t.Fatal("expected unique constraint violation on (message_id, chat_id)")
	}

	_, err = db.Exec(`INSERT INTO telegram_messages (message_id, chat_id, text) VALUES (7, 200, 'world')`)
	if err != nil {
		t.Fatalf("same message_id in a different chat should succeed: %v", err)
	}
}

func TestChatIDNormalisation(t *testing.T) {
	tests := []struct {
		name   string
		chatID int64
		kind   string
		rawID  int64
	}{
		{"user", ChatIDFromUser(12345), PeerUser, 12345},
		{"group", ChatIDFromGroup(12345), PeerGroup, 12345},
		{"channel", ChatIDFromChannel(12345), PeerChannel, 12345},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, rawID := SplitChatID(tt.chatID)
			if kind != tt.kind || rawID != tt.rawID {
				t.Fatalf("SplitChatID(%d) = (%q, %d), want (%q, %d)",
					tt.chatID, kind, rawID, tt.kind, tt.rawID)
			}
		})
	}
}

// A user, a legacy chat and a channel can share a raw ID; the normalised form
// must keep them distinct so chat_id is safe as a primary key.
func TestChatIDNoCollisionAcrossPeerKinds(t *testing.T) {
	const raw int64 = 777
	seen := map[int64]string{}
	for kind, id := range map[string]int64{
		PeerUser:    ChatIDFromUser(raw),
		PeerGroup:   ChatIDFromGroup(raw),
		PeerChannel: ChatIDFromChannel(raw),
	} {
		if other, dup := seen[id]; dup {
			t.Fatalf("%s and %s both normalise to %d", kind, other, id)
		}
		seen[id] = kind
	}
}

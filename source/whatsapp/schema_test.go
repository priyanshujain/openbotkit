package whatsapp

import (
	"testing"

	"github.com/priyanshujain/openbotkit/store"
)

func testDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(store.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrate(t *testing.T) {
	db := testDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Verify tables exist by querying them.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM whatsapp_messages").Scan(&count); err != nil {
		t.Fatalf("query messages table: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM whatsapp_chats").Scan(&count); err != nil {
		t.Fatalf("query chats table: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM whatsapp_contacts").Scan(&count); err != nil {
		t.Fatalf("query contacts table: %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := testDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate should be idempotent: %v", err)
	}
}

func TestMigrateUniqueConstraint(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Insert a message.
	_, err := db.Exec(`INSERT INTO whatsapp_messages (message_id, chat_jid, text) VALUES ('msg1', 'chat1', 'hello')`)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Same message_id + chat_jid should conflict.
	_, err = db.Exec(`INSERT INTO whatsapp_messages (message_id, chat_jid, text) VALUES ('msg1', 'chat1', 'world')`)
	if err == nil {
		t.Fatal("expected unique constraint violation, got nil")
	}

	// Different chat_jid should succeed.
	_, err = db.Exec(`INSERT INTO whatsapp_messages (message_id, chat_jid, text) VALUES ('msg1', 'chat2', 'world')`)
	if err != nil {
		t.Fatalf("different chat_jid should succeed: %v", err)
	}
}

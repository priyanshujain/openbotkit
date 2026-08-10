package telegram

import (
	"os"
	"path/filepath"
	"slices"
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

// skillOnlyInternalTables are the tables the telegram-read skill deliberately
// leaves out: gotd's pts/qts bookkeeping is plumbing, not something an agent
// should be reading or joining against.
var skillOnlyInternalTables = map[string]bool{"telegram_updates_state": true}

// The schema lives twice: here and in skills/telegram-read/schema.sql, which is
// what the agent reads before writing SQL. Nothing keeps them in step, so a
// column added on one side quietly makes the other side's queries wrong.
func TestSkillSchemaMatches(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "skills", "telegram-read", "schema.sql"))
	if err != nil {
		t.Fatalf("read skill schema: %v", err)
	}

	skillDB, err := store.Open(store.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { skillDB.Close() })
	if _, err := skillDB.Exec(string(raw)); err != nil {
		t.Fatalf("apply skill schema: %v", err)
	}

	want := tableColumns(t, testDB(t))
	got := tableColumns(t, skillDB)

	for table, columns := range want {
		if skillOnlyInternalTables[table] {
			if _, documented := got[table]; documented {
				t.Fatalf("%s is internal; drop it from the skill schema or from the exemption list", table)
			}
			continue
		}
		skillColumns, ok := got[table]
		if !ok {
			t.Fatalf("skills/telegram-read/schema.sql is missing %s", table)
		}
		if !slices.Equal(columns, skillColumns) {
			t.Fatalf("%s columns differ:\n  source: %v\n  skill:  %v", table, columns, skillColumns)
		}
	}

	for table := range got {
		if _, ok := want[table]; !ok {
			t.Fatalf("skills/telegram-read/schema.sql documents %s, which the source schema does not create", table)
		}
	}
}

// tableColumns maps each telegram table to its column names, sorted.
func tableColumns(t *testing.T, db *store.DB) map[string][]string {
	t.Helper()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'telegram_%'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		tables = append(tables, name)
	}
	rows.Close()

	out := map[string][]string{}
	for _, table := range tables {
		cols, err := db.Query("SELECT name FROM pragma_table_info('" + table + "')")
		if err != nil {
			t.Fatalf("columns of %s: %v", table, err)
		}
		var names []string
		for cols.Next() {
			var name string
			if err := cols.Scan(&name); err != nil {
				t.Fatalf("scan column: %v", err)
			}
			names = append(names, name)
		}
		cols.Close()
		slices.Sort(names)
		out[table] = names
	}
	return out
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

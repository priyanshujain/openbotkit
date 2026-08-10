package contacts

import (
	"testing"
	"time"

	gmailsrc "github.com/73ai/openbotkit/source/gmail"
	imsrc "github.com/73ai/openbotkit/source/imessage"
	wasrc "github.com/73ai/openbotkit/source/whatsapp"
	"github.com/73ai/openbotkit/store"
)

func openSourceDB(t *testing.T, migrateFn func(*store.DB) error) *store.DB {
	t.Helper()
	db, err := store.Open(store.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrateFn(db); err != nil {
		t.Fatalf("migrate source: %v", err)
	}
	return db
}

func TestSyncWhatsApp(t *testing.T) {
	contactsDB := testDB(t)
	waDB := openSourceDB(t, wasrc.Migrate)

	// Seed a WhatsApp contact.
	_, err := waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, first_name, full_name, push_name, business_name)
		VALUES ('919876543210@s.whatsapp.net', '+919876543210', 'David', 'David Chen', 'Dave', '')`)
	if err != nil {
		t.Fatalf("seed wa contact: %v", err)
	}

	// Seed a message for interaction counting.
	_, err = waDB.Exec(`INSERT INTO whatsapp_messages (message_id, chat_jid, sender_jid, text, timestamp)
		VALUES ('msg1', '919876543210@s.whatsapp.net', '919876543210@s.whatsapp.net', 'hello', ?)`, time.Now())
	if err != nil {
		t.Fatalf("seed wa message: %v", err)
	}

	result, err := syncFromWhatsApp(contactsDB, waDB)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Created != 1 {
		t.Errorf("created = %d, want 1", result.Created)
	}

	// Verify contact was created with correct identities.
	contacts, _ := ListContacts(contactsDB, 10, 0)
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}

	full, _ := GetContact(contactsDB, contacts[0].ID)
	if len(full.Identities) < 2 {
		t.Errorf("identities = %d, want >= 2 (phone + wa_jid)", len(full.Identities))
	}
	if len(full.Aliases) == 0 {
		t.Error("expected at least one alias")
	}
}

func TestSyncWhatsAppIncremental(t *testing.T) {
	contactsDB := testDB(t)
	waDB := openSourceDB(t, wasrc.Migrate)

	_, _ = waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, full_name) VALUES ('111@s.whatsapp.net', '+111', 'Alice')`)
	r1, err := syncFromWhatsApp(contactsDB, waDB)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if r1.Created != 1 {
		t.Fatalf("first sync: created = %d, want 1", r1.Created)
	}

	// Add new contact and sync again.
	_, _ = waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, full_name) VALUES ('222@s.whatsapp.net', '+222', 'Bob')`)
	r2, err := syncFromWhatsApp(contactsDB, waDB)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if r2.Created != 1 {
		t.Errorf("second sync: created = %d, want 1 (only new contact)", r2.Created)
	}

	count, _ := CountContacts(contactsDB)
	if count != 2 {
		t.Errorf("total contacts = %d, want 2", count)
	}
}

func TestSyncGmail(t *testing.T) {
	contactsDB := testDB(t)
	gmailDB := openSourceDB(t, gmailsrc.Migrate)

	_, err := gmailDB.Exec(`INSERT INTO emails (message_id, account, from_addr, to_addr, date)
		VALUES ('e1', 'me@gmail.com', 'Alice Smith <alice@example.com>', 'me@gmail.com', ?)`, time.Now())
	if err != nil {
		t.Fatalf("seed gmail: %v", err)
	}

	result, err := syncFromGmail(contactsDB, gmailDB)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	// Should create contacts for both alice@example.com and me@gmail.com.
	if result.Created < 1 {
		t.Errorf("created = %d, want >= 1", result.Created)
	}

	// Verify alice has an alias.
	results, _ := SearchContacts(contactsDB, "Alice", 10)
	if len(results) != 1 {
		t.Fatalf("search alice: got %d, want 1", len(results))
	}
}

func TestSyncGmailDedup(t *testing.T) {
	contactsDB := testDB(t)
	gmailDB := openSourceDB(t, gmailsrc.Migrate)

	for i := 0; i < 3; i++ {
		_, _ = gmailDB.Exec(`INSERT INTO emails (message_id, account, from_addr, to_addr, date)
			VALUES (?, 'me@gmail.com', 'alice@test.com', 'me@gmail.com', ?)`,
			"msg"+string(rune('0'+i)), time.Now())
	}

	result, _ := syncFromGmail(contactsDB, gmailDB)
	// alice@test.com should create only 1 contact.
	aliceCount := 0
	contacts, _ := ListContacts(contactsDB, 50, 0)
	for _, c := range contacts {
		full, _ := GetContact(contactsDB, c.ID)
		for _, id := range full.Identities {
			if id.IdentityValue == "alice@test.com" {
				aliceCount++
			}
		}
	}
	if aliceCount != 1 {
		t.Errorf("alice identity count = %d, want 1", aliceCount)
	}
	_ = result
}

func TestSyncIMessage(t *testing.T) {
	contactsDB := testDB(t)
	imDB := openSourceDB(t, imsrc.Migrate)

	_, err := imDB.Exec(`INSERT INTO imessage_handles (handle_id, service) VALUES ('+15551234567', 'iMessage')`)
	if err != nil {
		t.Fatalf("seed handle: %v", err)
	}
	_, err = imDB.Exec(`INSERT INTO imessage_handles (handle_id, service) VALUES ('bob@icloud.com', 'iMessage')`)
	if err != nil {
		t.Fatalf("seed handle: %v", err)
	}

	result, err := syncFromIMessage(contactsDB, imDB)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Created != 2 {
		t.Errorf("created = %d, want 2", result.Created)
	}

	// Verify phone handle created a phone identity.
	c, _ := FindContactByIdentity(contactsDB, "phone", "+15551234567")
	if c == nil {
		t.Error("phone contact not found")
	}

	// Verify email handle created an email identity.
	c, _ = FindContactByIdentity(contactsDB, "email", "bob@icloud.com")
	if c == nil {
		t.Error("email contact not found")
	}
}

func TestCrossSourceDedupPhone(t *testing.T) {
	contactsDB := testDB(t)
	waDB := openSourceDB(t, wasrc.Migrate)
	imDB := openSourceDB(t, imsrc.Migrate)

	// Same phone number in both WhatsApp and iMessage.
	_, _ = waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, full_name) VALUES ('15551234567@s.whatsapp.net', '+15551234567', 'Alice')`)
	_, _ = imDB.Exec(`INSERT INTO imessage_handles (handle_id, service) VALUES ('+15551234567', 'iMessage')`)

	_, _ = syncFromWhatsApp(contactsDB, waDB)
	_, _ = syncFromIMessage(contactsDB, imDB)

	// Should be ONE contact with identities from both sources.
	count, _ := CountContacts(contactsDB)
	if count != 1 {
		t.Errorf("contacts = %d, want 1 (dedup by phone)", count)
	}

	contacts, _ := ListContacts(contactsDB, 10, 0)
	full, _ := GetContact(contactsDB, contacts[0].ID)
	sources := make(map[string]bool)
	for _, id := range full.Identities {
		sources[id.Source] = true
	}
	if !sources["whatsapp"] || !sources["imessage"] {
		t.Errorf("expected both whatsapp and imessage sources, got %v", sources)
	}
}

func TestCrossSourceDedupEmail(t *testing.T) {
	contactsDB := testDB(t)
	gmailDB := openSourceDB(t, gmailsrc.Migrate)
	imDB := openSourceDB(t, imsrc.Migrate)

	// Same email in both Gmail and iMessage.
	_, _ = gmailDB.Exec(`INSERT INTO emails (message_id, account, from_addr, to_addr, date) VALUES ('e1', 'me@gmail.com', 'alice@test.com', 'me@gmail.com', ?)`, time.Now())
	_, _ = imDB.Exec(`INSERT INTO imessage_handles (handle_id, service) VALUES ('alice@test.com', 'iMessage')`)

	_, _ = syncFromGmail(contactsDB, gmailDB)
	_, _ = syncFromIMessage(contactsDB, imDB)

	// alice@test.com should be one contact.
	c, _ := FindContactByIdentity(contactsDB, "email", "alice@test.com")
	if c == nil {
		t.Fatal("alice not found")
	}

	sources := make(map[string]bool)
	for _, id := range c.Identities {
		sources[id.Source] = true
	}
	if !sources["gmail"] || !sources["imessage"] {
		t.Errorf("expected both gmail and imessage sources, got %v", sources)
	}
}

func TestNoAutoMergeByName(t *testing.T) {
	contactsDB := testDB(t)
	waDB := openSourceDB(t, wasrc.Migrate)

	// Two different "John Smith" with different phones.
	_, _ = waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, full_name) VALUES ('111@s.whatsapp.net', '+111', 'John Smith')`)
	_, _ = waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, full_name) VALUES ('222@s.whatsapp.net', '+222', 'John Smith')`)

	_, _ = syncFromWhatsApp(contactsDB, waDB)

	count, _ := CountContacts(contactsDB)
	if count != 2 {
		t.Errorf("contacts = %d, want 2 (no auto-merge by name)", count)
	}
}

func TestSyncOrchestrator(t *testing.T) {
	contactsDB := testDB(t)
	waDB := openSourceDB(t, wasrc.Migrate)

	_, _ = waDB.Exec(`INSERT INTO whatsapp_contacts (jid, phone, full_name) VALUES ('111@s.whatsapp.net', '+111', 'Test')`)

	sourceDBs := map[string]*store.DB{"whatsapp": waDB}
	result, err := Sync(contactsDB, sourceDBs, SyncOptions{Sources: []string{"whatsapp"}})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Created != 1 {
		t.Errorf("created = %d, want 1", result.Created)
	}

	// Verify sync state was saved.
	ts, _, _ := GetSyncState(contactsDB, "whatsapp")
	if ts == nil {
		t.Error("sync state not saved")
	}
}

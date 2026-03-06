package whatsapp

import "github.com/priyanshujain/openbotkit/store"

const schemaSQLite = `
CREATE TABLE IF NOT EXISTS whatsapp_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id TEXT NOT NULL,
	chat_jid TEXT NOT NULL,
	sender_jid TEXT,
	sender_name TEXT,
	text TEXT,
	timestamp DATETIME,
	media_type TEXT,
	media_path TEXT,
	is_group INTEGER DEFAULT 0,
	is_from_me INTEGER DEFAULT 0,
	reply_to_id TEXT,
	synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(message_id, chat_jid)
);

CREATE TABLE IF NOT EXISTS whatsapp_chats (
	jid TEXT PRIMARY KEY,
	name TEXT,
	is_group INTEGER DEFAULT 0,
	last_message_at DATETIME
);

CREATE TABLE IF NOT EXISTS whatsapp_contacts (
	jid TEXT PRIMARY KEY,
	phone TEXT,
	first_name TEXT,
	full_name TEXT,
	push_name TEXT,
	business_name TEXT,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_chat ON whatsapp_messages(chat_jid);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_timestamp ON whatsapp_messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_sender ON whatsapp_messages(sender_jid);
CREATE INDEX IF NOT EXISTS idx_whatsapp_contacts_phone ON whatsapp_contacts(phone);
`

const schemaPostgres = `
CREATE TABLE IF NOT EXISTS whatsapp_messages (
	id BIGSERIAL PRIMARY KEY,
	message_id TEXT NOT NULL,
	chat_jid TEXT NOT NULL,
	sender_jid TEXT,
	sender_name TEXT,
	text TEXT,
	timestamp TIMESTAMPTZ,
	media_type TEXT,
	media_path TEXT,
	is_group BOOLEAN DEFAULT FALSE,
	is_from_me BOOLEAN DEFAULT FALSE,
	reply_to_id TEXT,
	synced_at TIMESTAMPTZ DEFAULT NOW(),
	UNIQUE(message_id, chat_jid)
);

CREATE TABLE IF NOT EXISTS whatsapp_chats (
	jid TEXT PRIMARY KEY,
	name TEXT,
	is_group BOOLEAN DEFAULT FALSE,
	last_message_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS whatsapp_contacts (
	jid TEXT PRIMARY KEY,
	phone TEXT,
	first_name TEXT,
	full_name TEXT,
	push_name TEXT,
	business_name TEXT,
	updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_chat ON whatsapp_messages(chat_jid);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_timestamp ON whatsapp_messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_sender ON whatsapp_messages(sender_jid);
CREATE INDEX IF NOT EXISTS idx_whatsapp_contacts_phone ON whatsapp_contacts(phone);
`

func Migrate(db *store.DB) error {
	schema := schemaSQLite
	if db.IsPostgres() {
		schema = schemaPostgres
	}
	_, err := db.Exec(schema)
	return err
}

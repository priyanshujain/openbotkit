CREATE TABLE IF NOT EXISTS telegram_messages (
  id INTEGER PRIMARY KEY,
  message_id INTEGER NOT NULL,
  chat_id INTEGER NOT NULL,
  sender_id INTEGER,
  sender_name TEXT,
  text TEXT,
  timestamp DATETIME,
  media_type TEXT,
  reply_to_id INTEGER DEFAULT 0,
  is_outgoing INTEGER DEFAULT 0,
  edit_date DATETIME,
  synced_at DATETIME,
  UNIQUE(message_id, chat_id)
);

CREATE TABLE IF NOT EXISTS telegram_chats (
  chat_id INTEGER PRIMARY KEY,
  type TEXT,              -- 'user', 'group' or 'channel'
  title TEXT,
  username TEXT,          -- without the leading @
  access_hash INTEGER DEFAULT 0,
  is_group INTEGER DEFAULT 0,
  is_channel INTEGER DEFAULT 0,
  last_message_at DATETIME
);

CREATE TABLE IF NOT EXISTS telegram_users (
  user_id INTEGER PRIMARY KEY,
  username TEXT,
  first_name TEXT,
  last_name TEXT,
  phone TEXT,
  access_hash INTEGER DEFAULT 0,
  is_bot INTEGER DEFAULT 0,
  updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS telegram_sync_state (
  chat_id INTEGER PRIMARY KEY,
  min_id INTEGER DEFAULT 0,
  max_id INTEGER DEFAULT 0,
  backfilled_until DATETIME,
  updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_telegram_messages_chat ON telegram_messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_timestamp ON telegram_messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_sender ON telegram_messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_telegram_users_username ON telegram_users(username);

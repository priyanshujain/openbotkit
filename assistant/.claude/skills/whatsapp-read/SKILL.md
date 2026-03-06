---
name: whatsapp-read
description: Search WhatsApp messages, check chats, find conversations, look up what someone said
allowed-tools: Bash(sqlite3 *)
---

## Database

Path: `~/.obk/whatsapp/data.db`

## Schema

```sql
whatsapp_messages (
  id INTEGER PRIMARY KEY,
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
  synced_at DATETIME,
  UNIQUE(message_id, chat_jid)
)

whatsapp_chats (
  jid TEXT PRIMARY KEY,
  name TEXT,
  is_group INTEGER DEFAULT 0,
  last_message_at DATETIME
)

whatsapp_contacts (
  jid TEXT PRIMARY KEY,
  phone TEXT,
  first_name TEXT,
  full_name TEXT,
  push_name TEXT,
  business_name TEXT,
  updated_at DATETIME
)
```

Indexes: chat_jid, timestamp, sender_jid, contacts(phone).

## Query patterns

```bash
# List chats
sqlite3 ~/.obk/whatsapp/data.db "SELECT jid, name, is_group, last_message_at FROM whatsapp_chats ORDER BY last_message_at DESC LIMIT 20;"

# Recent messages in a chat
sqlite3 ~/.obk/whatsapp/data.db "SELECT timestamp, sender_name, text FROM whatsapp_messages WHERE chat_jid = '<jid>' ORDER BY timestamp DESC LIMIT 30;"

# Search messages by text
sqlite3 ~/.obk/whatsapp/data.db "SELECT timestamp, sender_name, text FROM whatsapp_messages WHERE LOWER(text) LIKE '%keyword%' ORDER BY timestamp DESC LIMIT 20;"

# Messages from a specific person
sqlite3 ~/.obk/whatsapp/data.db "SELECT timestamp, text FROM whatsapp_messages WHERE LOWER(sender_name) LIKE '%name%' ORDER BY timestamp DESC LIMIT 20;"

# My sent messages
sqlite3 ~/.obk/whatsapp/data.db "SELECT timestamp, chat_jid, text FROM whatsapp_messages WHERE is_from_me = 1 ORDER BY timestamp DESC LIMIT 20;"

# Group messages
sqlite3 ~/.obk/whatsapp/data.db "SELECT m.timestamp, m.sender_name, m.text FROM whatsapp_messages m JOIN whatsapp_chats c ON c.jid = m.chat_jid WHERE c.name LIKE '%group name%' ORDER BY m.timestamp DESC LIMIT 30;"

# Messages with media
sqlite3 ~/.obk/whatsapp/data.db "SELECT timestamp, sender_name, media_type, media_path FROM whatsapp_messages WHERE media_type IS NOT NULL AND media_type != '' ORDER BY timestamp DESC LIMIT 20;"

# Message count per chat
sqlite3 ~/.obk/whatsapp/data.db "SELECT c.name, COUNT(*) as cnt FROM whatsapp_messages m JOIN whatsapp_chats c ON c.jid = m.chat_jid GROUP BY m.chat_jid ORDER BY cnt DESC LIMIT 20;"
```

# Look up a contact by name
sqlite3 -header -column ~/.obk/whatsapp/data.db "SELECT jid, phone, full_name, push_name, business_name FROM whatsapp_contacts WHERE LOWER(full_name) LIKE '%name%' OR LOWER(push_name) LIKE '%name%' OR LOWER(first_name) LIKE '%name%';"

# Look up a contact by phone number
sqlite3 -header -column ~/.obk/whatsapp/data.db "SELECT jid, phone, full_name, push_name FROM whatsapp_contacts WHERE phone LIKE '%number%';"
```

Always use `-header -column` or `-json` mode for readable output.

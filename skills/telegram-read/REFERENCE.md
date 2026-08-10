## Schema

Full database schema: see schema.sql in this skill directory.

## Chat IDs

`chat_id` uses the Bot API form, so it identifies the peer kind on sight:

- positive: a one-to-one chat with that user, and `chat_id` equals `telegram_users.user_id`
- negative, small: a legacy group
- starts with `-100`: a channel or supergroup

`telegram_chats.type` spells this out as `user`, `group` or `channel`. A supergroup has
both `is_group` and `is_channel` set.

## Query patterns

```bash
# List chats
obk db telegram "SELECT chat_id, type, title, username, last_message_at FROM telegram_chats ORDER BY last_message_at DESC LIMIT 20;"

# Recent messages in a chat
obk db telegram "SELECT timestamp, sender_name, text FROM telegram_messages WHERE chat_id = <chat_id> ORDER BY timestamp DESC LIMIT 30;"

# Search messages by text
obk db telegram "SELECT timestamp, chat_id, sender_name, text FROM telegram_messages WHERE LOWER(text) LIKE '%keyword%' ORDER BY timestamp DESC LIMIT 20;"

# Messages from a specific person, by name or @username
# Step 1: find their user_id
obk db telegram "SELECT user_id, username, first_name, last_name FROM telegram_users WHERE LOWER(username) LIKE '%name%' OR LOWER(first_name) LIKE '%name%' OR LOWER(last_name) LIKE '%name%';"
# Step 2: their one-to-one chat_id is the same number as their user_id
obk db telegram "SELECT timestamp, text FROM telegram_messages WHERE chat_id = <user_id> ORDER BY timestamp DESC LIMIT 20;"
# Or everything they said anywhere, groups included
obk db telegram "SELECT timestamp, chat_id, text FROM telegram_messages WHERE sender_id = <user_id> ORDER BY timestamp DESC LIMIT 20;"

# Shortcut: join users and messages to search by person name
obk db telegram "SELECT m.timestamp, m.text FROM telegram_messages m JOIN telegram_users u ON m.sender_id = u.user_id WHERE LOWER(u.first_name) LIKE '%name%' OR LOWER(u.username) LIKE '%name%' ORDER BY m.timestamp DESC LIMIT 20;"

# My sent messages
obk db telegram "SELECT timestamp, chat_id, text FROM telegram_messages WHERE is_outgoing = 1 ORDER BY timestamp DESC LIMIT 20;"

# Group and channel messages by chat name
obk db telegram "SELECT m.timestamp, m.sender_name, m.text FROM telegram_messages m JOIN telegram_chats c ON c.chat_id = m.chat_id WHERE LOWER(c.title) LIKE '%group name%' ORDER BY m.timestamp DESC LIMIT 30;"

# Messages with media
obk db telegram "SELECT timestamp, chat_id, sender_name, media_type FROM telegram_messages WHERE media_type != '' ORDER BY timestamp DESC LIMIT 20;"

# Replies to a specific message
obk db telegram "SELECT timestamp, sender_name, text FROM telegram_messages WHERE chat_id = <chat_id> AND reply_to_id = <message_id> ORDER BY timestamp;"

# Message count per chat
obk db telegram "SELECT c.title, COUNT(*) AS cnt FROM telegram_messages m JOIN telegram_chats c ON c.chat_id = m.chat_id GROUP BY m.chat_id ORDER BY cnt DESC LIMIT 20;"

# Look up a user by @username or phone
obk db telegram "SELECT user_id, username, first_name, last_name, phone FROM telegram_users WHERE username = 'handle' OR phone LIKE '%number%';"
```

## Notes

- Only channels and supergroups have a per-chat message ID sequence of their own, so
  `message_id` is unique only together with `chat_id`.
- `sender_name` is a snapshot taken at sync time and can be empty for channel posts,
  where the channel itself is the author. Join `telegram_users` when you need the
  current name.
- History reaches back as far as the configured backfill window (90 days by default).
  `telegram_sync_state.backfilled_until` shows how far back each chat actually goes.

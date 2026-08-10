## Command

```bash
obk telegram messages send --to <chat> --text "<message>"
```

`--to` takes a numeric chat ID, an `@username`, or a chat title. Names are matched
against both title and username; if more than one chat matches, the command lists the
candidates and sends nothing.

## Finding the recipient

Resolve the recipient before sending. Use unified contacts first, then fall back to
Telegram-specific queries.

```bash
# Step 1: search unified contacts (cross-source, ranked by frequency)
obk contacts search "David"

# Step 2: search synced Telegram chats
obk telegram chats list --search david

# Fallback: query users directly
obk db telegram "SELECT user_id, username, first_name, last_name FROM telegram_users WHERE LOWER(first_name) LIKE '%name%' OR LOWER(username) LIKE '%name%';"
```

A user's one-to-one `chat_id` is the same number as their `user_id`. Groups are
negative, and channels and supergroups start with `-100`.

## Confirmation rules

- If the user's intent is clear and only ONE contact matches, send immediately, no need to confirm
- If MULTIPLE contacts match, show the matches and ask the user to pick
- If NO contacts match, tell the user and ask for clarification
- Only confirm content if the user's message is ambiguous

## Examples

```bash
# Send to a person by username
obk telegram messages send --to "@ann" --text "Hello!"

# Send to a person by chat ID
obk telegram messages send --to 123456789 --text "Hello!"

# Send to a supergroup or channel
obk telegram messages send --to -1001234567890 --text "Hey everyone"
```

## Notes

- Requires an authenticated Telegram session (`obk telegram auth login`)
- The recipient must already be synced, because sending needs the access hash recorded
  during sync. Run `obk telegram sync` if a chat is missing.
- The sent message is saved to the local database automatically

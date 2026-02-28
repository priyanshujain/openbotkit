---
name: whatsapp-send
description: Send a WhatsApp message to a contact or group
allowed-tools: Bash(obk *), Bash(sqlite3 *)
---

## Command

```bash
obk whatsapp messages send --to <jid> --text <message>
```

## Finding the recipient JID

Look up JIDs from the local database before sending:

```bash
# List all chats with JIDs
sqlite3 -header -column ~/.obk/whatsapp/data.db "SELECT jid, name, is_group FROM whatsapp_chats ORDER BY last_message_at DESC LIMIT 20;"

# Search for a contact by name
sqlite3 -header -column ~/.obk/whatsapp/data.db "SELECT jid, name FROM whatsapp_chats WHERE LOWER(name) LIKE '%search term%';"
```

JID formats:
- Individual: `<phone>@s.whatsapp.net` (e.g. `1234567890@s.whatsapp.net`)
- Group: `<id>@g.us`

## Example

```bash
# Send a message to a contact
obk whatsapp messages send --to "1234567890@s.whatsapp.net" --text "Hello!"

# Send a message to a group
obk whatsapp messages send --to "120363001234567890@g.us" --text "Hey everyone"
```

## Notes

- Requires an authenticated WhatsApp session (`obk whatsapp auth login`)
- The sent message is saved to the local database automatically
- Always confirm the recipient with the user before sending

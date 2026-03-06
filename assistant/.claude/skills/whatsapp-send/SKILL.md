---
name: whatsapp-send
description: Send a WhatsApp message to a contact or group
allowed-tools: Bash(obk *), Bash(sqlite3 *)
---

## Command

```bash
obk whatsapp messages send --to <jid> --text "<message>"
```

## Finding the recipient

Always resolve the recipient before sending. Use the contacts table first, then fall back to chats.

```bash
# Search contacts by name (preferred — has phone number and full name)
sqlite3 -header -column ~/.obk/whatsapp/data.db "SELECT jid, phone, full_name, push_name FROM whatsapp_contacts WHERE LOWER(full_name) LIKE '%name%' OR LOWER(push_name) LIKE '%name%' OR LOWER(first_name) LIKE '%name%';"

# Fall back to chats if not found in contacts
sqlite3 -header -column ~/.obk/whatsapp/data.db "SELECT jid, name FROM whatsapp_chats WHERE LOWER(name) LIKE '%name%';"
```

JID formats:
- Individual: `<phone>@s.whatsapp.net` (e.g. `919876543210@s.whatsapp.net`)
- Group: `<id>@g.us`

## Confirmation rules

- If the user's intent is clear and only ONE contact matches → send immediately, no need to confirm
- If MULTIPLE contacts match → show the matches and ask the user to pick
- If NO contacts match → tell the user and ask for clarification
- Only confirm content if the user's message is ambiguous

## Example

```bash
# Send a message to a contact
obk whatsapp messages send --to "919876543210@s.whatsapp.net" --text "Hello!"

# Send a message to a group
obk whatsapp messages send --to "120363001234567890@g.us" --text "Hey everyone"
```

## Notes

- Requires an authenticated WhatsApp session (`obk whatsapp auth login`)
- The sent message is saved to the local database automatically

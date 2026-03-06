---
name: email-send
description: Send an email or create a draft email via Gmail
allowed-tools: Bash(obk *), Bash(sqlite3 *)
---

## Commands

### Send an email

```bash
obk gmail send --to <address> --subject <subject> --body <body> [--cc <address>] [--bcc <address>] [--account <email>]
```

### Create a draft

```bash
obk gmail drafts create --to <address> --subject <subject> --body <body> [--cc <address>] [--bcc <address>] [--account <email>]
```

## Finding the account

If the user has multiple Gmail accounts, look up which to use:

```bash
sqlite3 -header -column ~/.obk/gmail/data.db "SELECT DISTINCT account FROM gmail_emails;"
```

## Examples

```bash
# Send a simple email
obk gmail send --to "alice@example.com" --subject "Meeting tomorrow" --body "Hi Alice, confirming our meeting at 2pm."

# Send to multiple recipients with CC
obk gmail send --to "alice@example.com" --to "bob@example.com" --cc "manager@example.com" --subject "Update" --body "Here's the update."

# Create a draft instead of sending
obk gmail drafts create --to "alice@example.com" --subject "Proposal" --body "Draft content here"

# Send from a specific account
obk gmail send --to "client@example.com" --subject "Invoice" --body "Please find attached." --account "work@gmail.com"
```

## Notes

- Requires authenticated Gmail account (`obk gmail auth login`)
- The `--to`, `--cc`, and `--bcc` flags accept multiple values
- Always confirm the recipient and content with the user before sending
- When unsure whether to send or draft, default to creating a draft

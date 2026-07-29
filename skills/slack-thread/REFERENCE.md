## When to use

- Someone shares a Slack permalink and you need the conversation behind it
- You need a whole thread including every reply, not just the one linked message
- Catching up on or summarising a discussion that happened in a thread
- You need a file or image attached to a thread — they carry content that is nowhere in the text
- Replying inside an existing thread

## When NOT to use

- Reading a channel's recent messages — use `obk slack read <channel>`
- Searching across Slack — use `obk slack search "<query>"`
- Sending a new message that isn't a reply to an existing thread

## Commands

### Read a thread

```bash
obk slack thread https://acme.slack.com/archives/C01Q9G9CD6X/p1785334150236249
obk slack thread --channel-id C01Q9G9CD6X --thread-id p1785334150236249
obk slack thread --channel-id support --thread-id p1785334150236249
```

Pass a permalink **or** `--channel-id` + `--thread-id`, never both. `--channel-id` also accepts a
channel name. Any message in the thread works — a link to a reply returns the whole thread, so you
never need to hunt for the root.

Add `--refresh-users` to re-fetch display names instead of using the cache.

Output is JSON:

```json
{
  "channel_id": "C01Q9G9CD6X",
  "thread_id": "p1785334150236249",
  "thread_ts": "1785334150.236249",
  "users": { "U02ABC": "rahul.support", "U04DEF": "priya.eng" },
  "messages": [
    {
      "message_id": "p1785334150236249",
      "ts": "1785334150.236249",
      "user": "U02ABC",
      "user_name": "rahul.support",
      "text": "<@U04DEF> can you check <#C02XY|ledger>?",
      "files": [{ "id": "F0BL9", "name": "shot.png", "url_private": "https://files.slack.com/..." }]
    }
  ]
}
```

Message text is **raw** — mentions stay as `<@U04DEF>`. Use the top-level `users` map to decode
them. That map covers everyone referenced anywhere in the thread: authors, mentions, reaction
voters, editors, and bots. Channel refs like `<#C02XY|ledger>` already carry the name inline.

Messages also carry `attachments` and `blocks` when present. Bots and integrations put their real
payload in `attachments`, so check there when `text` looks empty.

### Download a file from the thread

```bash
obk slack media download "<url_private>" > shot.png
obk slack media download "https://acme.slack.com/files/U01ABC/F01ABC/shot.png" > shot.png
```

Takes a `url_private` from a message's `files[]`, or a `/files/…` permalink. Writes the bytes to
**stdout**, so you must redirect to a file — it refuses to dump binary into a terminal. Logs and
errors go to stderr, so the redirect stays clean.

Then read the file. For images, that means you can actually see what was shared.

### Reply to the thread

```bash
obk slack reply https://acme.slack.com/archives/C01Q9G9CD6X/p1785334150236249 --text "fixed in v2.4"
echo "fixed in v2.4" | obk slack reply --channel-id C01Q9G9CD6X --thread-id p1785334150236249
```

Same addressing as `thread`. The body comes from `--text` or stdin; piped stdin wins. It is sent
**verbatim** — no markdown conversion, so write Slack mrkdwn (`*bold*`, not `**bold**`). Multi-line
bodies and code fences survive intact. On success it prints JSON including the new message's
permalink.

> [!CAUTION]
> `reply` posts immediately as you. There is no confirmation prompt and no undo. Confirm the
> target thread with the user before replying to anything you did not just read.

## Examples

Reading a thread, opening what was shared in it, and replying:

```bash
# 1. Read the thread the user linked
obk slack thread "https://acme.slack.com/archives/C01Q9G9CD6X/p1785334150236249" > /tmp/thread.json

# 2. List the files attached anywhere in it
jq -r '.messages[].files[]?.url_private' /tmp/thread.json

# 3. Download one and read it
obk slack media download "https://files.slack.com/files-pri/T1-F0BL9/shot.png" > /tmp/shot.png

# 4. Reply in the thread
obk slack reply "https://acme.slack.com/archives/C01Q9G9CD6X/p1785334150236249" \
  --text "confirmed, shipping in v2.4"
```

Summarising a thread without replying:

```bash
obk slack thread "<permalink>" | jq -r '.messages[] | "\(.user_name // .user): \(.text)"'
```

Decoding a mention using the `users` map:

```bash
# text: "<@U04DEF> can you check this?"  ->  users["U04DEF"] == "priya.eng"
jq '.users' /tmp/thread.json
```

## Notes

- Requires `obk slack auth login`.
- Names are cached per workspace in `~/.obk/slack/users-<workspace>.json`, so repeat runs cost no
  API calls. Use `--refresh-users` if someone changed their display name.
- Auth for file downloads uses the Slack Desktop cookie, which expires. If a download fails with
  "got an HTML page instead of the file", the credentials are stale — run `obk slack auth login`.
  The command errors instead of writing that login page into your image file.
- A permalink from a different workspace than the one you're authenticated to is rejected with an
  explicit error rather than a confusing `channel_not_found`.
- DM (`D…`), private group (`G…`), and public channel (`C…`) permalinks all work.
- Long threads are fully paginated — no silent truncation.
- Bulk-downloading every file in a thread is not supported; download them one URL at a time.

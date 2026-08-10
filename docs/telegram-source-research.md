# Research: Telegram as a data source

> **Status: built.** `source/telegram` ships on `gotd/td` as a user session. See
> "What actually shipped" at the end for where the implementation departed from
> this research.

## Question

We have `channel/telegram` (a bot you talk to) but no `source/telegram` (messages you can query).
Can we add one, and how?

Short answer: yes, but only via an MTProto user session. The bot token we already have cannot
read your Telegram history, and no amount of work on the bot channel will change that.

## Where we are today

Telegram is wired in as a channel only:

- `channel/telegram/poller.go` long-polls `getUpdates` and hands messages to the agent
- `channel/push_telegram.go` sends outbound notifications
- `config.TelegramConfig` holds `bot_token` and `owner_id`

Nothing persists those messages. There is no `source/telegram`, no `internal/cli/telegram`,
no `skills/telegram-read`, and `config.SourceDataDSN` has no telegram case.

The comparison that matters is WhatsApp, which has all four pieces plus a `whatsmeow` client
that logs in as the user's own account. That is what makes `obk db whatsapp` possible.

## The three ways to get Telegram data

### 1. Bot API (what we have) — cannot work

The Bot API is real-time only. A bot receives updates for chats it is a member of, from the
moment it joins, and Telegram drops undelivered updates after 24 hours. There is no method to
fetch messages from before the bot joined, by design: it stops bots retroactively harvesting
chats. In groups, privacy mode further limits a non-admin bot to commands addressed at it.

Your own DMs with other people are invisible to a bot entirely. So the Bot API gives us zero
history and zero coverage of the chats that actually matter.

### 2. Telegram Business bot (Bot API 7.2+) — partial, no history

Since March 2024 a Telegram Premium user can connect a bot to their account under
Settings > Telegram Business > Chatbots. The bot then receives `business_message`,
`edited_business_message` and `deleted_business_messages` updates for the owner's **private**
chats, and can send as the owner by passing `business_connection_id`.

This is legitimate, first-party, and carries no account risk. But:

- Requires Telegram Premium on the account.
- Private chats only. Groups and channels are not covered.
- Still no history. You get messages from the moment of connection forward, nothing before.
- The bot must have Business Mode enabled in BotFather.

Worth noting: our channel is pinned to `go-telegram-bot-api/telegram-bot-api v5.5.1`, which is
frozen at roughly Bot API 6.x (2022). It has no `BusinessMessage`, no `BusinessConnection`, not
even `MessageReaction`. Taking this path means swapping the library for `go-telegram/bot` or
`mymmrac/telego`, both of which track current Bot API. That swap is probably worth doing on its
own merits regardless of which source approach we pick.

### 3. MTProto user session (gotd/td) — the only complete option

Log in as the user's own Telegram account, the same way the desktop and mobile apps do. Full
dialog list, full history via `messages.getHistory`, live updates via `updates.getDifference`
and update handlers, groups and channels included.

`github.com/gotd/td` is the pure-Go MTProto client. Actively maintained (releases through mid
2026), no cgo, feature parity with TDLib as a goal, ~150kb per idle client. It sits in exactly
the same slot for Telegram that `whatsmeow` occupies for WhatsApp, including being an
unofficial-client library.

Auth flow: register an `api_id` / `api_hash` at my.telegram.org, then phone number, login code,
optional 2FA password. `telegram/auth.NewFlow(...).IfNecessary(...)` handles the whole thing.
Since Telegram is already signed in on this machine, the login code arrives in-app rather than
by SMS, which makes setup less annoying than WhatsApp's QR dance.

## The local Telegram install is not a shortcut

Worth checking, since iMessage and Apple Notes work exactly this way. It does not pan out here.

The installed client is Telegram for macOS (`ru.keepcoder.Telegram`, App Store build), not
Telegram Desktop. Its data lives under
`~/Library/Group Containers/6N38VWS5BX.ru.keepcoder.Telegram/appstore/`:

- `accounts-metadata/db/db_sqlite` — plain SQLite, a single `t2(key BLOB, value BLOB)` table,
  8 rows. Opens fine.
- `account-<id>/postbox/db/db_sqlite` — 58 MB, SQLCipher with a plaintext 16-byte header. The
  file starts with `SQLite format 3\0` and then garbage; `sqlite3` refuses it with
  "unsupported file format".

I did not establish where the postbox key lives. It is not in the login keychain under a
service named "Telegram", so it is either inside that metadata db or under some other keychain
item.

Even if the key were recovered, this is a dead end. Postbox is not a relational schema like
iMessage's `chat.db`. It is a key-value store whose values are Telegram's own `PostboxCoding`
binary serialization. Reading messages out of it means reimplementing Telegram's Swift
serialization format and tracking it as they change it. Nobody does this. Bridges do not do
this.

Telegram Desktop's `tdata` has known Go decryption tools when no local passcode is set, but it
is not installed here and the payload has the same problem: a custom binary cache, not a
queryable store.

The one genuinely useful local artifact is the official **Export chat history** feature
(JSON/HTML, complete, includes media). It is a manual one-shot export, so it is a plausible
backfill bootstrap but not a sync mechanism.

## What everyone else does

**OpenClaw** does exactly what we do: bot token, real-time channel, no source. Business bot
support is an open feature request (openclaw/openclaw#20786) with an unmerged PR. Their
`resolveTelegramAllowedUpdates()` omits the business update types, so Telegram rejects the bot
when a user turns on Business Mode. We do not set `allowed_updates` at all, so we would receive
business updates by default, but our 2022-vintage library has no types to decode them into.

**n8n, Zapier, LangChain telegram tools, and every "build a Telegram AI assistant" tutorial**:
bot token, real-time only. None of them read your history.

**mautrix-telegram and Beeper** are the only category that actually ingests a user's Telegram
history, and they do it with an MTProto user session via Telethon, with explicit backfill
commands, batch limits, and forward-backfill timeouts. That is the reference implementation for
what we would be building.

So: if we want a real Telegram source, we are not following the crowd, we are following the
bridges.

## Risk

MTProto user clients ("userbots") are the thing to be honest about. Telegram publishes an API
and permits third-party clients, and reading your own history at a sane rate is ordinary client
behaviour. Bans are driven by behavioural signals: mass DMs, scraping, bulk joins, fresh
accounts, datacenter IPs, aggressive call rates. A long-standing personal account doing
read-only sync from a residential IP is low risk, but not zero, and the downside is losing the
Telegram account rather than just the integration.

Practical mitigations: honour `FLOOD_WAIT` strictly, rate-limit backfill, page `getHistory`
with sleeps rather than hammering, and keep the account's normal client sessions alive.

This is the same category of risk we already accepted with `whatsmeow`, and arguably lower,
since Telegram documents its client API and WhatsApp does not.

## Recommendation

Build `source/telegram` on `gotd/td` as a user session, mirroring the WhatsApp source layout.
The Business-bot path is safer but delivers private chats only, going forward only, and needs
Premium. A Telegram source with no history and no groups would not answer the questions people
actually ask ("what did X say about Y last month"), so it is not worth the build on its own.

Keep the Business bot in the back pocket: if the userbot risk is unacceptable, it is the
fallback, and it composes with the library upgrade we should do anyway.

## Sketch of the work

Mirroring `source/whatsapp` piece for piece:

- `source/telegram/client.go` — gotd client, session persisted next to the other source state
- `source/telegram/auth.go` — phone / code / 2FA flow, driven from `obk setup telegram`
- `source/telegram/schema.go` — `telegram_messages`, `telegram_chats`, `telegram_users`, sqlite
  and postgres variants, same shape as the whatsapp tables
- `source/telegram/sync.go` — dialog enumeration, paged `messages.getHistory` backfill with a
  per-chat high-water mark, then `updates.getDifference` for incremental
- `source/telegram/store.go` — save/query helpers, `CountMessages`, `LastSyncTime`
- `source/telegram/telegram.go` — `Source` impl, `Status`, `init()` registration
- `internal/cli/telegram/` — `auth`, `sync`, `messages`, `chats`, `contacts`
- `config`: `TelegramDataDSN()` plus a `"telegram"` case in `SourceDataDSN`
- `skills/telegram-read/` — SKILL.md, REFERENCE.md, schema.sql
- `settings/registry.go` — api_id / api_hash entries, credentials via keyring

Open questions before writing any of it:

- api_id/api_hash are per-developer, not per-user. Do we ship one for openbotkit, or make each
  user register their own at my.telegram.org? Shipping ours means our app id absorbs everyone's
  behaviour and can be revoked for all users at once. Making users register is one more setup
  step but isolates blast radius. Bridges generally ship their own and accept the risk.
- Backfill depth. All history is a lot of rows and a lot of API calls. A `--since` window or a
  per-chat message cap is probably the sane default.
- Whether the source session and the bot channel coexist on the same account, and whether the
  bot's own DMs with you get double-recorded.

## What actually shipped

The recommendation held: `source/telegram` runs on `gotd/td` as a user session, laid out
piece for piece against `source/whatsapp`. The open questions resolved as follows.

**Each user registers their own api_id/api_hash.** `obk setup telegram` walks through
my.telegram.org and stores the pair in the OS keyring as `keychain:obk/telegram/api_id` and
`keychain:obk/telegram/api_hash`, with `TELEGRAM_API_ID` / `TELEGRAM_API_HASH` as the headless
fallback. Shipping one pair would have collected `API_ID_PUBLISHED_FLOOD` for everyone, and a
single revocation would have taken every install down at once.

**Backfill defaults to 90 days**, configurable via `telegram.backfill_days` or `--since`
(`30d`, `12h`, `YYYY-MM-DD`, or `all`). Progress is recorded per chat in `telegram_sync_state`,
so an interrupted run resumes from the oldest message it stored rather than starting over.

**The two Telegram configs coexist.** `config.TelegramConfig` was already taken by the bot
channel under `channels.telegram`, so the source type is `TelegramSourceConfig` on a new
top-level `telegram` key. The setup wizard lists them as separate options.

Departures from the sketch worth knowing about:

- **Login is a browser QR code, not phone and code.** `obk telegram auth login` serves a page
  on `127.0.0.1:8086`, mirroring `obk whatsapp auth login`. Port `8085` was already taken by
  both the WhatsApp QR server and the Google OAuth callback. It binds to loopback because
  `/api/password` forwards cloud passwords to Telegram; `--addr` opts out for container use.
- **2FA needed writing ourselves.** `qrlogin` returns `SESSION_PASSWORD_NEEDED` rather than
  handling the cloud password, so the auth page carries a real password state that collects it
  over `POST /api/password` and retries on a wrong password.
- **Chat IDs are normalised to the Bot API form.** Telegram numbers users, legacy chats and
  channels in separate spaces, so a raw ID is not unique. `telegram_chats.chat_id` uses the
  familiar `-100` prefix for channels and a plain negative for legacy groups, which makes it
  safe as a primary key and matches the IDs users see elsewhere.
- **Backfill sits behind a `Fetcher` interface**, following `source/twitter` rather than
  `source/whatsapp`, so parsing and resume logic are covered by tests with no network.
- **The daemon notifies on rows actually written.** WhatsApp's `runWhatsAppSync` fires the
  notifier on a blind 30s ticker, which churns reactive triggers; the Telegram update handler
  calls it only when a message lands.
- **Access hashes are stored on `telegram_chats` and `telegram_users`.** This is what lets
  `obk telegram messages send` rebuild an input peer without a second lookup, and it doubles as
  the backing store for the update manager's access hashers.

Adding `gotd/contrib` for `floodwait` and `ratelimit` pulled `minio-go` forward to v7.2.1,
which requires `charmbracelet/x/ansi` v0.11.7, which needs `x/cellbuf` v0.0.15 to compile. That
cellbuf bump is why an otherwise unrelated dependency moved in the same change.

Still open: whether the bot channel and the source double-record the bot's own DMs with you.
Nothing has been done about that yet.

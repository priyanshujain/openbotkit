# OpenBotKit

A toolkit for building AI personal assistants through data source integrations.

OpenBotKit (`obk`) serves dual purposes: a **CLI tool** for syncing and querying personal data, and a **Go library** that agent developers can import to build AI assistants.

## Install

```bash
go install github.com/priyanshujain/openbotkit@latest
```

Or build from source:

```bash
git clone https://github.com/priyanshujain/openbotkit.git
cd openbotkit
go build -o obk .
```

## Quick Start

```bash
# Initialize configuration
obk config init

# Place your Google OAuth credentials
cp credentials.json ~/.obk/gmail/credentials.json

# Authenticate your Gmail account
obk gmail auth login

# Sync emails
obk gmail sync

# List stored emails
obk gmail emails list

# Search emails
obk gmail emails search "invoice"

# Check status of all sources
obk status
```

## CLI Commands

```
obk version                          # Print version
obk status                           # All sources: connected?, items, last sync

obk config init                      # Create default config at ~/.obk/config.yaml
obk config show                      # Print resolved config
obk config set <key> <value>         # Set a config value
obk config path                      # Print config directory

obk gmail auth login                 # OAuth2 browser flow
obk gmail auth logout [--account]    # Remove stored tokens
obk gmail auth status                # Show connected accounts

obk gmail sync                       # Incremental sync
    [--account EMAIL]                # Filter to one account
    [--full]                         # Re-fetch everything
    [--after DATE]                   # Only emails after this date
    [--download-attachments]         # Save attachments to disk

obk gmail emails list                # Paginated list of stored emails
    [--account EMAIL] [--from ADDR]
    [--subject TEXT] [--after DATE]
    [--before DATE] [--limit N]
    [--json]

obk gmail emails get <message-id>    # Full email details
    [--json]

obk gmail emails search <query>      # Full-text search
    [--json]

obk gmail attachments list           # List attachment metadata
    [--email-id ID] [--json]
```

## Library Usage

```go
import (
    "github.com/priyanshujain/openbotkit/source/gmail"
    "github.com/priyanshujain/openbotkit/store"
)

// Open database
db, _ := store.Open(store.SQLiteConfig("gmail.db"))
gmail.Migrate(db)

// Create Gmail source
g := gmail.New(gmail.Config{
    CredentialsFile: "credentials.json",
    TokenDBPath:     "tokens.db",
})

// Sync emails
result, _ := g.Sync(ctx, db, gmail.SyncOptions{Full: false})

// Query stored emails
emails, _ := gmail.ListEmails(db, gmail.ListOptions{
    From:  "someone@example.com",
    Limit: 10,
})
```

## Configuration

Config lives at `~/.obk/config.yaml` (override with `OBK_CONFIG_DIR`):

```yaml
gmail:
  credentials_file: ~/.obk/gmail/credentials.json
  download_attachments: false
  storage:
    driver: sqlite    # or "postgres"
    dsn: ""           # postgres DSN; sqlite path auto-derived
```

## Data Directory

```
~/.obk/
├── config.yaml
└── gmail/
    ├── credentials.json    # Google OAuth client creds (user provides)
    ├── tokens.db           # OAuth tokens (always local SQLite)
    ├── data.db             # Email data (when using SQLite)
    └── attachments/        # Downloaded attachments
```

## Prerequisites

- Go 1.24+
- Gmail API credentials ([Google Cloud Console](https://console.cloud.google.com/apis/credentials))

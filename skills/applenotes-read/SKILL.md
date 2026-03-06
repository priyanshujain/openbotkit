---
name: applenotes-read
description: Search Apple Notes, find notes by title or content, browse notes by folder
allowed-tools: Bash(sqlite3 *)
---

## Database

Path: `~/.obk/applenotes/data.db`

## Schema

```sql
applenotes_notes (
  id INTEGER PRIMARY KEY,
  apple_id TEXT NOT NULL UNIQUE,
  title TEXT,
  body TEXT,
  folder TEXT,
  folder_id TEXT,
  account TEXT,
  password_protected INTEGER DEFAULT 0,
  created_at DATETIME,
  modified_at DATETIME,
  synced_at DATETIME
)

applenotes_folders (
  id INTEGER PRIMARY KEY,
  apple_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  parent_apple_id TEXT,
  account TEXT
)
```

Indexes: apple_id, folder, modified_at.

## Query patterns

```bash
# Recent notes
sqlite3 ~/.obk/applenotes/data.db "SELECT modified_at, folder, title FROM applenotes_notes ORDER BY modified_at DESC LIMIT 20;"

# Search by title
sqlite3 ~/.obk/applenotes/data.db "SELECT modified_at, folder, title FROM applenotes_notes WHERE LOWER(title) LIKE '%keyword%' ORDER BY modified_at DESC LIMIT 20;"

# Full text search across title and body
sqlite3 ~/.obk/applenotes/data.db "SELECT modified_at, folder, title, substr(body, 1, 200) FROM applenotes_notes WHERE LOWER(title) LIKE '%term%' OR LOWER(body) LIKE '%term%' ORDER BY modified_at DESC LIMIT 10;"

# Read full note
sqlite3 ~/.obk/applenotes/data.db "SELECT title, folder, account, created_at, modified_at, body FROM applenotes_notes WHERE id = <id>;"

# Notes in a specific folder
sqlite3 ~/.obk/applenotes/data.db "SELECT modified_at, title FROM applenotes_notes WHERE LOWER(folder) = 'notes' ORDER BY modified_at DESC LIMIT 20;"

# List all folders
sqlite3 ~/.obk/applenotes/data.db "SELECT name, account, (SELECT COUNT(*) FROM applenotes_notes WHERE folder_id = f.apple_id) as note_count FROM applenotes_folders f ORDER BY name;"

# Notes by account
sqlite3 ~/.obk/applenotes/data.db "SELECT account, COUNT(*) FROM applenotes_notes GROUP BY account;"

# Recently modified notes (last 7 days)
sqlite3 ~/.obk/applenotes/data.db "SELECT modified_at, folder, title FROM applenotes_notes WHERE modified_at >= datetime('now', '-7 days') ORDER BY modified_at DESC;"
```

Always use `-header -column` or `-json` mode for readable output.

---
name: memory-read
description: Recall past conversations, what was discussed before, previous questions asked, conversation history
allowed-tools: Bash(sqlite3 *)
---

## Database

Path: `~/.obk/memory/data.db`

## Schema

```sql
memory_conversations (
  id INTEGER PRIMARY KEY,
  session_id TEXT NOT NULL UNIQUE,
  cwd TEXT,
  started_at DATETIME,
  updated_at DATETIME
)

memory_messages (
  id INTEGER PRIMARY KEY,
  conversation_id INTEGER REFERENCES memory_conversations(id),
  role TEXT NOT NULL,  -- "user" or "assistant"
  content TEXT,
  timestamp DATETIME
)
```

Indexes: session_id, conversation_id, role.

## Query patterns

```bash
# Recent conversations
sqlite3 ~/.obk/memory/data.db "SELECT id, session_id, cwd, started_at, updated_at FROM memory_conversations ORDER BY updated_at DESC LIMIT 10;"

# Messages in a conversation
sqlite3 ~/.obk/memory/data.db "SELECT role, substr(content, 1, 200) FROM memory_messages WHERE conversation_id = <id> ORDER BY timestamp;"

# Search across all conversations
sqlite3 ~/.obk/memory/data.db "SELECT c.session_id, c.cwd, m.role, substr(m.content, 1, 200) FROM memory_messages m JOIN memory_conversations c ON c.id = m.conversation_id WHERE LOWER(m.content) LIKE '%keyword%' ORDER BY m.timestamp DESC LIMIT 20;"

# What did I ask about a topic
sqlite3 ~/.obk/memory/data.db "SELECT c.cwd, m.content FROM memory_messages m JOIN memory_conversations c ON c.id = m.conversation_id WHERE m.role = 'user' AND LOWER(m.content) LIKE '%topic%' ORDER BY m.timestamp DESC LIMIT 10;"

# What was the assistant's response about a topic
sqlite3 ~/.obk/memory/data.db "SELECT substr(m.content, 1, 500) FROM memory_messages m JOIN memory_conversations c ON c.id = m.conversation_id WHERE m.role = 'assistant' AND LOWER(m.content) LIKE '%topic%' ORDER BY m.timestamp DESC LIMIT 10;"

# Conversations by project directory
sqlite3 ~/.obk/memory/data.db "SELECT id, session_id, started_at FROM memory_conversations WHERE cwd LIKE '%project-name%' ORDER BY updated_at DESC;"

# Stats
sqlite3 ~/.obk/memory/data.db "SELECT COUNT(*) as conversations FROM memory_conversations; SELECT COUNT(*) as messages FROM memory_messages;"
```

Always use `-header -column` or `-json` mode for readable output.

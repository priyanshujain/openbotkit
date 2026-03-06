# Assistant

Personal assistant powered by Claude Code with access to email, WhatsApp, conversation history, and Google Workspace services.

## Setup

1. Install and configure obk:
   ```bash
   go install github.com/priyanshujain/openbotkit@latest
   obk setup
   ```

2. This directory must be symlinked outside the repo to avoid loading the parent CLAUDE.md.

   Claude Code walks up the directory tree and loads all CLAUDE.md files from parent directories.
   There is no way to disable this ([open feature request](https://github.com/anthropics/claude-code/issues/20880)).
   If you run `claude` directly from `assistant/` inside the repo, it will load the root CLAUDE.md
   (dev rules, commit conventions, etc.) into the assistant context, which is not what we want.

   ```bash
   ln -s /path/to/openbotkit/assistant ~/assistant
   cd ~/assistant && claude
   ```

3. `make install` creates the skills symlink (`assistant/.claude/skills → ~/.obk/skills`).

## Prerequisites

- `obk` must be in PATH: `make install` (from repo root)
- Gmail synced: `obk gmail sync` (after `obk setup`)
- WhatsApp synced: `obk auth whatsapp login`
- Google Workspace (optional): configured via `obk setup` (requires `gws` CLI)

## What's inside

- `CLAUDE.md` — personal assistant persona and behavioral rules
- `.claude/settings.json` — Stop hook for conversation capture, gws permissions
- `.claude/skills → ~/.obk/skills` — symlink to installed skills (created by `make install`)

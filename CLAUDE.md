## Project Guidelines

- Do not call the task done until it is fully complete and tested.
- Always write tests for new features and bug fixes.
- Do not dismiss bug as a pre-existing" issue even if it was present before your change. It does not matter, it's still your responsibility to fix it. When you see a bug, fix it. Don't ignore it.

## Coding Guidelines

  - Keep code simple and easy to read.
  - Avoid excessive comments — only comment when absolutely necessary.

## Destructive Actions

  - NEVER delete databases, config files, or user data. If a schema changes, rename/migrate in place.
  - Always prefer non-destructive alternatives (ALTER TABLE RENAME, mv, cp) over delete-and-recreate.
  - Ask before removing any file that contains user data.

## Git Branch Rules

  - No slashes in branch names (e.g., use `fix-something` not `fix/something`).

## Git Commit Rules

  - Commit after every small, atomic change. Each commit should touch 1-3 files max.
  - Use conventional commit format: `feat|fix|refactor|docs|test|chore|ci(scope): message`
  - Never use `git add .` or `git add -A`. Always stage specific files by name.
  - Keep commits small: aim for under 20 lines changed per commit.
  - Don't batch multiple unrelated changes into one commit.
  - Commit early and often — a working 5-line change is better than a pending 200-line change.

## Pull Request Rules

  - PR descriptions are plain text. No markdown, no headings, no bullets, no bold, no code blocks, no emoji, no tables.
  - A few lines, that's it. Don't write an essay.
  - Super casual, like telling a teammate over chat. Lowercase is fine.
  - Don't polish it. A few typos and loose grammar are fine and preferred over something that reads like a template.
  - Keep the facts right even though the tone is casual. Casual is about the voice, not about being vague or wrong.
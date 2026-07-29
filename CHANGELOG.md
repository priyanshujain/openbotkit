# Changelog

All notable changes to OpenBotKit are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/).

## [0.13.0] (2026-07-29)

### Added

- **slack**: Add ConversationsHistoryAll with cursor pagination
- **slack/cli**: Add export command for full channel JSON export

## [0.12.0] (2026-04-06)

### Added

- **channel**: Add Registry for centralized pusher management
- **gmail**: Return new email IDs from Sync for event hooks
- **daemon**: Add fan-out and data payload to SyncNotifier
- **daemon**: Pass new email IDs through gmail sync to notifier
- **hooks**: Add event_hooks schema, types, and store
- **gmail**: Add GetEmailsByIDs for loading emails by row ID
- **daemon**: Add EventHookWorker for post-sync LLM classification
- **daemon**: Add HookListener for event-driven hook dispatch
- **daemon**: Wire channel registry, hooks DB, and hook listener
- **cli**: Add OBK_GMAIL_DRY_RUN env var for email send and draft
- **tools**: Add prompt prefixes to gate email write commands
- **test**: Add CapturePusher, RecordingInteractor, AgentWithInteractor

### Fixed

- **test**: Run event hook test through full River pipeline
- **jobs**: Early return in loadEmails for empty IDs
- **jobs**: Truncate by runes instead of bytes for UTF-8 safety
- **daemon**: Remove leaky C() method from SyncNotifier
- **jobs**: Wire UpdateLastRun into EventHookWorker
- **cli**: Handle json.Marshal error in gmail dry-run paths
- **daemon**: Add panic recovery to HookListener goroutine

## [0.11.0] (2026-04-06)

### Added

- **registry**: Init Go module and DB store wrapper
- **registry**: Add User/UseCase models and DB schema
- **registry**: Add server bootstrap, config, and health endpoint
- **registry**: Add CORS, body limit, and JWT auth middleware
- **registry**: Add Google OAuth, demo login, and user handlers
- **registry**: Add store queries for users and use cases
- **registry**: Add public browse and detail handlers
- **registry**: Add create, update, delete use case handlers
- **registry**: Add fork and dashboard handlers
- **registry**: Add 8 curated seed use cases across domains
- **registry**: Init frontend with React, Vite, Tailwind, shadcn deps
- **registry**: Add HTML entry points, TSX entries, and global CSS
- **registry**: Add frontend lib (types, API client, auth hook, utils)
- **registry**: Add shadcn/ui components
- **registry**: Add Layout and all pages (Home, Detail, Form, Dashboard)
- **registry**: Add Express production server with API proxy
- **registry**: Replace seed data with 18 researched use cases
- **registry**: Add pagination controls to browse page

### Changed

- **registry**: Single-column layout grouped by domain

### Fixed

- **registry**: Add input validation for use case fields
- **registry**: Enforce JWT secret in production, fix fork race condition
- **registry**: Add error states, loading states, permission checks
- **registry**: Restrict CORS to exact frontend URLs
- **registry**: Hide private/draft use cases from non-authors
- **registry**: Validate use case ID as hex before API calls
- **registry**: Default empty fields on update like create does
- **registry**: Check rows.Err() after DB iteration loops
- **registry**: Set Secure flag on auth cookie in production
- **registry**: Add max length validation for description (5000 chars)
- **registry**: Decrement fork count when forked use case is deleted
- **registry**: Add search debounce and abort controller
- **registry**: Deduplicate auth requests with cached singleton
- **registry**: Add client-side form validation and inline errors
- **registry**: Fix demo login redirect and add port 5174 to CORS
- **registry**: Improve mobile layout for header and detail footer
- **registry**: Use destructive color for error messages on home page
- **registry**: Highlight active nav link in header
- **registry**: Replace alert/confirm with inline error and confirm UI
- **registry**: Escape LIKE wildcards in search queries

## [0.10.0] (2026-04-01)

### Added

- **config**: Add generic SetByPath for arbitrary config keys
- **skills**: Add custom skill install/remove/list/get/update functions
- **cli**: Add obk skills create/update/remove/list/show commands
- **skills**: Add skill-creator meta-skill
- **skills**: Register skill-creator in builtinSkills
- **server**: Add credential request/form/store endpoints
- **skills**: Add config-manage skill for configuration discovery

### Changed

- **cli**: Replace hardcoded config set switch with SetByPath

### Fixed

- **skills**: Preserve custom/external skills during declarative Install()
- **server**: Use keyring mock in credential tests, add auth test
- **spectest**: Add skill-creator to installed skills in test fixture
- **usecase**: Rewrite integration tests with natural prompts
- **skills**: Validate skill names to prevent path traversal
- **config**: Fix dead empty-path guard in SetByPath
- **server**: Sweep expired credential tokens on create
- **cli**: Isolate cobra flag state in skills CLI tests

## [0.9.1] (2026-04-01)

### Fixed

- **deps**: Bump picomatch and smol-toml for security patches
- **deps**: Bump testcontainers-go to v0.41.0
- **daemon**: Make TestCheckOverdueRecurringSkipsRecentRun time-agnostic

## [0.9.0] (2026-04-01)

### Added

- **config**: Add workspace directory config and setting
- **subagent**: Use system blocks and increase maxIter to 25
- **tools**: Add NewSubagentRegistry for enriched subagent
- **telegram**: Enrich subagent tools and add workspace to prompt
- **whatsapp**: Enrich subagent tools and add workspace to prompt
- **cli**: Enrich subagent tools and add workspace to prompt
- **usecase**: Register subagent and workspace in test fixture
- **usecase**: Add delegation agent builder and test interactor
- **spectest**: Add AssertChecklist for binary yes/no judge evaluation
- **spectest**: Add JudgeFastProvider/JudgeFastModel to LocalFixture
- **usecase**: Wire fast judge model for checklist evaluation
- **spectest**: Add timing instrumentation to AssertChecklist
- **websearch**: Add StatusError type and error classification
- **websearch**: Error-type-aware health tracker with half-open state
- **delegate_task**: Add LLM-decided timeout_minutes parameter

### Changed

- **subagent**: Return raw output, let registry handle truncation
- **delegate**: Return raw output from sync path, simplify prompt
- **websearch**: Pass failure kind to health tracker
- **websearch**: Remaining engines return StatusError
- **tools**: Extract BuildSubagentTool to deduplicate session setup
- **websearch**: Merge stateHealthy and stateHalfOpen switch cases

### Fixed

- **usecase**: Fix web search test prompts and judge criteria
- **test**: Update subagent tests for SystemBlocks and maxIter 25
- **usecase**: Fix delegate test prompt and add deliver-results instruction
- **prompt**: Add deliver-results instruction to delegate_task section
- **spectest**: Use fast judge for AssertChecklist, handle empty response
- **usecase**: Use highest-priority agent for delegate test, add timing
- Resolve merge conflicts with origin/master
- **websearch**: DuckDuckGo Sec-Fetch headers and StatusError
- **websearch**: Mojeek omit s param on page 1, add Sec-Fetch headers
- **websearch**: Brave Sec-Fetch headers and StatusError
- **settings**: Update tree tests for Workspace category
- **delegate_task**: Pass timeout_minutes through async path
- **websearch**: Scope 202 rate-limit classification to DuckDuckGo
- **tools**: Ensure workspace directory exists in BuildSubagentTool

## [0.8.0] (2026-03-31)

### Added

- **x**: Add query ID config loader with YAML support
- **x**: Add credential storage with keyring and token validation
- **x**: Add transaction ID generator stub
- **x**: Add core GraphQL client with Chrome TLS fingerprinting
- **x**: Add GraphQL endpoint methods for all 10 operations
- **x**: Add domain types and dual SQLite/Postgres schema
- **x**: Add store CRUD operations with upsert and search
- **x**: Add URT response parsers for timeline, search, notifications
- **x**: Add source interface implementation and sync engine
- **x**: Add X config with storage settings and DSN helper
- **x**: Add CLI commands and wire into root
- **x**: Add query ID extractor tool
- **x**: Add X to settings TUI data sources
- **x**: Add programmatic login flow via username/password
- **x**: Add login wizard to settings TUI
- **x**: Update CLI auth to use username/password login
- **x**: Add mock DOM for JS instrumentation execution
- **x**: Add JS instrumentation solver using goja VM
- **browser**: Add Chrome cookie decryption package
- **browser**: Add Safari binary cookie parser
- **browser**: Add Firefox cookie reader
- **browser**: Add cookie extraction orchestrator
- **x**: Add browser session extraction
- **x**: Replace CLI login with cookie extraction
- **x**: Replace TUI login wizard with cookie extraction
- **browser**: Add single-browser extraction and AvailableBrowsers
- **x**: Add ExtractSessionFromBrowserByName
- **x**: Add browser selection to CLI auth login
- **x**: Add browser selection to TUI settings wizard
- **x**: Add X to setup wizard with browser selection
- **x**: Add replies command to show replies to a post
- **x**: Add x-read skill for timeline, search, and notifications
- **x**: Add x-post skill for posting, replying, liking, reposting
- **x**: Register x-read and x-post in builtin skills

### Changed

- **x**: Remove login flow code
- **x**: Deduplicate isPermissionError into cookies.IsPermissionError
- **audit**: Add LogQuick helper, simplify logXAudit in CLI and TUI

### Fixed

- **x**: Update settings TUI to show account status with sign-in prompt
- **x**: Add audit logging to login flow and fix error display
- **x**: Replace stub instrumentation with real JS execution
- **x**: Improve JS solver with proper DOM mock and function extraction
- **browser**: Remove Safari from cookie extraction priority
- **settings**: Remove X-specific storage driver/DSN from settings TUI
- **chrome**: Handle SHA256 hash prefix in Chrome v130+ cookie decryption
- **x**: Validate session with real API call on login and status check
- **chrome**: Validate all PKCS7 padding bytes before stripping

## [0.7.0] (2026-03-29)

### Added

- **spectest**: Export NewEnv and Dir for fixture reuse
- **usecase**: Add profile-based use case test framework
- **usecase**: Add ScheduleAgent and SchedDBPath helpers
- **spectest**: Install schedule-task and web skills in test fixture
- **usecase**: Add web tools and scheduling context to agents

### Changed

- **usecase**: Use full production-like agent with all tools

### Fixed

- **spectest**: Disable thinking for judge LLM calls
- **spectest**: Increase judge MaxTokens for thinking models
- **usecase**: Use same agent for execution simulation in reminders test
- **usecase**: Handle MkdirAll error and update README
- **usecase**: Guard against missing profile in config.Profiles
- **usecase**: Find schedule by type instead of assuming index order

### Performance

- **spectest**: Cache obk binary build across test fixtures

## [0.6.0] (2026-03-25)

### Added

- **backup**: Notify via Telegram on final failure

### Changed

- **daemon**: Remove backup periodic job from River
- **daemon**: Unified tick loop with sleep/wake catch-up

### Fixed

- **backup**: Handle nil Channels config in notifyFailure
- **settings**: Call LinkSource("backup") in backup wizard

## [0.5.1] (2026-03-25)

### Fixed

- **ci**: Skip tag creation when version tag already exists
- **ci**: Skip changelog release when version tag already exists

## [0.5.0] (2026-03-25)

### Added

- **agent**: Add WithOnIntermediateText callback for tool-use loops
- **telegram**: Send intermediate LLM text to user
- **whatsapp**: Send intermediate LLM text to user

### Fixed

- **telegram**: Prevent ack model from generating questions
- **tools**: Increase delegate_task timeout to 15 minutes

## [0.4.0] (2026-03-25)

### Added

- **config**: Add multi-account WhatsApp config types and helpers
- **config**: Add per-account WhatsApp linked state helpers
- **channel**: Add WhatsApp channel with text-based approval flow
- **channel**: Add WhatsApp event handler with message filtering
- **scheduler**: Add WhatsApp fields to ChannelMeta
- **channel**: Add WhatsApp session manager
- **channel**: Add WhatsApp pusher for scheduled notifications
- **daemon**: Add multi-account WhatsApp sync support
- **server**: Add WhatsApp channel startup in server mode
- **cli**: Add --account flag and accounts list command

### Fixed

- **channel**: Add compile-time Channel interface check

## [0.3.0] (2026-03-23)

### Added

- **gws**: Add layered parameter passing for helper commands

## [0.2.1] (2026-03-23)

### Fixed

- **ci**: Replace deprecated macos-13 runner with cross-compilation
- **ci**: Only auto-bump patch version, not minor
- **ci**: Prevent auto-bump of major version for breaking changes
- **ci**: Restore default minor bump for feat commits

## [0.2.0] (2026-03-22)

### Added

- **agent**: Check context cancellation at iteration boundary
- **tools**: Add cancel support to TaskTracker
- **tools**: Register cancel func in delegate task async
- **telegram**: Add Interrupter interface
- **telegram**: Implement kill switch in SessionManager
- **telegram**: Add callback routing and interrupt channels
- **telegram**: Add interrupt and kill handling to Poller

### Fixed

- **ci**: Remove [skip ci] that blocks release workflow
- **server**: Wire Interrupter into Poller constructor
- **telegram**: Send feedback when /kill races with agent completion

## [0.1.0] (2026-03-22)

### Added

- **memory**: Add memory source types
- **memory**: Add database schema
- **memory**: Add store queries
- **memory**: Add capture logic for transcript parsing
- **memory**: Implement source interface
- **config**: Add memory source configuration
- **memory**: Add capture cli command
- **memory**: Add memory cli group
- **config**: Add whatsapp config section
- **whatsapp**: Add type definitions
- **whatsapp**: Add database schema and migration
- **whatsapp**: Add whatsmeow client wrapper
- **whatsapp**: Add qr code web auth flow
- **whatsapp**: Add database operations
- **whatsapp**: Add message sync engine with history support
- **whatsapp**: Add source struct and registration
- **whatsapp**: Add cli commands for auth, sync, and queries
- **cli**: Register whatsapp command and status integration
- **whatsapp**: Redesign qr auth page with instructions and better ux
- **cli**: Register memory commands
- **cli**: Add memory to status display
- **plugin**: Add claude code plugin manifest
- **plugin**: Add memory capture hook
- **assistant**: Add personal assistant persona
- **assistant**: Add stop hook for memory capture
- **assistant**: Add data access skills for email, whatsapp, and memory
- **gmail**: Add compose types for send and draft operations
- **gmail**: Add compose, send, and draft creation logic
- **gmail**: Upgrade OAuth scopes to include compose permission
- **gmail**: Add Send and CreateDraft methods to Gmail struct
- **gmail**: Add CLI send command
- **gmail**: Add CLI drafts create command
- **gmail**: Register send and drafts CLI commands
- **whatsapp**: Add SendInput and SendResult types
- **whatsapp**: Add SendText function for sending messages
- **whatsapp**: Add CLI send command
- **whatsapp**: Add whatsapp-send skill
- **gmail**: Add compose types for send and draft operations
- **gmail**: Add compose, send, and draft creation logic
- **gmail**: Upgrade OAuth scopes to include compose permission
- **gmail**: Add Send and CreateDraft methods to Gmail struct
- **gmail**: Add CLI send command
- **gmail**: Add CLI drafts create command
- **gmail**: Register send and drafts CLI commands
- **config**: Add DaemonConfig and JobsDBPath for daemon infrastructure
- **daemon**: Add Mode type with standalone/worker parsing
- **daemon**: Add Gmail sync and Reminder River job workers
- **daemon**: Add River client initialization with SQLite driver
- **daemon**: Add WhatsApp sync goroutine management
- **daemon**: Add daemon orchestrator with River and WhatsApp lifecycle
- **cli**: Add 'obk daemon' command with mode flag
- **service**: Add platform detection and service manager interface
- **service**: Add macOS launchd install/uninstall/status
- **service**: Add Linux systemd install/uninstall/status
- **cli**: Add 'obk service install/uninstall/status' commands
- **provider**: Add Provider interface for shared auth credentials
- **provider/google**: Add unified token store with scope tracking
- **provider/google**: Add dbTokenSource for auto-refresh persistence
- **provider/google**: Add OAuth flow with email extraction from userinfo
- **provider/google**: Implement Google provider with incremental auth
- **config**: Add ProviderDir and EnsureProviderDir path helpers
- **config**: Add ProvidersConfig types and accessor methods
- **cli/auth**: Add top-level auth command group for google and whatsapp
- **cli**: Wire auth command group to root command
- **migrate**: Add migration from legacy gmail layout to provider layout
- **cli**: Add auto-migration hook to root PersistentPreRun
- **gmail**: Add default 7-day sync window and Before support
- **cli/gmail**: Add --days flag to sync command
- **cli/gmail**: Add fetch command for on-demand email retrieval
- **tty**: Add TTY detection for interactive-only commands
- **cli/auth**: Add interactive TUI for Google scope management
- **cli**: Add interactive setup wizard for first-time configuration
- **cli/config**: Add providers.google.credentials_file config key
- **auth**: Add gmail.compose scope alias and interactive choice
- **assistant**: Add email-send skill for Gmail send and drafts
- **whatsapp**: Add Close() method to Client
- **whatsapp**: Add waitForHistorySync helper
- **whatsapp**: Add syncing state to auth page UI
- **whatsapp**: Expose syncing state in /api/qr endpoint
- **whatsapp**: Wait for history sync after QR auth
- **gmail**: Add SyncState type
- **gmail**: Add sync state schema
- **gmail**: Add sync state store functions
- **gmail**: Add History API fetcher functions
- **gmail**: Use History API for incremental sync
- **whatsapp**: Add contacts table schema and Contact type
- **whatsapp**: Add contact store functions
- **whatsapp**: Sync contacts from whatsmeow on connect
- **whatsapp**: Add contacts list CLI command
- **daemon**: Add file lock to prevent multiple instances
- **applenotes**: Add types for Apple Notes source
- **applenotes**: Add DB schema with dual SQLite/Postgres support
- **applenotes**: Add store CRUD for notes and folders
- **applenotes**: Add AppleScript execution layer
- **applenotes**: Add sync logic
- **applenotes**: Add Source interface implementation
- **config**: Add AppleNotes config with storage defaults
- **cli**: Add applenotes command with sync, list, get, search
- **cli**: Wire applenotes into root and status commands
- **daemon**: Add Apple Notes periodic sync goroutine
- **daemon**: Wire Apple Notes sync into daemon Run()
- **config**: Add Enabled field to all source configs
- **daemon**: Skip sync for sources that are not enabled
- **config**: Add per-source config.json with linked state
- **daemon**: Guard all syncs behind IsSourceLinked check
- **cli**: Link sources on first sync or auth
- **setup**: Add Apple Notes to guided setup wizard
- **assistant**: Add applenotes-read skill for AI queries
- **skills**: Add go:embed directive for built-in skills
- **skills**: Add manifest types and read/write logic
- **config**: Add Integrations config for gws services
- **skills**: Add declarative install logic with gws support
- **setup**: Add gws service selection and skill installation
- **cli**: Add obk update command
- **assistant**: Add gws bash permission to settings.json
- **makefile**: Add skills install and symlink creation
- **skills**: Add skill index generation
- **skills**: Regenerate index after Install
- **provider**: Define LLM provider interface and types
- **provider**: Implement Anthropic provider
- **config**: Add models configuration
- **provider**: Add keychain, registry, and model routing
- **agent**: Implement core agent loop
- **agent**: Add tool registry
- **agent**: Implement bash tool
- **agent**: Implement file read/write/edit tools
- **agent**: Implement load_skills and search_skills tools
- **channel**: Define Channel interface
- **channel**: Implement CLI chat channel
- **cli**: Add obk chat command
- **provider**: Implement OpenAI provider
- **provider**: Add Vertex AI support to Anthropic provider
- **provider**: Add WithTokenSource option for Vertex AI auth
- **provider**: Implement Gemini provider
- **agent**: Add Gemini to integration tests and register factory
- **provider**: Add Vertex AI support to Gemini provider
- **config**: Add RequireSetup check for LLM model configuration
- **cli**: Require setup before running chat and daemon
- **config**: Add Vertex AI fields to ModelProviderConfig
- **provider**: Extract GcloudTokenSource to shared package
- **cli**: Add gcloud helper for account/project listing
- **cli**: Add LLM model setup TUI flow
- **cli**: Integrate LLM Models into obk setup wizard
- **config**: Add RequireSetup check for LLM model configuration
- **cli**: Require setup before running chat and daemon
- **service**: Add Start and Stop methods to Manager interface
- **cli**: Add start, stop, and restart service subcommands
- **service**: Add Windows service manager stub
- **agent**: Add ScrubCredentials function
- **agent**: Apply credential scrubbing to tool output
- **tools**: Cap tool output at 512KB in registry
- **provider**: Add APIError type and error classification
- **provider**: Add ResilientProvider with retry logic
- **provider**: Wrap providers with resilience in registry
- **agent**: Add maxHistory field and WithMaxHistory option
- **agent**: Add compactHistory method
- **cli**: Add doctor command for installation health checks
- **provider**: Add RateLimiter using x/time/rate
- **agent**: Add WithRateLimit option and rate check
- **memory**: Create memory package with types
- **memory**: Create DB schema with migrate
- **memory**: Create CRUD store operations
- **config**: Add UserMemoryConfig for personal memory
- **cli**: Add obk memory list/add/delete commands
- **memory**: Add FormatForPrompt for prompt injection
- **memory**: Create extraction pipeline
- **memory**: Create reconciliation module
- **cli**: Add obk memory extract command
- **assistant**: Run memory extraction after history capture
- **skills**: Add memory-save skill for personal memory
- **cli**: Add user_memory DB to doctor check
- **cli**: Save obk chat conversations to history DB
- **config**: Add Mode, Remote, Auth, Channels types and SourceDataDSN
- **cli**: Add obk db command for SQL queries
- **server**: Add basic auth middleware
- **server**: Add health endpoint
- **server**: Add DB proxy endpoint
- **server**: Add memory CRUD endpoints
- **server**: Add server struct, routes, and Run
- **cli**: Add obk server command
- **daemon**: Add Option for conditional sync
- **deps**: Add go-telegram-bot-api v5
- **telegram**: Implement Channel interface
- **telegram**: Add long-polling loop with owner filter
- **telegram**: Add session manager with history and memory
- **server**: Wire Telegram channel into server
- **remote**: Add Client with DB, Health, and Memory methods
- **cli**: Add mode helper for remote client
- **cli**: Add remote proxying to db command
- **cli**: Add remote proxying to memory commands
- **remote**: Add AppleNotesPush method
- **server**: Add Apple Notes push endpoint
- **daemon**: Add bridge mode for Apple Notes push
- **cli**: Add --bridge flag to service run
- **envload**: Add .env loader package without dependency cycles
- **deps**: Add testcontainers-go for Docker E2E tests
- **servertest**: Add backend-agnostic server test suite
- **server**: Add Gmail and WhatsApp API endpoints
- **remote**: Add Gmail and WhatsApp client methods
- **cli**: Add remote proxying to Gmail commands
- **cli**: Add remote proxying to WhatsApp send
- **cli**: Add remote deployment path to setup
- **cli**: Add remote proxying to status command
- **server**: Add POST /api/memory/extract endpoint
- **remote**: Add MemoryExtract method and CLI proxy routing
- **telegram**: Add session timeout and async memory extraction
- **bridge**: Track last push timestamp to skip unchanged notes
- **whatsapp**: Export AuthPage and WaitForSync helpers
- **server**: Add WhatsApp QR auth handler
- **remote**: Add WaitWhatsAppAuth polling method
- **cli**: Add remote mode to WhatsApp auth login
- **spectest**: Add fixture interface and data types
- **spectest**: Add assertion helpers
- **spectest**: Add local fixture implementation
- **spectest**: Add WhatsApp and memory fixture types
- **spectest**: Add LLM judge assertion layer
- **spectest**: Add WhatsApp/memory DB setup and Given helpers
- **spectest**: Add multi-provider support with EachProvider helper
- **tools**: Load_skills reads REFERENCE.md for full skill content
- **skills**: Split GWS skills into SKILL.md and REFERENCE.md at install
- **tools**: Add SubagentTool for delegating tasks to child agents
- **telegram**: Wire SubagentTool into Telegram session
- **cli**: Wire SubagentTool into CLI chat
- **prompt**: Add sub-agent usage guidance to system prompts
- **tools**: Add Has and ToolNames methods to Registry
- **tools**: Add BuildBaseSystemPrompt to auto-generate prompt from registry
- **provider**: Add SystemBlock, CacheControl types and cache usage fields
- **anthropic**: Add prompt caching support
- **openai**: Parse cached tokens and support SystemBlocks
- **gemini**: Add CachedContent lifecycle and cache usage parsing
- **prompt**: Add BuildSystemBlocks for structured system prompts
- **agent**: Add WithSystemBlocks option
- **usage**: Add usage tracking store with SQLite schema
- **agent**: Add UsageRecorder interface and option
- **config**: Add usage storage config
- **cli**: Wire up usage recording in CLI channel
- **telegram**: Wire up usage recording in Telegram channel
- **cli**: Add obk usage command with daily/monthly views
- **imessage**: Add types and config
- **imessage**: Add source interface
- **imessage**: Add database schema
- **imessage**: Add store CRUD functions
- **imessage**: Add chat.db reader
- **imessage**: Add sync orchestrator
- **config**: Add imessage config and DSN helper
- **cli**: Add imessage root command
- **cli**: Add imessage sync command
- **cli**: Add imessage query commands
- **cli**: Register imessage command
- **daemon**: Add imessage periodic sync
- **daemon**: Wire imessage into daemon
- **cli**: Add imessage to status command
- **remote**: Add IMessagePush client method
- **daemon**: Add imessage to bridge
- **server**: Add imessage push handler
- **server**: Register imessage route and migration
- **oauth**: Add ScopeWaiter for async auth completion signaling
- **oauth**: Add AuthURL for progressive consent URL generation
- **oauth**: Add ExchangeCode for server-side token exchange
- **oauth**: Add AccessToken for raw token string retrieval
- **channel**: Add SendLink method to Channel interface
- **telegram**: Implement SendLink with inline URL button
- **cli**: Implement SendLink for terminal output
- **tools**: Define Interactor interface for tool-to-user side channel
- **channel**: Add Interactor adapter bridging Channel to tools.Interactor
- **tools**: Add GuardedWrite reusable approval helper
- **tools**: Add TokenBridge for gws token injection
- **tools**: Add CommandRunner interface and GWSRunner implementation
- **tools**: Add ScopeChecker interface for scope queries
- **tools**: Add GWSExecuteTool struct and schema
- **server**: Create ScopeWaiter and Google auth on startup
- **server**: Add Google OAuth callback handler for progressive consent
- **server**: Register /auth/google/callback route
- **telegram**: Register GWSExecuteTool in agent creation
- **skills**: Add RefreshGWSSkills for version/scope-triggered refresh
- **server**: Refresh gws skills on startup
- **server**: Trigger skill refresh after Google OAuth callback
- **tools**: Add gws_execute instructions to system prompt
- **finance**: Add types and source interface
- **finance**: Add Yahoo Finance HTTP client
- **finance**: Add CLI commands for quote and rate lookup
- **cli**: Register finance command in root
- **cli**: Register finance source in status command
- **config**: Add WebSearchConfig with proxy, timeout, cache_ttl
- **websearch**: Add source scaffolding with types, schema, engine interface
- **websearch**: Add DuckDuckGo and Wikipedia search engines
- **websearch**: Add search orchestrator and URL fetcher with SSRF protection
- **cli**: Add obk websearch search and fetch commands
- **websearch**: Register source in status and add web-search/web-fetch skills
- **skills**: Add memory-read skill for recalling saved memories
- **contacts**: Add domain types for unified contact store
- **contacts**: Add database schema with dual SQLite/Postgres support
- **contacts**: Add phone/email normalization helpers
- **contacts**: Add CRUD store operations for contacts, identities, aliases
- **contacts**: Add fuzzy search with frequency-based ranking
- **contacts**: Add ContactsConfig, DSN method, and defaults to config
- **contacts**: Add WhatsApp sync adapter
- **contacts**: Add Gmail sync adapter
- **contacts**: Add iMessage sync adapter
- **contacts**: Add Apple Contacts sync adapter (macOS)
- **contacts**: Add sync orchestrator and fix nullable column handling
- **contacts**: Add CLI commands for search, get, list, sync
- **contacts**: Register contacts CLI command in root
- **contacts**: Add background sync goroutine to daemon
- **contacts**: Add contacts-search skill definition and reference
- **contacts**: Register contacts-search as always-eligible skill
- **contacts**: Add contacts migration to server startup
- **contacts**: Add JSON tags to exported types
- **websearch**: Add Brave search engine
- **websearch**: Add Mojeek search engine
- **websearch**: Add Yahoo search engine
- **websearch**: Add Yandex search engine
- **websearch**: Add Google search engine
- **websearch**: Register new engines in buildEngines
- **websearch**: Add Date/Image to Result and NewsEngine interface
- **websearch**: Add DuckDuckGo news search
- **websearch**: Add Yahoo news search
- **websearch**: Add news search orchestrator
- **websearch**: Add CLI news command
- **websearch**: Add WithDB option and CacheTTL config
- **websearch**: Add search_cache and fetch_cache table schemas
- **websearch**: Add cache store functions and NoCache option
- **websearch**: Integrate cache into Search, News, and Fetch
- **websearch**: Add --no-cache flag and DB wiring to CLI commands
- **websearch**: Add backends list command
- **spectest**: Add ContactFixture type and GivenContacts to Fixture interface
- **spectest**: Add contacts DB setup, skills, and GivenContacts implementation
- **spectest**: Add OBK_TEST_PROVIDER env var to filter to single provider
- Add Slack integration with agent tools and desktop auth
- **websearch**: Add search history recording and cache clearing
- **websearch**: Add relevance ranking with frequency and token scoring
- **websearch**: Add backend health tracking with cooldown
- **websearch**: Integrate health tracking into search orchestration
- **websearch**: Add category column to search_history schema
- **websearch**: Add history and cache clear CLI commands
- **websearch**: Add backends config field to filter auto engine set
- **websearch**: Add httpclient package with UA rotation and rate limiting
- **websearch**: Add pagination support with --page flag
- **websearch**: Add Bing search engine (disabled by default)
- **websearch**: Concurrent multi-backend dispatch with errgroup
- **tools**: Add AgentRunner for external CLI detection and execution
- **tools**: Add delegate_task tool with approval flow
- **tools**: Register delegate_task in CLI and Telegram
- **tools**: Add delegate_task system prompt section
- **tools**: Add TaskTracker for background task management
- **tools**: Add async mode to delegate_task
- **tools**: Add check_task tool
- **tools**: Register check_task and wire TaskTracker
- **tools**: Update delegate_task prompt for async mode
- **tools**: Add StreamRunner for agent CLI streaming output
- **tools**: Add structured spec support to delegate_task
- **tools**: Add progress reporting for async delegate_task
- **tools**: Update system prompt for multi-step workflows
- **tools**: Add Codex CLI support for delegate_task
- **applenotes**: Add CheckPermission probe function
- **contacts**: Add CheckAppleContactsPermission probe function
- **setup**: Add Apple Contacts to source selection list
- **setup**: Add permission probe to setupAppleNotes
- **setup**: Add setupAppleContacts with permission probe
- **scheduler**: Add schedule types and ChannelMeta struct
- **scheduler**: Add schema and migration for schedules table
- **scheduler**: Add CRUD store operations for schedules
- **config**: Add SchedulerConfig with storage DSN and defaults
- **channel**: Add Pusher interface for outbound message delivery
- **channel**: Add TelegramPusher for scheduled task delivery
- **channel**: Add SlackPusher for scheduled task delivery
- **tools**: Add AutoApproveInteractor for headless scheduled tasks
- **channel**: Add PusherFactory to create channel-specific pushers
- **daemon**: Add ScheduledTaskWorker River job for running tasks
- **daemon**: Register ScheduledTaskWorker in River client
- **daemon**: Add Scheduler with cron, one-shot polling, and reload
- **daemon**: Wire Scheduler into Daemon lifecycle
- **tools**: Add create_schedule, list_schedules, delete_schedule tools
- **tools**: Add scheduled tasks section to system prompt
- **telegram**: Register schedule tools and add scheduler migration
- **safety**: Add safety section to system prompt
- **safety**: Add bash command allowlist/blocklist filter
- **safety**: Add filter and workdir options to BashTool
- **safety**: Apply default blocklist to standard registry bash
- **safety**: Add restricted registry for scheduled tasks
- **safety**: Add AuditDBPath to config
- **safety**: Add audit trail package
- **safety**: Wire audit logging into tool registry
- **safety**: Add content boundary markers and injection scanner
- **safety**: Wrap untrusted tool output with boundary markers
- **safety**: Add risk level classification for tools
- **safety**: Add risk-aware GuardedAction function
- **safety**: Migrate slack_react to RiskLow (auto-approve)
- **safety**: Migrate gws_execute writes to RiskHigh
- **safety**: Wire audit logger into CLI, Telegram, and scheduled tasks
- **safety**: Add session-scoped auto-approve rules with rubber-stamp detection
- **safety**: Integrate approval rules and rubber-stamp detection into GuardedAction
- **safety**: Upgrade injection scanner with base64 and homoglyph detection
- **safety**: Add BatchInteractor interface and batch approval helper
- **tools**: Wire ApprovalRules into slack_send, slack_edit, gws_execute, delegate_task
- **tools**: Add WebSearcher interface and WebToolDeps
- **tools**: Add web_search tool
- **tools**: Add web_fetch tool with sub-agent summarizer
- **tools**: Mark web tool output as untrusted
- **tools**: Add Web section to system prompt
- **cli**: Register web tools in chat command
- **telegram**: Register web tools in session
- **agent**: Add WithHistory option and wire conversation context in Telegram
- **config**: Add timezone setting and use it in Telegram system prompt
- **config**: Add timezone to config set command and expose in agent prompt
- **server**: Add service management subcommands
- **cli**: Add logs command for daemon and server
- **make**: Restart running services after install
- **config**: Add CallbackURL and NgrokDomain fields to GWSConfig
- **oauth**: Parameterize loadConfig redirect URL for public callbacks
- **cli**: Add ngrok setup wizard for public OAuth callbacks
- **cli**: Integrate ngrok tunnel setup into GWS setup flow
- **cli**: Add Telegram setup and restructure setup flow order
- **tools**: Add per-tool truncation helpers
- **tools**: Apply per-tool truncation to bash
- **tools**: Apply per-tool truncation to gws_execute
- **tools**: Apply per-tool truncation to web and slack tools
- **config**: Add scratch directory helpers
- **tools**: Add file fallback for large tool output
- **telegram**: Thread scratch dir through sessions
- **agent**: Add microcompaction for stale tool results
- **agent**: Wire microcompact into agent loop
- **cli**: Thread scratch dir through chat command
- **history**: Add LoadRecentSession query
- **history**: Add LoadSessionMessages query
- **telegram**: Restore recent session on startup
- **telegram**: Handle /start command to reset session
- **provider**: Add context window lookup for known models
- **config**: Add context_window and compaction_threshold fields
- **agent**: Add Summarizer interface and compaction options
- **agent**: Implement LLM-based conversation summarizer
- **telegram**: Wire context window and summarizer into session
- **provider**: Add prefix matching to DefaultContextWindow
- **telegram**: Add Request and MakeRequest to botSender interface
- **telegram**: Add processingFeedback for natural typing behavior
- **telegram**: Add notifyingExecutor wrapper for tool start callbacks
- **telegram**: Wire processingFeedback into session message handling
- **provider**: Add TierNano with fallback cascade
- **tools**: Add ResolveNanoProvider helper
- **telegram**: Use nano tier for feedback and memory reconciliation
- **provider**: Add Groq and OpenRouter providers
- **provider**: Add OpenRouter provider
- **imports**: Register groq and openrouter provider factories
- **config**: Add profile system with starter/standard/premium
- **cli**: Add profile selection and nano tier to setup models
- **config**: Add Category field to ModelProfile and yaml tags to ProfileTiers
- **config**: Add gemini single-provider profile
- **config**: Add anthropic single-provider profile
- **config**: Add openrouter and openai single-provider profiles
- **config**: Add CustomProfile type to ModelsConfig
- **config**: Add model catalog with tier recommendations
- **config**: Add ValidateProfileName for custom profiles
- **cli**: Add obk config profiles list command
- **cli**: Add obk config profiles show command
- **cli**: Add obk config profiles create TUI builder
- **cli**: Add obk config profiles delete command
- **cli**: Integrate custom profiles into obk setup models
- **config**: Add groq open-source single-provider profile
- **agent**: Add full-permission flags to delegate_task CLI agents
- **history**: Add ended_at column to history_conversations
- **delegate**: Write results to scratch file instead of inline
- **setup**: Add iMessage to source picker on macOS
- **setup**: Add timezone prompt when not configured
- **gmail**: Move Google auth commands into gmail package
- **whatsapp**: Move WhatsApp auth commands into whatsapp package
- **slack**: Move Slack auth commands into slack package
- **cli**: Add --json flag to commands missing structured output
- **cli**: Add confirmation prompts to destructive commands
- **obkmacos**: Add Swift helper binary for native macOS integrations
- **obkmacos**: Add Go bridge package for Swift helper binary
- **setup**: Add obk setup applecontacts and applenotes subcommands
- **install**: Add cross-platform install script
- **website**: Add Astro project setup with Tailwind CSS 4
- **website**: Add layout, global styles, and toolbox favicon
- **website**: Add nav, hero, and mission components
- **website**: Add CTA, footer, and index page
- **website**: Use one-liner installer, add capabilities nav link
- **website**: Add capabilities page with integrations, safety, and demo
- **website**: Add terminal UI component for install section
- **website**: Use terminal UI for capabilities install section
- **website**: Add copy button to terminal, keep commands on single line
- **website**: Add built-by credit line to footer
- **website**: Add captions below architecture and demo images
- **website**: Highlight active nav link on capabilities page
- **website**: Add support section with GitHub star link
- **tty**: Add platform-specific isTerminal using stdlib syscall
- **tools**: Add three-outcome filter with FilterResult type
- **tools**: Add approval gate to BashTool for non-allowlisted commands
- **cli**: Add CLIInteractor for terminal approval prompts
- **tools**: Add approval gates to file_write, file_edit, and registry
- **tools**: Add dir_explore tool for safe directory exploration
- **tools**: Add content_search tool for regex file searching
- **tools**: Add sandbox runtime infrastructure (Seatbelt/bwrap)
- **tools**: Add sandbox_exec tool for sandboxed code execution
- **tools**: Register new tools and update prompt/sanitize
- **settings**: Add settings service with tree registry and tests
- **settings**: Add bubbletea TUI for settings browser
- **cli**: Add obk settings command
- **settings**: TUI uses dynamic options and shows verify status
- **cli**: Inject provider verification into settings command
- **settings**: Add EditFunc, ReadOnly fields and RebuildTree method
- **settings**: Add huh form helpers for profile wizard
- **settings**: Profile-first model configuration flow
- **settings**: TUI quit-wizard-restart pattern
- **config**: Add ModelsDir path helper for model cache
- **provider**: Add ListModels and VerifyModelAccess
- **provider**: Add ModelCache for local model list caching
- **tui**: Inline profile wizard with state machine
- **server**: Add auth redirect trampoline handler
- **server**: Register GET /auth/redirect route
- **tools**: Wrap OAuth URL with auth redirect trampoline
- **telegram**: Wire auth redirect URL from callback config
- **config**: Add LearningsDir() path helper
- **learnings**: Add git-backed markdown store
- **tools**: Add learnings save/read/search/extract tools
- **prompt**: Add learnings section to system prompt
- **server**: Add GET /learnings/{topic} HTTP endpoint
- **cli**: Add learnings open/list/search subcommands
- **cli**: Register learnings command in root
- **telegram**: Register learnings tools in session agent
- **cli**: Register learnings tools in chat command
- **jobs**: Classify errors for adaptive retry timing
- **jobs**: Cancel non-retryable jobs with river.JobCancel
- **scheduler**: Increase MaxAttempts to 3
- **tools**: Add scrubEnv to remove sensitive env vars
- **tasks**: Add schema and migration
- **tasks**: Add CRUD store functions
- **tasks**: Add cleanup with 7-day retention
- **config**: Add tasks storage config
- **cli**: Wire persistent task tracker
- **telegram**: Wire persistent task tracker
- **delegate**: Add audit logging with context=delegated
- **provider**: Extract shared pricing map
- **agent**: Add BudgetTracker and BudgetChecker
- **agent**: Add WithBudgetChecker option to agent loop
- **scheduler**: Update store for budget columns
- **provider**: Export Router.Resolve method
- **jobs**: Use model tier and budget in scheduled task worker
- **tools**: Add model_tier and max_budget_usd to schedule tools
- **scheduler**: Add reactive type and trigger fields to types
- **scheduler**: Add trigger columns to schema
- **scheduler**: Update store for trigger columns
- **scheduler**: Add trigger validation and query builder
- **daemon**: Add SyncNotifier for sync completion signals
- **daemon**: Wire sync notifier into all sync goroutines
- **scheduler**: Add reactive check loop
- **tools**: Add reactive type to schedule tools
- **cerebras**: Add Cerebras provider
- **cerebras**: Register env var and model listing
- **cerebras**: Wire provider into CLI setup
- **provider**: Add context windows and pricing for new models
- **profiles**: Add free profile using Gemini + Cerebras
- **provider**: Add Z.AI GLM provider
- **provider**: Register ZAI_API_KEY env var
- **provider**: Add GLM model context windows
- **provider**: Add Z.AI model listing
- **config**: Add Z.AI profile with glm-4.5-flash default
- **settings**: Add Z.AI to provider settings UI
- **config**: Add BackupConfig, R2Config, GDriveConfig structs and backup paths
- **backup**: Add Backend interface and local filesystem implementation
- **backup**: Add manifest types, load/save, and diff logic
- **backup**: Add file scanner with include/exclude rules
- **backup**: Add VACUUM INTO helper for safe SQLite snapshots
- **backup**: Add core backup Run() flow
- **backup**: Add R2/S3 storage backend using minio-go
- **backup**: Add Google Drive storage backend
- **backup**: Add restore, list snapshots, and get manifest
- **backup**: Add setupBackup to obk setup wizard
- **backup**: Add backup category to settings TUI
- **backup**: Add CLI commands (now, list, status, restore)
- **backup**: Add daemon periodic backup job
- **settings**: Add backup wizard flow — destination → credentials → verify → schedule
- **backup**: Add R2 field descriptions and remove GDrive folder prompt
- **settings**: Hide R2 category when destination is not R2
- **settings**: Add triggerBackup callback and IsBackupDestConfigured
- **backup**: Transactional wizard with rollback and auto-trigger
- **backup**: Wire up triggerBackup callback in settings TUI
- **backup**: Remove schedule prompt from obk setup
- **website**: Add user memory to agent harness section
- **website**: Mention remote deployment in how-it-works
- **website**: Add backup and restore section
- **website**: Add changelog page
- **website**: Add changelog link to navigation

### Changed

- Strip unnecessary comments and docstrings
- **plugin**: Remove plugin in favor of directory hooks
- **gmail**: Use provider interface instead of direct OAuth
- **cli/gmail**: Wire sync and auth to Google provider
- **cli**: Wire status command to Google provider
- **gmail**: Remove old tokenstore (moved to provider/google)
- **cli**: Remove old auth commands from gmail and whatsapp
- **gmail**: Adapt send/draft to provider-based auth
- **cli**: Remove os.Remove from CLI logout
- **daemon**: Remove mode concept (standalone/worker)
- **cli**: Move install/uninstall/status under daemon command
- **config**: Remove stale Mode field from DaemonConfig
- **config**: Remove Enabled field from source configs
- **skills**: Move built-in skills to project root
- **oauth**: Move provider/ to oauth/ directory
- **oauth**: Update all import paths to oauth/google
- **skills**: Extract email-read schema to schema.sql
- **skills**: Extract whatsapp-read schema to schema.sql
- **skills**: Extract memory-read schema to schema.sql
- **skills**: Extract applenotes-read schema to schema.sql
- **skills**: Embed all skill files and copy during install
- **provider**: Use shared GcloudTokenSource in tests
- **provider**: Change Factory to accept ModelProviderConfig
- **cli**: Rename daemon command to service
- **store**: Switch to modernc sqlite driver
- **platform**: Add cross-platform shutdown signals
- **cli**: Use platform shutdown signals
- **daemon**: Split file locking by platform
- **provider**: Cross-platform credential storage
- **provider**: Use go-keyring with file-based fallback
- **agent**: Fix struct field alignment
- **cli**: Load config once in doctor and use deterministic db order
- **provider**: Rename KeychainLoad/KeychainStore to LoadCredential/StoreCredential
- **cli**: Use renamed StoreCredential function
- **provider**: Remove file-based fallback, use keyring only
- **oauth**: Remove redundant comments from tokenstore and tokensource
- **provider**: Remove redundant comments from anthropic provider
- **whatsapp**: Remove redundant comment from Close method
- **cli**: Remove redundant section comments from setup flow
- **daemon**: Remove redundant lock function comments
- **provider**: Remove redundant comments from credential helpers
- **provider**: Remove redundant comments from error classification
- **provider**: Remove redundant comments from ratelimit and resilient
- **agent**: Remove redundant comments from scrub and compact
- **source**: Rename memory package to history
- **config**: Rename MemoryConfig to HistoryConfig
- **cli**: Rename memory command to history
- **cli**: Update status.go for history rename
- **skills**: Rename memory-read skill to history-read
- Update tests and docs for history rename
- **memory**: Introduce LLM interface for testability
- **skills**: Migrate read skills to obk db
- **skills**: Migrate send skills to obk db
- **testutil**: Delegate LoadEnv to envload package
- **server**: Rewrite local E2E tests to use servertest suite
- **server**: Rewrite Docker test to use servertest suite
- **test**: Remove redundant integration tests
- **testutil**: Remove unused helpers
- **remote**: Remove unused AppleNoteItem struct
- **telegram**: Migrate from log to log/slog
- **daemon**: Migrate from log to log/slog
- **server**: Migrate from log to log/slog
- **sources**: Migrate from log to log/slog
- **server**: Run DB migrations once at startup
- **spectest**: Simplify to single-prompt autonomous orchestration
- **cli**: Rewrite system prompt with structured tool and skill rules
- **telegram**: Rewrite system prompt to match CLI structure
- **spectest**: Align system prompt with production CLI prompt
- **skills**: Add REFERENCE.md files for all built-in skills
- **skills**: Slim down SKILL.md to summary and pointer
- **server**: Rename e2e tests to server API contract tests
- **agent**: Rename integration tests to provider conformance tests
- **memory**: Rename integration tests to describe what they verify
- **telegram**: Rename session tests to describe what they verify
- **tools**: Extract NewStandardRegistry to eliminate duplication
- **cli**: Use BuildBaseSystemPrompt instead of local copy
- **telegram**: Use BuildBaseSystemPrompt instead of local copy
- **spectest**: Use BuildBaseSystemPrompt instead of local copy
- **cli**: Use BuildSystemBlocks for cache-optimized system prompt
- **telegram**: Use BuildSystemBlocks for cache-optimized system prompt
- **spectest**: Use BuildSystemBlocks for cache-optimized system prompt
- **gemini**: Extract buildToolDeclarations helper
- **server**: Wire Interactor and auth deps into SessionManager
- **setup**: Use obk auth for GWS instead of gws auth login
- **finance**: Use errors.New for sentinel error
- **browser**: Extract shared Chrome TLS transport from finance
- **websearch**: Remove unused cache tables and fields
- **websearch**: Remove CacheTTL config and --no-cache CLI flags
- **websearch**: Replace stderr logging with slog in search
- **websearch**: Deduplicate Chrome User-Agent constant
- **skills**: Make memory-save description save-only
- **spectest**: Update tests to use fixture's AssertJudge method
- **websearch**: Replace dedup with rankResults in search orchestrator
- **websearch**: Use HTTPDoer interface and httpclient for engines
- **websearch**: Move normalizeURL to rank.go
- **daemon**: Add PusherFactory and AgentRunner hooks to worker
- **audit**: Deduplicate openAuditLogger into audit.OpenDefault
- **tools**: Remove unused ToolRiskLevel function and map
- **tools**: Remove unused BatchInteractor and batch approval
- **tools**: Extract shared web tool setup helpers
- **telegram**: Remove timezone from system prompt, use user memory instead
- **service**: Make service manager generic with named services
- **tools**: Lower registry safety net to 100KB
- **agent**: Add token-based compaction with LLM summarization
- **telegram**: Pass message ID through channel for reactions
- **gws**: Accept structured params/body instead of command string
- **gws**: Remove command normalization (dot expand, prefix strip)
- **gws**: Accept structured params/body instead of command string
- **telegram**: Inline Telegram HTML renderer, drop telegold dep
- **tools**: Extract truncation magic numbers to named constants
- **gmail**: Rename tables from gmail_emails to emails
- **gws**: Remove hardcoded API enablement, let agent handle it
- **cli**: Extract printProfileDetails to remove show duplication
- **history**: Remove auto-migration for ended_at column
- **cli**: Remove auth package, deregister from root
- **cli**: Rename config profiles show to describe
- **cli**: Rename config show to config list
- **cli**: Rename auth status to auth list across all providers
- **cli**: Normalize Short descriptions to "Manage X data source" pattern
- **cli**: Unify service and server into single `obk service` command
- **contacts**: Replace osascript with obkmacos for Apple Contacts
- **applenotes**: Replace osascript with obkmacos for Apple Notes
- **install**: Rewrite installer following industry patterns
- **tty**: Drop direct go-isatty import in favor of local isTerminal
- **usage**: Move from source/usage to service/usage
- **scheduler**: Move from source/scheduler to service/scheduler
- **history**: Move from source/history to service/history
- **contacts**: Move from source/contacts to service/contacts
- **memory**: Move from memory/ to service/memory/
- **history**: Remove dead source.Source implementation
- **status**: Group service/ imports separately from source/
- **tools**: Clarify DefaultBlocklist as legacy fallback
- **tools**: Combine duplicate file_write/file_edit extractPattern cases
- **settings**: Providers-first UX with dynamic model selects
- **settings**: Rename labels for clarity
- **settings**: Reorder profiles and simplify list labels
- **cli**: Use ListModels API for provider verification
- **settings**: Read models from cache, remove static ModelCatalog
- **settings**: Remove EditFunc field, add Save method
- **settings**: Remove dead wizard functions from registry
- **settings**: Delete wizard.go
- **tui**: Remove quit-restart loop
- **cli**: Rename shadowed openCmd variable to opener
- **tools**: Add NewPersistentTaskTracker with DB backing
- **cli**: Use shared pricing from provider package
- **tools**: Extract OpenPersistentTaskTracker to deduplicate
- **profiles**: Replace deprecated gemini-2.0-flash with gemini-2.5-flash-lite
- **profiles**: Reshuffle free plan tiers for better fit
- **audit**: Migrate audit log from SQLite to JSONL
- **audit**: Update callers to use AuditJSONLPath
- **usage**: Migrate usage records from SQLite to JSONL
- **usage**: Update callers to use JSONL path
- **websearch**: Migrate search history from SQLite to JSONL
- **websearch**: Update history tests and callers for JSONL
- **memory**: Migrate user memories from SQLite to Markdown files
- **memory**: Update CLI callers to use Markdown store
- **memory**: Update server callers to use Markdown store
- **memory**: Update telegram callers to use Markdown store
- **memory**: Update spectest to use Markdown store
- **history**: Rewrite core store from SQLite to JSONL
- **history**: Update capture and tests for JSONL store
- **history**: Update CLI callers to use JSONL store
- **history**: Update server and memory extract callers for JSONL
- **history**: Update telegram callers and tests for JSONL store
- **usage**: Rename Migrate to EnsureDir for consistency
- **zai**: Simplify integration test to single model
- **backup**: Extract shared ResolveBackend to eliminate triplication
- **settings**: Export BackupDest and remove duplicate in tui package
- **website**: Render changelog from CHANGELOG.md at build time

### Fixed

- **whatsapp**: Faster qr polling and delayed server shutdown for better ux
- **whatsapp**: Use correct sqlite dsn params for foreign keys
- **whatsapp**: Set device name to openbotkit instead of other device
- **whatsapp**: Connect before sending logout request
- **whatsapp**: Keep client connected longer for device key exchange
- **whatsapp**: Wait after logout so unlink request is processed
- **plugin**: Use plugin-root-relative hooks path
- **memory**: Increase scanner buffer for large transcripts
- **whatsapp**: Use modernc.org/sqlite pragma syntax in connection string
- **cli**: Resolve go vet warnings for redundant newlines in Println
- **gmail**: Remove duplicate types and methods from merge
- **whatsapp**: Delete session DB on logout to prevent stale state
- **whatsapp**: Show linking intermediate state during QR auth
- **service**: Update launchd/systemd to use daemon run subcommand
- **skills**: Add missing applenotes-read to embedded skills
- **skills**: Add applenotes-read to builtinSkills, generalize auth check
- **skills**: Avoid append mutation and handle gws copy errors
- **setup**: Use os.UserHomeDir, add timeout to gws wait loop
- **tests**: Use t.Setenv, check errors, add applenotes test
- **assistant**: Mention Apple Notes in CLAUDE.md description
- **skills**: Rewrite scope parsing to handle multi-scope lines
- **cli**: Pass selected account to gcloudProjects
- **cli**: Clean single-line output for setup-required error
- **gemini**: Skip integration test on invalid API key
- **agent**: Validate provider keys before running integration tests
- **service**: Update launchd plist to use service run
- **service**: Update systemd unit to use service run
- **docker**: Add multi-arch build support
- **test**: Skip gcloud tests when not authenticated
- **test**: Skip gcloud token test when not authenticated
- **test**: Check for gcloud accounts before listing projects
- **test**: Check for gcloud accounts before token test
- **daemon**: Store Windows lock handle to avoid Fd() duplication
- **test**: Use USERPROFILE on Windows and skip perm check
- **cli**: Remove keychain reference from error message
- **whatsapp**: Close client in test to release SQLite handle
- **whatsapp**: Close client in send test to release SQLite handle
- **test**: Handle context cancellation in daemon shutdown test
- **agent**: Prevent panic when maxHistory < keepMessages
- **agent**: Scrub Bearer tokens in authorization headers
- **provider**: Make ErrorPermanent the zero value for ErrorKind
- **agent**: Derive keep count from maxHistory instead of fixed constant
- **provider**: Propagate operational keyring errors instead of silent fallback
- **memory**: Scan timestamps as strings in Get()
- **test**: Correct stale memory-read references in error messages
- **skills**: Sync schema.sql with actual Go migration schema
- **cli**: Add missing EnsureSourceDir call in memory delete
- Update gemini model to 2.5-flash and increase MaxTokens
- **agent**: Load .env in integration tests for API keys
- **memory**: Load .env in integration tests for API keys
- **gemini**: Load .env in integration test for API key
- **envload**: Always override env vars from .env file
- **telegram**: Handle errors from SaveMessage and rand.Read
- **remote**: URL-encode category param, replace interface{} with any
- **docker**: Create /data source directories in Dockerfile
- **docker**: Provide config.yaml with model config in container
- **cli**: Guard against nil cfg.Remote in proxy commands
- **server**: Prevent SQL injection in DB proxy endpoint
- **server**: Require auth credentials to start server
- **server**: Add request body size limit (1MB)
- **server**: Stop leaking internal errors to API clients
- **telegram**: Serialize message handling and fix stale context
- **remote**: Remove fmt.Println from library code
- **telegram**: Guard against nil From field in updates
- **server**: Sanitize error messages and add gmail migration
- **spectest**: Improve system prompt for multi-source reliability
- **provider**: Add Name field to ToolResult
- **agent**: Set tool name on results for provider compatibility
- **gemini**: Collect all tool results and use function names
- **spectest**: Calibrate judge to not flag real data as fabricated
- **skills**: Update memory-save description to include recall capability
- **agent**: Set ToolResult.Name in integration test tool roundtrip
- **telegram**: Replace busy-wait spin loop with channel notification
- **whatsapp**: Widen timing upper bounds in sync tests
- **whatsapp**: Remove empty TestChatName stub
- **tests**: Replace real BashTool with stubTool in subagent tests
- **spectest**: Clarify judge criteria for memory+email correlation test
- **prompt**: Move telegram-specific conciseness rule out of shared prompt
- **tools**: Make ToolSchemas ordering deterministic
- **usage**: Close DB handle in Recorder to prevent leak
- **usage**: Use longest-match prefix for cost estimation
- **server**: Add usage migration to server startup
- **usage**: Add TOTAL label to cost summary row
- **tools**: Block gws commands in bash to enforce gws_execute
- **oauth**: Store scopes+account in ScopeWaiter for callback lookup
- **tools**: Use crypto/rand for OAuth state instead of timestamp
- **server**: Reject OAuth callback when no account is resolved
- **tools**: Remove keyword fallback from isWriteCommand, trust + prefix only
- **tools**: Propagate Notify/NotifyLink errors instead of dropping them
- **telegram**: Cache manifest at SessionManager creation instead of per-message
- **finance**: Use errors.Is, url.Values, and drain response body
- **finance**: Reduce TestQuoteTimeout sleep from 2s to 200ms
- **websearch**: Add User-Agent header to Wikipedia API requests
- **websearch**: Eliminate SSRF TOCTOU via dialer-level IP check
- **websearch**: Cap response body at 10 MB to prevent OOM
- **websearch**: Use url.Parse for GitHub URL normalization
- **websearch**: Use rune-based truncation to preserve UTF-8
- **websearch**: Use rune length for truncation guard
- **skills**: Remove hardcoded ~/.obk/ paths from SKILL.md files
- **spectest**: Upgrade OpenAI model from gpt-4o-mini to gpt-4.1-mini
- **skills**: Rewrite memory-save docs to emphasize recall over save
- **spectest**: Install memory-read skill in test fixture
- **spectest**: Use dedicated judge provider to avoid self-evaluation bias
- **contacts**: Remove dead code in NormalizePhone
- **contacts**: Remove unused UpsertContact with broken semantics
- **contacts**: Use transaction in MergeContacts and handle UNIQUE conflicts
- **contacts**: Wrap DeleteContact in a transaction
- **contacts**: Escape LIKE wildcards in search queries
- **contacts**: Fix WhatsApp interaction double-counting
- **contacts**: Log errors instead of silently ignoring in Apple Contacts sync
- **contacts**: Log SaveSyncState errors and use slices.Contains
- **contacts**: Prevent truncate panic on small max values
- **contacts**: Fix indentation and gate Apple Contacts sync on darwin
- **websearch**: Check format when serving fetch cache hits
- **websearch**: Preserve query params in dedup normalizeURL
- **websearch**: Store and retrieve content_type in fetch cache
- **websearch**: Limit response body reads in DDG news and VQD fetch
- **websearch**: Return errors from CLI commands instead of os.Exit
- **spectest**: Relax judge criteria for contact resolution tests
- **spectest**: Mention contacts in judge evaluation prompt
- **spectest**: Remove duplicate EachProvider comment block
- **websearch**: Include page number in cache key
- **websearch**: Add Migrate call to history command
- **websearch**: Reuse httpclient across searches
- **websearch**: Add bing to --backend flag help text
- **websearch**: Log warnings on cache/history write failures
- **websearch**: Reject queries longer than 2000 chars
- **websearch**: Fix flaky health cooldown test on Windows
- **tools**: Fix Gemini prompt passing and Claude stream-json args
- **telegram**: Reuse TaskTracker across messages in session
- **tools**: Pass RunOption to StreamRunner for max_budget_usd
- **tools**: Fix data race in mockInteractor used by GWS test
- **tools**: Remove hardcoded OS-specific paths from tests
- **tools**: Parse Gemini and Codex streaming JSON formats
- **tools**: Drop non-text protocol events in stream parser
- **scheduler**: Disable one-shot on enqueue instead of marking completed
- **config**: Add nil guard in SchedulerDataDSN
- **jobs**: Return errors on pusher failure instead of silently succeeding
- **scheduler**: Return errors from parseTime and json.Unmarshal in scanSchedule
- **scheduler**: Parameterize enabled column for Postgres compatibility
- **scheduler**: Use parent context in cron callbacks instead of Background
- **scheduler**: Return error when deleting non-existent schedule
- **audit**: Add Close method to Logger and wire defer-close at all call sites
- **tools**: Cap approval history to prevent unbounded growth
- **tools**: Harden bash filter against bypass vectors
- **audit**: Use cross-platform invalid path in TestOpenDefault_BadPath
- **spec**: Adjust web test judge criteria for reliability
- **spec**: Make judge prompt tool-agnostic
- **telegram**: Init websearch once to prevent DB and provider leak
- **cli**: Close websearch DB handle on exit
- **tools**: Guard NewWebSearchInstance against nil config
- **whatsapp**: Save history sync messages during QR auth login
- **skills**: Whatsapp-read uses contact join to find messages by person name
- **skills**: Format whatsapp timestamps as human-readable dates
- **telegram**: Instruct agent to format dates in human-friendly format
- **whatsapp**: Populate sender_name from conversation display name during history sync
- **gmail**: Normalize email dates to UTC before storing
- **skills**: Use relative path for gws generate-skills output dir
- **oauth**: Convert gws service names to full Google API scope URLs
- **service**: Include user PATH in launchd and systemd services
- **gws**: Strip duplicate gws prefix from commands
- **server**: Wire GWS callback URL to Google OAuth config
- **server**: Signal scope waiter on OAuth exchange failure
- **cli**: Auto-detect ngrok domain and fix tunnel port to 8443
- **cli**: Remove duplicate credentials prompt from ngrok setup
- **telegram**: Add warn logging to restoreSession error paths
- **agent**: Guard against empty summary in compactWithSummary
- **telegram**: Address review issues in processingFeedback
- **cli**: Skip file permission check on Windows in ngrok config test
- **telegram**: Close audit logger and recorder in newAgent tests
- **gws**: Add re-auth retry when token is expired
- **oauth**: Force prompt=consent to get refresh token
- **gemini**: Add 2-minute HTTP timeout to prevent hanging requests
- **oauth**: Auto-discover email in ExchangeCode for first-time auth
- **server**: Allow first-time OAuth callback with empty account
- **tools**: Resolve account dynamically after first-time OAuth
- **prompt**: Instruct agent to load gws skills before gws_execute
- **gws**: Expand dot syntax in commands (files.list → files list)
- **skills**: Auto-include gws-shared when loading any gws skill
- **prompt**: Add --params syntax reminder to GWS instructions
- **gws**: Trigger re-auth when token refresh fails
- **gws**: Restore prefix stripping and add token-expired re-auth
- **gws**: Use shlex to parse commands with quoted --params JSON
- **gws**: Use brace-matching splitter for --params JSON arguments
- **gws**: Update system prompt to teach structured params/body usage
- **agent**: Restore microcompaction lost during merge
- **telegram**: Retry send as plain text on Markdown parse failure
- **telegram**: Convert Markdown to Telegram HTML via telegold
- **telegram**: Add Info-level logging to decideAck for feedback observability
- **oauth**: Prevent DB connection leak in dbTokenSource
- **tools**: Add truncation to file_read output
- **tools**: Add 2-minute timeout to gws CLI execution
- **tools**: Add byte cap to load_skills output
- **tools**: Add truncation to subagent and delegate_task output
- **jobs**: Wire scratch dir for scheduled task agent
- **tools**: Wire scratch dir to subagent child registries
- **telegram**: Disable thinking for ack decision to reduce latency
- **agent**: Set default context window and compaction threshold
- **server**: Reject OAuth callback when scopes are missing
- **telegram**: Re-send typing indicator after text ack
- **telegram**: Update test to expect HTML parse mode
- **server**: Register waiter in empty-account auth test
- **server**: Use 'server run' subcommand in Docker test
- **docker**: Use 'server run' as default CMD
- **tools**: Write full output to fallback file, not truncated version
- **telegram**: Only retry send as plain text on 400 Bad Request
- **tools**: Sanitize tool call ID in file fallback path
- Log EnsureScratchDir errors instead of silently ignoring
- **config**: Skip Unix perm check on Windows in scratch dir test
- **tools**: Use file-as-parent trick for cross-platform unwritable path
- **tools**: Reduce gws_execute timeout from 2min to 30s
- **gemini**: Reduce HTTP timeout from 2min to 60s
- **anthropic**: Add 60s HTTP timeout to prevent hanging requests
- **openai**: Add 60s HTTP timeout to prevent hanging requests
- **oauth**: Resolve account from store and always merge scopes in GrantScopes
- **skills**: Always install built-in skills regardless of auth state
- **prompt**: Direct email operations to bash skills, not gws_execute
- **gws**: Add gmail to serviceToScope and fallback scope lookup
- **agent**: Include tool output alongside error in tool results
- **prompt**: Add gws retry guidance for failed commands
- **cli**: Fall through to interactive TUI when --scopes not given
- **cli**: Don't remove unmanaged scopes in interactive TUI
- **oauth**: Auto-open browser for OAuth URL instead of printing it
- **cli**: Improve multi-select UX with height and hint text
- **cli**: Remove redundant gmail.compose scope from picker
- **gmail**: Remove migration code, not needed pre-launch
- **gws**: Use manifest Write field to determine write commands
- **gws**: Trigger re-auth when Google returns insufficient scopes
- **gws**: Detect SERVICE_DISABLED and send API enablement link to user
- **gws**: Detect SERVICE_DISABLED and annotate for agentic URL extraction
- **prompt**: Inject current date into system prompt dynamic block
- **prompt**: Include current time alongside date in system prompt
- **telegram**: Make agent responses more casual and human-like
- **config**: Use Gemini 2.0 Flash-Lite from Google AI Studio for nano
- **config**: Use Gemini direct for fast tier in starter/standard
- **telegram**: Initialize nanoProvider in session tests
- **telegram**: Skip memory extraction when no models configured
- **daemon**: Isolate tests from real config dir via OBK_CONFIG_DIR
- **provider**: Resolve context window for nested model IDs and add mistral
- **cli**: Add nil guard to setupWithCustomProfile
- **cli**: Replace interactive test that hangs on Windows CI
- **oauth**: Always set include_granted_scopes in GrantScopes
- **telegram**: Allow detailed responses when user asks for specifics
- **oauth**: Validate HTTPS before opening URL in browser
- **oauth**: Improve browser auth UX messaging
- **test**: Update expandScopes test for drive.readonly now being a known alias
- **agent**: Add permission flags to streaming runner too
- **telegram**: Handle empty LLM responses gracefully
- **history**: Exclude ended sessions from restore
- **telegram**: Mark session ended in DB on /start reset
- **prompt**: Update delegation guidance for file-based results
- **telegram**: Answer callback query and remove approval buttons
- **telegram**: Make Signal block until ack is sent
- **delegate**: Rewrite tool description to emphasize capability over friction
- **prompt**: Remove GWS anchoring lines and simplify delegation section
- **config**: Add SyncDays field to GmailConfig
- **setup**: Persist Gmail sync window selection to config
- **gmail**: Use config SyncDays as --days default when flag not set
- **setup**: Change Gmail access level to single-select
- **setup**: Add WhatsApp config handler in source loop
- **doctor**: Add missing database checks
- **status**: Add contacts database handling in status command
- **config**: Add PasswordRef and ResolvedPassword to RemoteConfig
- **setup**: Store remote password in keychain
- **cli**: Resolve remote password from keychain at all call sites
- **setup**: Add skip option when ngrok is not installed
- **setup**: Add skip option when gws CLI is not installed
- **cli**: Update all auth command string references to resource-first paths
- **docs**: Bind safety layer text to containers to prevent overflow
- **docs**: Widen sources and SQLite sections to match safety layer
- **docs**: Restore original diagram dimensions, fix text with smaller fonts
- **docs**: Bind all text to containers to prevent overflow
- **config**: Return error from ResolvedPassword instead of silent fallback
- **status**: Add contacts to status output
- **setup**: Validate timezone input with time.LoadLocation
- **docker**: Update CMD to use unified service command
- **launchd**: Use named wrapper scripts so Login Items shows distinct names
- **make**: Update Makefile to use unified service restart
- **launchd**: Rename wrappers to openbotkit-daemon and openbotkit-server
- **setup**: Link applecontacts instead of contacts in Apple Contacts setup
- **contacts**: Remove erroneous LinkSource("contacts") from sync command
- **daemon**: Add migration from contacts to applecontacts linking
- **spectest**: Skip spec tests unless OBK_TEST_PROVIDER is set
- **obkmacos**: Swap createdAt/modifiedAt in notes output
- **contacts**: Use FetchContacts for permission check to trigger TCC dialog
- **release**: Build obkmacos for both arm64 and amd64, upload as release assets
- **obkmacos**: Check authorization status before requesting contacts access
- **install**: Trigger xcode-select --install when swiftc missing
- **cli**: Guard setup applecontacts/applenotes on non-darwin
- **cli**: Guard applenotes sync on non-darwin
- **test**: Skip migration tests on non-darwin platforms
- **website**: Refine safety section — capture sentiment, not transcript
- **website**: Simplify terminal component — just dots and commands
- **website**: Soften terminal styling — lighter bg, smaller font, muted text
- **website**: Wrap terminal commands, move copy icon inline per command
- **website**: Right-align copy icons at end of each terminal line
- **website**: Replace em dashes with hyphens across page content
- **website**: Correct Codex CLI link in footer
- **website**: Credit OpenBotKit as a 73ai project in footer
- **website**: Position copy icons absolutely at end of each line
- **website**: Link MIT License to project's LICENSE file
- **website**: Correct PJ's Twitter handle to @pjay_in
- **website**: Use pjay not PJ in demo caption
- **website**: Update demo caption wording
- **website**: Rework hyphen punctuation with commas, colons, and parens
- **website**: Rename support section to reflect research focus
- **oauth**: Eliminate race in ScopeWaiter between NotifyLink and Signal
- **status**: Report error when history DB fails to open
- **tools**: Reject whitespace-only bash commands
- **tools**: Use 0600 permissions for sandbox temp code files
- **tools**: Use production timeout for agent integration tests
- **models**: Replace deprecated gemini-2.0-flash-lite with gemini-2.0-flash
- **tui**: Use heap-allocated pointers for huh form values
- **server**: Detect Telegram WebView via TelegramWebviewProxy
- **tools**: Make OAuth consent non-blocking
- **server**: Use Telegram.WebApp.openLink() in trampoline page
- **telegram**: Use web_app button for auth redirect links
- **prompt**: Guide agent to use service-specific APIs instead of Drive for content creation
- **tools**: Add nil-provider guard to extract, improve test coverage
- **prompt**: Clarify learnings are knowledge, not user profile data
- **tools**: Slim notification to topic+link, remove from main conversation
- **telegram**: Only pass notifier to extract tool, not main save tool
- **tools**: Use agent's own summary as extract notification, not hardcoded text
- **tools**: Add guardrails to extract sub-agent prompt against saving non-learnings
- **prompt**: Tell agent not to force extraction when session has no real learnings
- **tools**: Only link newly created topics in extract notification
- **tools**: Track saved topics during extraction instead of diffing all topics
- **server**: Initialize learnings Store once on Server struct
- **tools**: Add 2-minute timeout to extract goroutine
- **telegram**: Add notifier to direct save tool so saves push notifications
- **learnings**: Filter search to bullet lines, add path traversal guard, git user config, and human-readable list
- **jobs**: Rename NextRetryAt to NextRetry to match River interface
- **tools**: Scrub env for all external agent runners
- **scheduler**: Replace trigger query denylist with allowlist to prevent SQL injection
- **daemon**: Notify reactive triggers periodically during WhatsApp sync
- **daemon**: Format reactive trigger rows as key-value pairs
- **jobs**: Remove dead code in NextRetry for cancelled error types
- **tools**: Expand env scrub to catch DATABASE_URL, DSN, PRIVATE_KEY
- **cerebras**: Use real model names from Cerebras API
- **history**: Sanitize sessionID to prevent path traversal
- **memory**: Persist source field in bullet format
- **history**: Stop appending index on every SaveMessage
- **history**: Return last N messages instead of first N
- Log warnings for malformed JSONL lines instead of silent skip
- **history**: Replace O(n²) bubble sort with sort.Slice
- **zai**: Handle reasoning_content fallback
- **zai**: Report scanner errors in SSE stream parser
- **docs**: Correct deprecation note, model names, and setup description
- **backup**: Fix VacuumInto path collision and make Run() testable
- **settings**: Validate backup config before enabling
- **settings**: Enforce backup settings flow — lock enabled until destination authenticated
- **settings**: GDrive backup wizard runs OAuth + creates folder automatically
- **settings**: GDrive backup skips folder name prompt, goes straight to OAuth
- **settings**: Only launch backup wizard when not configured, normal toggle otherwise
- **settings**: Remove Google Drive Folder ID from backup settings display
- **backup**: Escape single quotes in VACUUM INTO dest path
- **backup**: Add confirmation prompt before restore overwrites files
- **backup**: Escape single quotes in GDrive API query strings
- **backup**: Add random suffix to manifest ID to prevent collisions
- **backup**: Stream file through zstd instead of reading entirely into memory
- **backup**: Skip symlinks in scanner to prevent infinite loops
- **backup**: Use sentinel error instead of string matching in GDrive backend
- **backup**: Format snapshot dates in obk backup list output
- **backup**: Show time-ago and local time in status, drop hostname
- **website**: Support versioned changelog entries from release-please
- **website**: Update LLM providers list to include Cerebras and ZAI
- **website**: Update provider list in cost-conscious belief
- **website**: Expand web search description with engine list
- **website**: Update Slack integration description
- **website**: List Google Workspace services explicitly
- **website**: Update skills count and highlight personas
- **website**: Update capabilities page meta description

## 2026-03-22

### Added

- **Backup system**: full backup and restore for all SQLite databases, config, and learnings.
- **Cloud backends**: Cloudflare R2, any S3-compatible storage, and Google Drive.
- **Backup wizard**: interactive setup in the settings TUI walks through credentials and verification.
- **Backup daemon**: automatic scheduled backups run in the background.
- **CLI commands**: `obk backup now`, `obk backup list`, `obk backup status`, `obk backup restore`.

## 2026-03-20

### Added

- **Cerebras provider**: new LLM provider with fast inference.
- **Z.AI provider**: GLM models via Z.AI API.
- **Free tier**: Gemini + Cerebras profile with zero API costs to get started.
- **Reactive scheduler**: triggers that fire in response to data changes, not just cron schedules.
- **Budget tracking**: track and limit LLM spending per session.

### Changed

- **History format**: migrated conversation history from SQLite to JSONL files for portability.
- **Memory format**: migrated user memory from SQLite to Markdown files.

## 2026-03-19

### Added

- **Settings TUI**: interactive terminal UI for configuring providers, profiles, sources, and backup.
- **Learnings system**: assistant extracts and remembers facts about you across conversations, stored as local Markdown files, searchable via CLI and tools.
- **Model cache**: providers now cache available models locally for faster startup.

### Fixed

- **Telegram WebView auth**: OAuth consent links now open correctly inside Telegram's in-app browser.

## 2026-03-18

### Added

- **Bash sandboxing**: bash commands run inside a sandbox (Seatbelt on macOS, bwrap on Linux).
- **File approval gates**: `file_write` and `file_edit` require explicit user approval.
- **sandbox_exec tool**: sandboxed code execution for untrusted scripts.

### Changed

- **Source and service refactor**: reorganized source packages and unified service commands into `obk service`.

## 2026-03-17

### Added

- **Website launch**: Astro + Tailwind site with capabilities and mission pages at openbotkit.dev.
- **macOS native integration**: Swift helper binary (`obkmacos`) for Apple Notes, Contacts, and iMessage.
- **Installer script**: `curl -fsSL .../install.sh | sh` for one-command setup.

## 2026-03-15

### Added

- **Google Workspace integration**: 15 services (Calendar, Gmail, Docs, Sheets, Slides, Drive, Meet, Chat, Tasks, Keep, Forms, Classroom, People, Admin Reports, Workflows) with progressive consent.
- **LLM profiles**: starter, standard, and premium tiers with cost guidance.
- **Groq provider**: fast open-source model inference.
- **OpenRouter provider**: access to 100+ models through a single API key.

### Changed

- **Delegation improvements**: file-based delegation results, improved prompt guidance, and full-permission flags for delegate tasks.

## 2026-03-14

### Added

- **Tool output management**: structured tool output handling with size limits and truncation.
- **Nano tier**: ultra-cheap LLM tier for feedback extraction and memory reconciliation.

### Fixed

- **Provider timeouts**: added 60-second HTTP timeouts to OpenAI, Anthropic, and Gemini providers to prevent hanging requests.

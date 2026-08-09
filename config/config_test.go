package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIncludesWhatsApp(t *testing.T) {
	cfg := Default()
	if cfg.WhatsApp == nil {
		t.Fatal("expected WhatsApp config in defaults")
	}
	if cfg.WhatsApp.Storage.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", cfg.WhatsApp.Storage.Driver)
	}
}

func TestApplyDefaultsWhatsApp(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.WhatsApp == nil {
		t.Fatal("applyDefaults should create WhatsApp config")
	}
	if cfg.WhatsApp.Storage.Driver != "sqlite" {
		t.Fatalf("expected sqlite, got %q", cfg.WhatsApp.Storage.Driver)
	}
}

func TestWhatsAppDataDSNDefault(t *testing.T) {
	cfg := Default()
	dsn := cfg.WhatsAppDataDSN()
	if !strings.HasSuffix(dsn, filepath.Join("whatsapp", "data.db")) {
		t.Fatalf("expected path ending in whatsapp/data.db, got %q", dsn)
	}
}

func TestWhatsAppDataDSNCustom(t *testing.T) {
	cfg := Default()
	cfg.WhatsApp.Storage.DSN = "postgres://localhost/wa"
	dsn := cfg.WhatsAppDataDSN()
	if dsn != "postgres://localhost/wa" {
		t.Fatalf("expected custom DSN, got %q", dsn)
	}
}

func TestWhatsAppSessionDBPath(t *testing.T) {
	cfg := Default()
	path := cfg.WhatsAppSessionDBPath()
	if !strings.HasSuffix(path, filepath.Join("whatsapp", "session.db")) {
		t.Fatalf("expected path ending in whatsapp/session.db, got %q", path)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
whatsapp:
  storage:
    driver: postgres
    dsn: postgres://localhost/watest
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.WhatsApp.Storage.Driver != "postgres" {
		t.Fatalf("expected postgres, got %q", cfg.WhatsApp.Storage.Driver)
	}
	if cfg.WhatsApp.Storage.DSN != "postgres://localhost/watest" {
		t.Fatalf("expected custom dsn, got %q", cfg.WhatsApp.Storage.DSN)
	}
}

func TestLoadFromMissingFile(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("missing file should return defaults, got error: %v", err)
	}
	if cfg.WhatsApp == nil {
		t.Fatal("expected defaults to include WhatsApp")
	}
}

func TestRequireSetup_NoModels(t *testing.T) {
	cfg := Default()
	if err := cfg.RequireSetup(); err == nil {
		t.Fatal("expected error when models not configured")
	}
}

func TestRequireSetup_NoDefault(t *testing.T) {
	cfg := Default()
	cfg.Models = &ModelsConfig{}
	if err := cfg.RequireSetup(); err == nil {
		t.Fatal("expected error when default model not set")
	}
}

func TestRequireSetup_OK(t *testing.T) {
	cfg := Default()
	cfg.Models = &ModelsConfig{Default: "anthropic/claude-sonnet-4-6"}
	if err := cfg.RequireSetup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvedMode_DefaultsToLocal(t *testing.T) {
	cfg := Default()
	if cfg.ResolvedMode() != ModeLocal {
		t.Fatalf("expected local, got %q", cfg.ResolvedMode())
	}
}

func TestResolvedMode_ExplicitValues(t *testing.T) {
	tests := []struct {
		mode Mode
		want Mode
	}{
		{ModeLocal, ModeLocal},
		{ModeRemote, ModeRemote},
		{ModeServer, ModeServer},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			cfg := Default()
			cfg.Mode = tt.mode
			if cfg.ResolvedMode() != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, cfg.ResolvedMode())
			}
		})
	}
}

func TestIsLocal_IsRemote_IsServer(t *testing.T) {
	cfg := Default()
	if !cfg.IsLocal() {
		t.Fatal("default should be local")
	}
	if cfg.IsRemote() {
		t.Fatal("default should not be remote")
	}
	if cfg.IsServer() {
		t.Fatal("default should not be server")
	}

	cfg.Mode = ModeRemote
	if cfg.IsLocal() {
		t.Fatal("remote should not be local")
	}
	if !cfg.IsRemote() {
		t.Fatal("remote should be remote")
	}

	cfg.Mode = ModeServer
	if !cfg.IsServer() {
		t.Fatal("server should be server")
	}
}

func TestSchedulerDataDSNDefault(t *testing.T) {
	cfg := Default()
	dsn := cfg.SchedulerDataDSN()
	if !strings.HasSuffix(dsn, filepath.Join("scheduler", "data.db")) {
		t.Fatalf("expected path ending in scheduler/data.db, got %q", dsn)
	}
}

func TestSchedulerDataDSNCustom(t *testing.T) {
	cfg := Default()
	cfg.Scheduler.Storage.DSN = "postgres://localhost/sched"
	dsn := cfg.SchedulerDataDSN()
	if dsn != "postgres://localhost/sched" {
		t.Fatalf("expected custom DSN, got %q", dsn)
	}
}

func TestApplyDefaultsScheduler(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	if cfg.Scheduler == nil {
		t.Fatal("applyDefaults should create Scheduler config")
	}
	if cfg.Scheduler.Storage.Driver != "sqlite" {
		t.Fatalf("expected sqlite, got %q", cfg.Scheduler.Storage.Driver)
	}
}

func TestSourceDataDSN_ValidSources(t *testing.T) {
	cfg := Default()
	sources := []struct {
		name   string
		suffix string
	}{
		{"gmail", filepath.Join("gmail", "data.db")},
		{"whatsapp", filepath.Join("whatsapp", "data.db")},
		{"history", filepath.Join("history", "data.db")},
		{"user_memory", filepath.Join("user_memory", "data.db")},
		{"applenotes", filepath.Join("applenotes", "data.db")},
		{"imessage", filepath.Join("imessage", "data.db")},
		{"websearch", filepath.Join("websearch", "data.db")},
		{"scheduler", filepath.Join("scheduler", "data.db")},
		{"telegram", filepath.Join("telegram", "data.db")},
	}
	for _, s := range sources {
		t.Run(s.name, func(t *testing.T) {
			dsn, err := cfg.SourceDataDSN(s.name)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(dsn, s.suffix) {
				t.Fatalf("expected path ending in %s, got %q", s.suffix, dsn)
			}
		})
	}
}

func TestSourceDataDSN_UnknownSource(t *testing.T) {
	cfg := Default()
	_, err := cfg.SourceDataDSN("unknown")
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestTelegramSourceDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.Telegram == nil {
		t.Fatal("applyDefaults should create Telegram source config")
	}
	if cfg.Telegram.Storage.Driver != "sqlite" {
		t.Fatalf("expected sqlite, got %q", cfg.Telegram.Storage.Driver)
	}
	if got := cfg.TelegramBackfillDays(); got != 90 {
		t.Fatalf("expected 90 day default backfill, got %d", got)
	}

	if !strings.HasSuffix(cfg.TelegramSessionPath(), filepath.Join("telegram", "session.json")) {
		t.Fatalf("unexpected session path %q", cfg.TelegramSessionPath())
	}
}

// The source config must not collide with the bot channel's telegram config.
func TestTelegramSourceAndChannelCoexist(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Channels = &ChannelsConfig{Telegram: &TelegramConfig{BotToken: "bot-token", OwnerID: 42}}
	cfg.Telegram.BackfillDays = 30
	cfg.Telegram.APIIDRef = "keychain:obk/telegram/api_id"

	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Channels == nil || loaded.Channels.Telegram == nil {
		t.Fatal("channel telegram config lost")
	}
	if loaded.Channels.Telegram.BotToken != "bot-token" {
		t.Fatalf("bot token = %q", loaded.Channels.Telegram.BotToken)
	}
	if loaded.Telegram == nil {
		t.Fatal("telegram source config lost")
	}
	if loaded.Telegram.BackfillDays != 30 {
		t.Fatalf("backfill days = %d", loaded.Telegram.BackfillDays)
	}
	if loaded.Telegram.APIIDRef != "keychain:obk/telegram/api_id" {
		t.Fatalf("api id ref = %q", loaded.Telegram.APIIDRef)
	}
}

func TestConfigRoundTrip_WithModeAndRemote(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Mode = ModeRemote
	cfg.Remote = &RemoteConfig{
		Server:   "https://example.com:8443",
		Username: "testuser",
		Password: "testpass",
	}
	cfg.Auth = &AuthConfig{
		Username: "admin",
		Password: "secret",
	}
	cfg.Channels = &ChannelsConfig{
		Telegram: &TelegramConfig{
			BotToken: "123:abc",
			OwnerID:  42,
		},
	}

	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Mode != ModeRemote {
		t.Fatalf("expected remote mode, got %q", loaded.Mode)
	}
	if loaded.Remote == nil || loaded.Remote.Server != "https://example.com:8443" {
		t.Fatal("remote config not preserved")
	}
	if loaded.Auth == nil || loaded.Auth.Username != "admin" {
		t.Fatal("auth config not preserved")
	}
	if loaded.Channels == nil || loaded.Channels.Telegram == nil || loaded.Channels.Telegram.OwnerID != 42 {
		t.Fatal("channels config not preserved")
	}
}

func TestBackwardCompat_NoModeField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
gmail:
  storage:
    driver: sqlite
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.IsLocal() {
		t.Fatalf("old config without mode should default to local, got %q", cfg.ResolvedMode())
	}
}

func TestWebSearchDataDSN(t *testing.T) {
	cfg := Default()
	dsn := cfg.WebSearchDataDSN()
	if !strings.HasSuffix(dsn, filepath.Join("websearch", "data.db")) {
		t.Fatalf("expected path ending in websearch/data.db, got %q", dsn)
	}
}

func TestWebSearchDataDSNCustom(t *testing.T) {
	cfg := Default()
	cfg.WebSearch.Storage.DSN = "postgres://localhost/ws"
	dsn := cfg.WebSearchDataDSN()
	if dsn != "postgres://localhost/ws" {
		t.Fatalf("expected custom DSN, got %q", dsn)
	}
}

func TestApplyDefaultsWebSearch(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.WebSearch == nil {
		t.Fatal("applyDefaults should create WebSearch config")
	}
	if cfg.WebSearch.Storage.Driver != "sqlite" {
		t.Fatalf("expected sqlite, got %q", cfg.WebSearch.Storage.Driver)
	}
	if cfg.WebSearch.Timeout != "15s" {
		t.Fatalf("expected 15s timeout, got %q", cfg.WebSearch.Timeout)
	}
}

func TestGWSCallbackURL_Nil(t *testing.T) {
	cfg := Default()
	if url := cfg.GWSCallbackURL(); url != "" {
		t.Fatalf("expected empty, got %q", url)
	}
}

func TestGWSCallbackURL_IntegrationsSetGWSNil(t *testing.T) {
	cfg := Default()
	cfg.Integrations = &IntegrationsConfig{}
	if url := cfg.GWSCallbackURL(); url != "" {
		t.Fatalf("expected empty when GWS is nil, got %q", url)
	}
}

func TestGWSCallbackURL_Set(t *testing.T) {
	cfg := Default()
	cfg.Integrations = &IntegrationsConfig{
		GWS: &GWSConfig{
			CallbackURL: "https://example.ngrok-free.app/auth/google/callback",
		},
	}
	want := "https://example.ngrok-free.app/auth/google/callback"
	if got := cfg.GWSCallbackURL(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGWSConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Integrations = &IntegrationsConfig{
		GWS: &GWSConfig{
			Enabled:     true,
			Services:    []string{"drive", "docs"},
			CallbackURL: "https://example.ngrok-free.app/auth/google/callback",
			NgrokDomain: "example.ngrok-free.app",
		},
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Integrations == nil || loaded.Integrations.GWS == nil {
		t.Fatal("GWS config not preserved")
	}
	if loaded.Integrations.GWS.CallbackURL != cfg.Integrations.GWS.CallbackURL {
		t.Fatalf("callback URL not preserved: %q", loaded.Integrations.GWS.CallbackURL)
	}
	if loaded.Integrations.GWS.NgrokDomain != cfg.Integrations.GWS.NgrokDomain {
		t.Fatalf("ngrok domain not preserved: %q", loaded.Integrations.GWS.NgrokDomain)
	}
}

func TestModelsConfig_CompactionFields_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Models = &ModelsConfig{
		Default:             "gemini/gemini-2.5-flash",
		ContextWindow:       150000,
		CompactionThreshold: 0.25,
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Models.ContextWindow != 150000 {
		t.Fatalf("ContextWindow = %d, want 150000", loaded.Models.ContextWindow)
	}
	if loaded.Models.CompactionThreshold != 0.25 {
		t.Fatalf("CompactionThreshold = %f, want 0.25", loaded.Models.CompactionThreshold)
	}
}

func TestCustomProfile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Models = &ModelsConfig{
		Default: "anthropic/claude-haiku-4-5",
		Profile: "my-setup",
		CustomProfiles: map[string]CustomProfile{
			"my-setup": {
				Label:       "My Budget Setup",
				Description: "Mixed providers for cost savings.",
				Tiers: ProfileTiers{
					Default: "anthropic/claude-haiku-4-5",
					Complex: "gemini/gemini-2.5-pro",
					Fast:    "gemini/gemini-2.5-flash-lite",
					Nano:    "gemini/gemini-2.5-flash-lite",
				},
				Providers: []string{"anthropic", "gemini"},
			},
		},
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Models.Profile != "my-setup" {
		t.Fatalf("Profile = %q, want my-setup", loaded.Models.Profile)
	}
	cp, ok := loaded.Models.CustomProfiles["my-setup"]
	if !ok {
		t.Fatal("custom profile my-setup not found after reload")
	}
	if cp.Label != "My Budget Setup" {
		t.Fatalf("Label = %q, want My Budget Setup", cp.Label)
	}
	if cp.Tiers.Default != "anthropic/claude-haiku-4-5" {
		t.Fatalf("Tiers.Default = %q, want anthropic/claude-haiku-4-5", cp.Tiers.Default)
	}
	if len(cp.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(cp.Providers))
	}
}

func TestCustomProfile_BackwardCompat_NoField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
models:
  default: anthropic/claude-sonnet-4-6
  profile: standard
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Models.CustomProfiles != nil {
		t.Fatal("expected nil CustomProfiles for old config")
	}
}

func TestCustomProfile_MultipleProfiles_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Models = &ModelsConfig{
		Default: "anthropic/claude-haiku-4-5",
		CustomProfiles: map[string]CustomProfile{
			"budget": {
				Label: "Budget",
				Tiers: ProfileTiers{
					Default: "gemini/gemini-2.5-flash",
					Complex: "gemini/gemini-2.5-pro",
					Fast:    "gemini/gemini-2.5-flash-lite",
					Nano:    "gemini/gemini-2.5-flash-lite",
				},
				Providers: []string{"gemini"},
			},
			"premium-mix": {
				Label: "Premium Mix",
				Tiers: ProfileTiers{
					Default: "anthropic/claude-sonnet-4-6",
					Complex: "anthropic/claude-opus-4-6",
					Fast:    "gemini/gemini-2.5-flash-lite",
					Nano:    "gemini/gemini-2.5-flash-lite",
				},
				Providers: []string{"anthropic", "gemini"},
			},
		},
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Models.CustomProfiles) != 2 {
		t.Fatalf("expected 2 custom profiles, got %d", len(loaded.Models.CustomProfiles))
	}
	if loaded.Models.CustomProfiles["budget"].Label != "Budget" {
		t.Fatalf("budget label = %q", loaded.Models.CustomProfiles["budget"].Label)
	}
	if loaded.Models.CustomProfiles["premium-mix"].Label != "Premium Mix" {
		t.Fatalf("premium-mix label = %q", loaded.Models.CustomProfiles["premium-mix"].Label)
	}
}

func TestCustomProfile_EmptyLabelAndDescription(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Models = &ModelsConfig{
		Default: "gemini/gemini-2.5-flash",
		CustomProfiles: map[string]CustomProfile{
			"minimal": {
				Tiers: ProfileTiers{
					Default: "gemini/gemini-2.5-flash",
					Complex: "gemini/gemini-2.5-pro",
					Fast:    "gemini/gemini-2.5-flash-lite",
					Nano:    "gemini/gemini-2.5-flash-lite",
				},
				Providers: []string{"gemini"},
			},
		},
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cp := loaded.Models.CustomProfiles["minimal"]
	if cp.Label != "" {
		t.Fatalf("expected empty label, got %q", cp.Label)
	}
	if cp.Description != "" {
		t.Fatalf("expected empty description, got %q", cp.Description)
	}
	if cp.Tiers.Default != "gemini/gemini-2.5-flash" {
		t.Fatal("tiers not preserved with empty label/description")
	}
}

func TestResolvedPassword_FromRef(t *testing.T) {
	r := &RemoteConfig{
		Password:    "plain",
		PasswordRef: "keychain:obk/remote",
	}
	resolver := func(ref string) (string, error) {
		if ref == "keychain:obk/remote" {
			return "secret-from-keychain", nil
		}
		return "", fmt.Errorf("not found")
	}
	got, err := r.ResolvedPassword(resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "secret-from-keychain" {
		t.Fatalf("expected keychain password, got %q", got)
	}
}

func TestResolvedPassword_RefFailsReturnsError(t *testing.T) {
	r := &RemoteConfig{
		Password:    "plain",
		PasswordRef: "keychain:obk/missing",
	}
	resolver := func(ref string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	_, err := r.ResolvedPassword(resolver)
	if err == nil {
		t.Fatal("expected error when resolver fails, got nil")
	}
}

func TestResolvedPassword_NoRef(t *testing.T) {
	r := &RemoteConfig{
		Password: "plain",
	}
	got, err := r.ResolvedPassword(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "plain" {
		t.Fatalf("expected plain password, got %q", got)
	}
}

func TestResolvedPassword_NilResolverWithRef(t *testing.T) {
	r := &RemoteConfig{
		Password:    "plain",
		PasswordRef: "keychain:obk/remote",
	}
	_, err := r.ResolvedPassword(nil)
	if err == nil {
		t.Fatal("expected error when resolver is nil but password_ref is set")
	}
}

func TestPasswordRef_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Mode = ModeRemote
	cfg.Remote = &RemoteConfig{
		Server:      "https://example.com:8443",
		Username:    "user",
		PasswordRef: "keychain:obk/remote",
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Remote.PasswordRef != "keychain:obk/remote" {
		t.Fatalf("PasswordRef not preserved: %q", loaded.Remote.PasswordRef)
	}
	if loaded.Remote.Password != "" {
		t.Fatalf("expected empty Password, got %q", loaded.Remote.Password)
	}
}

func TestGmailSyncDays_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Gmail.SyncDays = 30
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Gmail.SyncDays != 30 {
		t.Fatalf("expected SyncDays=30, got %d", loaded.Gmail.SyncDays)
	}
}

func TestWhatsAppAccountList_LegacyMode(t *testing.T) {
	cfg := Default()
	entries := cfg.WhatsAppAccountList()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Label != "default" {
		t.Fatalf("expected label 'default', got %q", entries[0].Label)
	}
	if entries[0].Role != "source" {
		t.Fatalf("expected role 'source', got %q", entries[0].Role)
	}
}

func TestWhatsAppAccountList_MultiAccount(t *testing.T) {
	cfg := Default()
	cfg.WhatsApp.Accounts = map[string]*WhatsAppAccount{
		"personal":  {Role: "source"},
		"assistant": {Role: "channel", OwnerJID: "123@s.whatsapp.net"},
	}
	entries := cfg.WhatsAppAccountList()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	found := map[string]WhatsAppAccountEntry{}
	for _, e := range entries {
		found[e.Label] = e
	}
	if found["personal"].Role != "source" {
		t.Fatalf("personal role = %q, want source", found["personal"].Role)
	}
	if found["assistant"].Role != "channel" {
		t.Fatalf("assistant role = %q, want channel", found["assistant"].Role)
	}
	if found["assistant"].OwnerJID != "123@s.whatsapp.net" {
		t.Fatalf("assistant owner_jid = %q", found["assistant"].OwnerJID)
	}
}

func TestWhatsAppAccountSessionDBPath(t *testing.T) {
	cfg := Default()
	path := cfg.WhatsAppAccountSessionDBPath("myaccount")
	if !strings.HasSuffix(path, filepath.Join("whatsapp", "myaccount", "session.db")) {
		t.Fatalf("expected path ending in whatsapp/myaccount/session.db, got %q", path)
	}
}

func TestWhatsAppAccountSessionDBPath_Default(t *testing.T) {
	cfg := Default()
	path := cfg.WhatsAppAccountSessionDBPath("default")
	if path != cfg.WhatsAppSessionDBPath() {
		t.Fatalf("default label should use legacy path, got %q vs %q", path, cfg.WhatsAppSessionDBPath())
	}
}

func TestWhatsAppAccountDir(t *testing.T) {
	cfg := Default()
	dir := cfg.WhatsAppAccountDir("myaccount")
	if !strings.HasSuffix(dir, filepath.Join("whatsapp", "myaccount")) {
		t.Fatalf("expected path ending in whatsapp/myaccount, got %q", dir)
	}
}

func TestWhatsAppConfig_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.WhatsApp.Accounts = map[string]*WhatsAppAccount{
		"personal":  {Role: "source"},
		"assistant": {Role: "channel", OwnerJID: "123@s.whatsapp.net"},
	}
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.WhatsApp.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(loaded.WhatsApp.Accounts))
	}
	if loaded.WhatsApp.Accounts["assistant"].OwnerJID != "123@s.whatsapp.net" {
		t.Fatalf("owner_jid not preserved: %q", loaded.WhatsApp.Accounts["assistant"].OwnerJID)
	}
	if loaded.WhatsApp.Accounts["personal"].Role != "source" {
		t.Fatalf("role not preserved: %q", loaded.WhatsApp.Accounts["personal"].Role)
	}
}

func TestDefaultWorkspaceDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", tmp)

	dir := WorkspaceDir()
	want := filepath.Join(tmp, "workspace")
	if dir != want {
		t.Fatalf("WorkspaceDir() = %q, want %q", dir, want)
	}
}

func TestResolvedWorkspaceDir_Default(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", tmp)

	cfg := Default()
	dir := cfg.ResolvedWorkspaceDir()
	if !strings.HasSuffix(dir, "workspace") {
		t.Fatalf("ResolvedWorkspaceDir() should end in 'workspace', got %q", dir)
	}
}

func TestResolvedWorkspaceDir_Custom(t *testing.T) {
	cfg := Default()
	cfg.Workspace = "/custom/path"
	dir := cfg.ResolvedWorkspaceDir()
	if dir != "/custom/path" {
		t.Fatalf("ResolvedWorkspaceDir() = %q, want /custom/path", dir)
	}
}

func TestEnsureWorkspaceDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", tmp)

	if err := EnsureWorkspaceDir(); err != nil {
		t.Fatalf("EnsureWorkspaceDir: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmp, "workspace"))
	if err != nil {
		t.Fatalf("workspace dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("workspace should be a directory")
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("expected 0700 perms, got %o", info.Mode().Perm())
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.WhatsApp.Storage.Driver = "postgres"
	cfg.WhatsApp.Storage.DSN = "postgres://test"

	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.WhatsApp.Storage.Driver != "postgres" {
		t.Fatalf("expected postgres after reload, got %q", loaded.WhatsApp.Storage.Driver)
	}
	if loaded.WhatsApp.Storage.DSN != "postgres://test" {
		t.Fatalf("expected custom DSN after reload, got %q", loaded.WhatsApp.Storage.DSN)
	}
}

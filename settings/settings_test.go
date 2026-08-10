package settings

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	"github.com/zalando/go-keyring"
)

func testService(cfg *config.Config) *Service {
	creds := make(map[string]string)
	return New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
		WithStoreCred(func(ref, value string) error {
			creds[ref] = value
			return nil
		}),
		WithLoadCred(func(ref string) (string, error) {
			v, ok := creds[ref]
			if !ok {
				return "", nil
			}
			return v, nil
		}),
	)
}

func TestGetSetMode(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "mode")
	if field == nil {
		t.Fatal("mode field not found")
	}

	got := svc.GetValue(field)
	if got != "local" {
		t.Errorf("default mode = %q, want %q", got, "local")
	}

	if err := svc.SetValue(field, "remote"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "remote" {
		t.Errorf("after set, mode = %q, want %q", got, "remote")
	}
}

func TestGetSetTimezone(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "timezone")
	if field == nil {
		t.Fatal("timezone field not found")
	}

	if err := svc.SetValue(field, "Asia/Ho_Chi_Minh"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "Asia/Ho_Chi_Minh" {
		t.Errorf("timezone = %q, want %q", got, "Asia/Ho_Chi_Minh")
	}
}

func TestTimezoneValidation(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "timezone")
	if field == nil {
		t.Fatal("timezone field not found")
	}

	if err := svc.SetValue(field, "Not/A/Timezone"); err == nil {
		t.Error("expected validation error for invalid timezone")
	}

	if err := svc.SetValue(field, ""); err != nil {
		t.Errorf("empty timezone should be valid: %v", err)
	}
}

func TestGetSetNilModels(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "models.default")
	if field == nil {
		t.Fatal("models.default field not found")
	}

	got := svc.GetValue(field)
	if got != "" {
		t.Errorf("nil models default = %q, want empty", got)
	}

	if err := svc.SetValue(field, "anthropic/claude-sonnet-4-6"); err != nil {
		t.Fatal(err)
	}
	if cfg.Models == nil {
		t.Fatal("Models should be initialized after set")
	}
	if got := svc.GetValue(field); got != "anthropic/claude-sonnet-4-6" {
		t.Errorf("default model = %q, want %q", got, "anthropic/claude-sonnet-4-6")
	}
}

func TestGetSetBool(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "gmail.download_attachments")
	if field == nil {
		t.Fatal("gmail.download_attachments field not found")
	}

	if err := svc.SetValue(field, "true"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "true" {
		t.Errorf("download_attachments = %q, want %q", got, "true")
	}
}

func TestGetSetNumber(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "gmail.sync_days")
	if field == nil {
		t.Fatal("gmail.sync_days field not found")
	}

	if err := svc.SetValue(field, "30"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "30" {
		t.Errorf("sync_days = %q, want %q", got, "30")
	}

	if err := svc.SetValue(field, "abc"); err == nil {
		t.Error("expected validation error for non-numeric value")
	}
}

func TestPasswordField(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "models.providers.anthropic.api_key")
	if field == nil {
		t.Fatal("anthropic api_key field not found")
	}

	got := svc.GetValue(field)
	if got != "not configured" {
		t.Errorf("initial api key status = %q, want %q", got, "not configured")
	}

	if err := svc.SetValue(field, "sk-test-key"); err != nil {
		t.Fatal(err)
	}
	got = svc.GetValue(field)
	if !strings.Contains(got, "sk-tes") {
		t.Errorf("after set, api key status = %q, want masked key", got)
	}
}

func TestNilChannels(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "channels.telegram.bot_token")
	if field == nil {
		t.Fatal("telegram bot_token field not found")
	}

	got := svc.GetValue(field)
	if got != "not configured" {
		t.Errorf("nil channels bot_token = %q, want %q", got, "not configured")
	}

	if err := svc.SetValue(field, "123:ABC"); err != nil {
		t.Fatal(err)
	}
	if cfg.Channels == nil || cfg.Channels.Telegram == nil {
		t.Fatal("Channels.Telegram should be initialized after set")
	}
}

func TestTreeStructure(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	tree := svc.Tree()
	if len(tree) != 8 {
		t.Fatalf("tree has %d top-level nodes, want 8", len(tree))
	}

	labels := []string{"General", "Workspace", "LLM Models", "Channels", "Data Sources", "Integrations", "Backup", "Advanced"}
	for i, n := range tree {
		if n.Category == nil {
			t.Errorf("tree[%d] is not a category", i)
			continue
		}
		if n.Category.Label != labels[i] {
			t.Errorf("tree[%d].Label = %q, want %q", i, n.Category.Label, labels[i])
		}
	}
}

func TestSaveError(t *testing.T) {
	cfg := config.Default()
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error {
			return fmt.Errorf("disk full")
		}),
	)

	field := findField(svc, "timezone")
	if field == nil {
		t.Fatal("timezone field not found")
	}

	err := svc.SetValue(field, "UTC")
	if err == nil {
		t.Fatal("expected save error")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "disk full")
	}
}

func TestCredentialNilHandlers(t *testing.T) {
	cfg := config.Default()
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
	)

	if err := svc.StoreCredential("ref", "val"); err == nil {
		t.Error("expected error from nil storeCred")
	}
	if _, err := svc.LoadCredential("ref"); err == nil {
		t.Error("expected error from nil loadCred")
	}
}

func TestProfileDisplaysLabel(t *testing.T) {
	cfg := &config.Config{
		Models: &config.ModelsConfig{
			Profile: "gemini",
		},
	}
	svc := testService(cfg)

	field := findField(svc, "models.profile")
	if field == nil {
		t.Fatal("models.profile field not found")
	}

	got := svc.GetValue(field)
	if !strings.Contains(got, "Gemini") {
		t.Errorf("profile display = %q, want to contain 'Gemini'", got)
	}
}

func TestProfileNotConfiguredDisplay(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "models.profile")
	if field == nil {
		t.Fatal("models.profile field not found")
	}

	got := svc.GetValue(field)
	if got != "(not configured)" {
		t.Errorf("nil profile display = %q, want %q", got, "(not configured)")
	}
}

func TestAuthMethodSelectRoundTrip(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "models.providers.anthropic.auth_method")
	if field == nil {
		t.Fatal("anthropic auth_method field not found")
	}

	got := svc.GetValue(field)
	if got != "api_key" {
		t.Errorf("default auth_method = %q, want %q", got, "api_key")
	}

	if err := svc.SetValue(field, "vertex_ai"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "vertex_ai" {
		t.Errorf("auth_method = %q, want %q", got, "vertex_ai")
	}
}

func TestContextWindowRoundTrip(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "models.context_window")
	if field == nil {
		t.Fatal("models.context_window field not found")
	}

	if got := svc.GetValue(field); got != "" {
		t.Errorf("default context_window = %q, want empty", got)
	}

	if err := svc.SetValue(field, "128000"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "128000" {
		t.Errorf("context_window = %q, want %q", got, "128000")
	}

	if err := svc.SetValue(field, ""); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "" {
		t.Errorf("cleared context_window = %q, want empty", got)
	}
}

func TestCompactionThresholdRoundTrip(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "models.compaction_threshold")
	if field == nil {
		t.Fatal("models.compaction_threshold field not found")
	}

	if err := svc.SetValue(field, "0.8"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "0.8" {
		t.Errorf("compaction_threshold = %q, want %q", got, "0.8")
	}
}

func TestNilIntegrationsGWS(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "integrations.gws.enabled")
	if field == nil {
		t.Fatal("gws.enabled field not found")
	}

	if got := svc.GetValue(field); got != "false" {
		t.Errorf("nil gws enabled = %q, want %q", got, "false")
	}

	if err := svc.SetValue(field, "true"); err != nil {
		t.Fatal(err)
	}
	if cfg.Integrations == nil || cfg.Integrations.GWS == nil {
		t.Fatal("Integrations.GWS should be initialized after set")
	}
	if got := svc.GetValue(field); got != "true" {
		t.Errorf("gws.enabled = %q, want %q", got, "true")
	}
}

func TestNilWebSearch(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "websearch.proxy")
	if field == nil {
		t.Fatal("websearch.proxy field not found")
	}

	if got := svc.GetValue(field); got != "" {
		t.Errorf("nil websearch proxy = %q, want empty", got)
	}

	if err := svc.SetValue(field, "socks5://localhost:1080"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "socks5://localhost:1080" {
		t.Errorf("proxy = %q, want %q", got, "socks5://localhost:1080")
	}
}

func TestNilDaemon(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "daemon.gmail_sync_period")
	if field == nil {
		t.Fatal("daemon.gmail_sync_period field not found")
	}

	if got := svc.GetValue(field); got != "" {
		t.Errorf("nil daemon sync period = %q, want empty", got)
	}

	if err := svc.SetValue(field, "30m"); err != nil {
		t.Fatal(err)
	}
	if cfg.Daemon == nil {
		t.Fatal("Daemon should be initialized after set")
	}
	if got := svc.GetValue(field); got != "30m" {
		t.Errorf("sync period = %q, want %q", got, "30m")
	}
}

func TestTelegramOwnerID(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "channels.telegram.owner_id")
	if field == nil {
		t.Fatal("telegram owner_id field not found")
	}

	if got := svc.GetValue(field); got != "" {
		t.Errorf("nil owner_id = %q, want empty", got)
	}

	if err := svc.SetValue(field, "123456789"); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetValue(field); got != "123456789" {
		t.Errorf("owner_id = %q, want %q", got, "123456789")
	}

	if err := svc.SetValue(field, "not-a-number"); err == nil {
		t.Error("expected validation error for non-numeric owner_id")
	}
}

func TestPasswordEmptySetIsNoop(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "models.providers.openai.api_key")
	if field == nil {
		t.Fatal("openai api_key field not found")
	}

	if err := svc.SetValue(field, ""); err != nil {
		t.Fatal(err)
	}

	if got := svc.GetValue(field); got != "not configured" {
		t.Errorf("after empty set, status = %q, want %q", got, "not configured")
	}
}

func TestPasswordLoadCredFailure(t *testing.T) {
	cfg := &config.Config{
		Models: &config.ModelsConfig{
			Providers: map[string]config.ModelProviderConfig{
				"openai": {APIKeyRef: "keychain:obk/openai"},
			},
		},
	}
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
		WithLoadCred(func(ref string) (string, error) {
			return "", fmt.Errorf("keychain locked")
		}),
	)

	field := findField(svc, "models.providers.openai.api_key")
	if field == nil {
		t.Fatal("openai api_key field not found")
	}

	got := svc.GetValue(field)
	if !strings.Contains(got, "configured") {
		t.Errorf("loadCred failure status = %q, want to contain %q", got, "configured")
	}
}

func TestGWSCallbackAndNgrok(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	cbField := findField(svc, "integrations.gws.callback_url")
	ngField := findField(svc, "integrations.gws.ngrok_domain")
	if cbField == nil || ngField == nil {
		t.Fatal("gws callback_url or ngrok_domain field not found")
	}

	if err := svc.SetValue(cbField, "https://example.com/callback"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetValue(ngField, "my-app.ngrok.io"); err != nil {
		t.Fatal(err)
	}

	if got := svc.GetValue(cbField); got != "https://example.com/callback" {
		t.Errorf("callback_url = %q, want %q", got, "https://example.com/callback")
	}
	if got := svc.GetValue(ngField); got != "my-app.ngrok.io" {
		t.Errorf("ngrok_domain = %q, want %q", got, "my-app.ngrok.io")
	}
}

func TestModelsTreeStructure(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	modelsNode := svc.Tree()[2]
	if modelsNode.Category == nil || modelsNode.Category.Label != "LLM Models" {
		t.Fatal("expected LLM Models category at index 2")
	}

	children := modelsNode.Category.Children
	if children[0].Field == nil || children[0].Field.Key != "models.profile" {
		t.Error("first child of Models should be Profile field")
	}

	last := children[len(children)-1]
	if last.Category == nil || last.Category.Key != "models.providers" {
		t.Error("last child of Models should be Providers category")
	}
}

func TestModelTierFieldsAreSelects(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	for _, key := range []string{"models.default", "models.complex", "models.fast", "models.nano"} {
		f := findField(svc, key)
		if f == nil {
			t.Fatalf("%s field not found", key)
		}
		if f.Type != TypeSelect {
			t.Errorf("%s type = %d, want TypeSelect (%d)", key, f.Type, TypeSelect)
		}
		if f.OptionsFunc == nil {
			t.Errorf("%s should have OptionsFunc", key)
		}
	}
}

func TestModelOptionsShowOnlyConfiguredProviders(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	// Populate model cache for anthropic.
	cache := provider.NewModelCache(config.ModelsDir())
	cache.Save("anthropic", &provider.CachedModelList{
		Provider:  "anthropic",
		FetchedAt: time.Now(),
		Models: []provider.AvailableModel{
			{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", Provider: "anthropic"},
		},
	})

	cfg := &config.Config{
		Models: &config.ModelsConfig{
			Providers: map[string]config.ModelProviderConfig{
				"anthropic": {APIKeyRef: "keychain:obk/anthropic"},
			},
		},
	}
	svc := testService(cfg)

	field := findField(svc, "models.default")
	if field == nil {
		t.Fatal("models.default not found")
	}

	opts := svc.ResolvedOptions(field)
	for _, o := range opts {
		if o.Value == "" {
			continue
		}
		if !strings.HasPrefix(o.Value, "anthropic/") {
			t.Errorf("option %q should be from anthropic, got %q", o.Label, o.Value)
		}
	}
	if len(opts) < 2 {
		t.Errorf("expected at least (none) + 1 anthropic model, got %d options", len(opts))
	}
}

func TestModelOptionsNoProvidersConfigured(t *testing.T) {
	cfg := &config.Config{}
	svc := testService(cfg)

	field := findField(svc, "models.default")
	if field == nil {
		t.Fatal("models.default not found")
	}

	opts := svc.ResolvedOptions(field)
	if len(opts) != 1 {
		t.Fatalf("expected 1 placeholder option, got %d", len(opts))
	}
	if opts[0].Value != "" {
		t.Errorf("placeholder option value = %q, want empty", opts[0].Value)
	}
}

func TestProfileFilteredByConfiguredProviders(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	cfg := &config.Config{
		Models: &config.ModelsConfig{
			Providers: map[string]config.ModelProviderConfig{
				"gemini": {APIKeyRef: "keychain:obk/gemini"},
			},
		},
	}
	svc := testService(cfg)

	field := findField(svc, "models.profile")
	if field == nil {
		t.Fatal("models.profile not found")
	}

	opts := svc.ResolvedOptions(field)
	for _, o := range opts {
		if o.Value == "" {
			continue
		}
		p, ok := config.Profiles[o.Value]
		if !ok {
			continue
		}
		for _, req := range p.Providers {
			if req != "gemini" {
				t.Errorf("profile %q requires %q but only gemini is configured", o.Value, req)
			}
		}
	}
}

func TestAfterSetCalledOnAPIKey(t *testing.T) {
	cfg := config.Default()
	verified := false
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
		WithStoreCred(func(ref, value string) error { return nil }),
		WithLoadCred(func(ref string) (string, error) { return "key", nil }),
		WithVerifyProvider(func(name string, pcfg config.ModelProviderConfig) error {
			verified = true
			return nil
		}),
	)

	field := findField(svc, "models.providers.anthropic.api_key")
	if field == nil {
		t.Fatal("anthropic api_key not found")
	}
	if field.AfterSet == nil {
		t.Fatal("api_key field should have AfterSet")
	}

	if err := svc.SetValue(field, "sk-test"); err != nil {
		t.Fatal(err)
	}

	msg := field.AfterSet(svc)
	if !verified {
		t.Error("verifyProvider was not called")
	}
	if !strings.Contains(msg, "verified") {
		t.Errorf("AfterSet msg = %q, want to contain 'verified'", msg)
	}
}

func TestAfterSetVerifyFailure(t *testing.T) {
	cfg := config.Default()
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
		WithStoreCred(func(ref, value string) error { return nil }),
		WithLoadCred(func(ref string) (string, error) { return "key", nil }),
		WithVerifyProvider(func(name string, pcfg config.ModelProviderConfig) error {
			return fmt.Errorf("invalid key")
		}),
	)

	field := findField(svc, "models.providers.openai.api_key")
	if field == nil {
		t.Fatal("openai api_key not found")
	}

	if err := svc.SetValue(field, "sk-bad"); err != nil {
		t.Fatal(err)
	}

	msg := field.AfterSet(svc)
	if !strings.Contains(msg, "Warning") {
		t.Errorf("AfterSet msg = %q, want to contain 'Warning'", msg)
	}
	if !strings.Contains(msg, "invalid key") {
		t.Errorf("AfterSet msg = %q, want to contain error detail", msg)
	}
}

func TestResolvedOptionsFallsBackToStatic(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "mode")
	if field == nil {
		t.Fatal("mode field not found")
	}

	opts := svc.ResolvedOptions(field)
	if len(opts) != 3 {
		t.Errorf("mode options count = %d, want 3", len(opts))
	}
}

func TestVerifyProviderNilHandler(t *testing.T) {
	cfg := config.Default()
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
	)

	err := svc.VerifyProvider("anthropic", config.ModelProviderConfig{})
	if err != nil {
		t.Errorf("nil verifyProvider should return nil, got %v", err)
	}
}

func findField(svc *Service, key string) *Field {
	return findFieldInNodes(svc.Tree(), key)
}

func TestBackupEnabledRequiresDestination(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if field == nil {
		t.Fatal("backup.enabled field not found")
	}

	// Enabling without any destination should fail.
	err := svc.SetValue(field, "true")
	if err == nil {
		t.Fatal("expected error enabling backup without destination")
	}
	if !strings.Contains(err.Error(), "select a destination") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBackupEnabledRequiresR2Credentials(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Destination: "r2",
		R2:          &config.R2Config{},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")

	// Enabling with destination=r2 but no credentials should fail.
	err := svc.SetValue(field, "true")
	if err == nil {
		t.Fatal("expected error enabling backup without R2 credentials")
	}
	if !strings.Contains(err.Error(), "configure R2") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBackupEnabledSucceedsWithR2Configured(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Destination: "r2",
		R2: &config.R2Config{
			Bucket:       "test-bucket",
			Endpoint:     "https://example.r2.cloudflarestorage.com",
			AccessKeyRef: "keychain:obk/r2-access-key",
			SecretKeyRef: "keychain:obk/r2-secret-key",
		},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if err := svc.SetValue(field, "true"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !cfg.Backup.Enabled {
		t.Error("backup should be enabled")
	}
}

func TestBackupEnabledRequiresGDriveFolderID(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Destination: "gdrive",
		GDrive:      &config.GDriveConfig{},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")

	err := svc.SetValue(field, "true")
	if err == nil {
		t.Fatal("expected error enabling backup without GDrive folder ID")
	}
	if !strings.Contains(err.Error(), "Google Drive folder ID") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBackupEnabledSucceedsWithGDriveConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Destination: "gdrive",
		GDrive:      &config.GDriveConfig{FolderID: "1abc"},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if err := svc.SetValue(field, "true"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !cfg.Backup.Enabled {
		t.Error("backup should be enabled")
	}
}

func TestBackupDisableAlwaysAllowed(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{Enabled: true, Destination: "r2"}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if err := svc.SetValue(field, "false"); err != nil {
		t.Fatalf("disabling should always work: %v", err)
	}
	if cfg.Backup.Enabled {
		t.Error("backup should be disabled")
	}
}

func TestBackupR2FieldsHiddenWhenNotR2(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{Destination: "gdrive"}
	svc := testService(cfg)

	for _, key := range []string{"backup.r2.bucket", "backup.r2.endpoint", "backup.r2.access_key", "backup.r2.secret_key"} {
		field := findFieldInNodes(svc.Tree(), key)
		if field != nil {
			t.Errorf("%s should not be in tree when destination is gdrive", key)
		}
	}
}

func TestBackupR2FieldsVisibleWhenR2(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{Destination: "r2"}
	svc := testService(cfg)

	for _, key := range []string{"backup.r2.bucket", "backup.r2.endpoint", "backup.r2.access_key", "backup.r2.secret_key"} {
		field := findFieldInNodes(svc.Tree(), key)
		if field == nil {
			t.Fatalf("%s should be in tree when destination is r2", key)
		}
	}
}

func TestBackupScheduleReadOnlyWhenNotEnabled(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.schedule")
	if field == nil {
		t.Fatal("backup.schedule field not found")
	}
	if !field.ReadOnly(cfg) {
		t.Error("schedule should be read-only when backup is not enabled")
	}

	// Even with destination configured but not enabled, schedule is locked.
	cfg.Backup = &config.BackupConfig{
		Destination: "r2",
		R2: &config.R2Config{
			Bucket:       "b",
			Endpoint:     "e",
			AccessKeyRef: "keychain:obk/r2-access-key",
			SecretKeyRef: "keychain:obk/r2-secret-key",
		},
	}
	if !field.ReadOnly(cfg) {
		t.Error("schedule should be read-only when destination is configured but backup not enabled")
	}
}

func TestBackupScheduleEditableWhenEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Enabled:     true,
		Destination: "r2",
		R2: &config.R2Config{
			Bucket:       "b",
			Endpoint:     "e",
			AccessKeyRef: "keychain:obk/r2-access-key",
			SecretKeyRef: "keychain:obk/r2-secret-key",
		},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.schedule")
	if field.ReadOnly(cfg) {
		t.Error("schedule should be editable when backup is enabled")
	}
}

func TestBackupEnabledReadOnlyWithoutDestination(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if field == nil {
		t.Fatal("backup.enabled field not found")
	}
	if field.ReadOnly == nil {
		t.Fatal("backup.enabled should have ReadOnly")
	}
	if !field.ReadOnly(cfg) {
		t.Error("enabled should be read-only when no destination is configured")
	}
}

func TestBackupEnabledEditableWhenDestConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Destination: "r2",
		R2: &config.R2Config{
			Bucket:       "b",
			Endpoint:     "e",
			AccessKeyRef: "keychain:obk/r2-access-key",
			SecretKeyRef: "keychain:obk/r2-secret-key",
		},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if field.ReadOnly(cfg) {
		t.Error("enabled should be editable when destination is fully configured")
	}
}

func TestBackupEnabledEditableWhenAlreadyEnabled(t *testing.T) {
	// Even if destination becomes unconfigured, user can still toggle OFF.
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Enabled:     true,
		Destination: "gdrive",
		GDrive:      &config.GDriveConfig{},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.enabled")
	if field.ReadOnly(cfg) {
		t.Error("enabled should be editable when already enabled (so user can disable)")
	}
}

func TestBackupDestinationChangeResetsEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Enabled:     true,
		Destination: "r2",
		R2: &config.R2Config{
			Bucket:       "b",
			Endpoint:     "e",
			AccessKeyRef: "keychain:obk/r2-access-key",
			SecretKeyRef: "keychain:obk/r2-secret-key",
		},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.destination")
	if err := svc.SetValue(field, "gdrive"); err != nil {
		t.Fatal(err)
	}
	if cfg.Backup.Enabled {
		t.Error("changing destination should reset enabled to false")
	}
}

func TestBackupDestinationSameValueKeepsEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Backup = &config.BackupConfig{
		Enabled:     true,
		Destination: "r2",
		R2: &config.R2Config{
			Bucket:       "b",
			Endpoint:     "e",
			AccessKeyRef: "keychain:obk/r2-access-key",
			SecretKeyRef: "keychain:obk/r2-secret-key",
		},
	}
	svc := testService(cfg)

	field := findFieldInNodes(svc.Tree(), "backup.destination")
	if err := svc.SetValue(field, "r2"); err != nil {
		t.Fatal(err)
	}
	if !cfg.Backup.Enabled {
		t.Error("setting same destination should keep enabled=true")
	}
}

func TestIsBackupDestConfiguredR2(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	// Not configured at all.
	if svc.IsBackupDestConfigured("r2") {
		t.Error("r2 should not be configured when backup is nil")
	}

	// Partially configured.
	cfg.Backup = &config.BackupConfig{
		Destination: "r2",
		R2:          &config.R2Config{Bucket: "b"},
	}
	if svc.IsBackupDestConfigured("r2") {
		t.Error("r2 should not be configured when only bucket is set")
	}

	// Fully configured.
	cfg.Backup.R2 = &config.R2Config{
		Bucket:       "b",
		Endpoint:     "e",
		AccessKeyRef: "ref-ak",
		SecretKeyRef: "ref-sk",
	}
	if !svc.IsBackupDestConfigured("r2") {
		t.Error("r2 should be configured when all fields are set")
	}
}

func TestIsBackupDestConfiguredGDrive(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	if svc.IsBackupDestConfigured("gdrive") {
		t.Error("gdrive should not be configured when backup is nil")
	}

	cfg.Backup = &config.BackupConfig{
		Destination: "gdrive",
		GDrive:      &config.GDriveConfig{},
	}
	if svc.IsBackupDestConfigured("gdrive") {
		t.Error("gdrive should not be configured when folder_id is empty")
	}

	cfg.Backup.GDrive.FolderID = "abc123"
	if !svc.IsBackupDestConfigured("gdrive") {
		t.Error("gdrive should be configured when folder_id is set")
	}
}

func findFieldInNodes(nodes []Node, key string) *Field {
	for _, n := range nodes {
		if n.Field != nil && n.Field.Key == key {
			return n.Field
		}
		if n.Category != nil {
			if f := findFieldInNodes(n.Category.Children, key); f != nil {
				return f
			}
		}
	}
	return nil
}

func TestTelegramAuthStatusField(t *testing.T) {
	// Isolated config dir, so the status does not depend on whether the
	// machine running the tests happens to be linked.
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	// The gate asks whether credentials resolve, so the keyring and the
	// environment both have to be this test's own.
	keyring.MockInit()
	t.Setenv("TELEGRAM_API_ID", "")
	t.Setenv("TELEGRAM_API_HASH", "")

	cfg := config.Default()
	svc := New(cfg,
		WithSaveFn(func(*config.Config) error { return nil }),
		WithStoreCred(provider.StoreCredential),
		WithLoadCred(provider.LoadCredential),
	)

	field := findField(svc, "telegram.auth_status")
	if field == nil {
		t.Fatal("telegram.auth_status field not found")
	}

	// Always read-only: pressing Enter dispatches the login flow instead of
	// editing the value.
	if field.ReadOnly == nil || !field.ReadOnly(cfg) {
		t.Fatal("telegram.auth_status must be permanently read-only")
	}

	// Signing in is pointless without app credentials, so it stays gated.
	if got := svc.GetValue(field); !strings.Contains(got, "api_id") {
		t.Fatalf("expected a credentials prompt before setup, got %q", got)
	}

	apiID := findField(svc, "telegram.api_id")
	apiHash := findField(svc, "telegram.api_hash")
	if apiID == nil || apiHash == nil {
		t.Fatal("telegram credential fields not found")
	}
	if err := svc.SetValue(apiID, "1234567"); err != nil {
		t.Fatalf("set api_id: %v", err)
	}
	if err := svc.SetValue(apiHash, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set api_hash: %v", err)
	}

	if !IsTelegramConfigured() {
		t.Fatal("credentials should be recorded in config")
	}
	if got := svc.GetValue(field); !strings.Contains(got, "Not connected") {
		t.Fatalf("expected a sign-in prompt once configured, got %q", got)
	}

	// Credentials live in the keyring, never in config.yaml.
	if cfg.Telegram.APIIDRef != "keychain:obk/telegram/api_id" {
		t.Fatalf("api_id ref = %q", cfg.Telegram.APIIDRef)
	}
	if cfg.Telegram.APIHashRef != "keychain:obk/telegram/api_hash" {
		t.Fatalf("api_hash ref = %q", cfg.Telegram.APIHashRef)
	}
	if got := svc.GetValue(apiHash); strings.Contains(got, "0123456789abcdef0123456789abcdef") {
		t.Fatalf("api_hash must be masked, got %q", got)
	}
}

func TestTelegramAPIIDValidation(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "telegram.api_id")
	if field == nil {
		t.Fatal("telegram.api_id field not found")
	}
	if err := svc.SetValue(field, "not-a-number"); err == nil {
		t.Fatal("expected a non-numeric api_id to be rejected")
	}
}

// A truncated paste should be caught here, not as an opaque MTProto error at
// login.
func TestTelegramAPIHashValidation(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "telegram.api_hash")
	if field == nil {
		t.Fatal("telegram.api_hash field not found")
	}
	for _, bad := range []string{"0123456789abcdef", "0123456789abcdef0123456789abcdefff", "zzz3456789abcdef0123456789abcdef"} {
		if err := svc.SetValue(field, bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
	if err := svc.SetValue(field, "0123456789ABCDEF0123456789abcdef"); err != nil {
		t.Fatalf("a valid api_hash was rejected: %v", err)
	}
}

// The account row and the credential rows must answer the same question. They
// disagreed once: the account row asked the source package, which reads the
// environment, while the credential rows read a config ref that a headless
// install never writes.
func TestTelegramCredentialRowsAgreeWithEnv(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	keyring.MockInit()
	t.Setenv("TELEGRAM_API_ID", "1234567")
	t.Setenv("TELEGRAM_API_HASH", "0123456789abcdef0123456789abcdef")

	cfg := config.Default()
	svc := testService(cfg)
	for _, key := range []string{"telegram.api_id", "telegram.api_hash"} {
		field := findField(svc, key)
		if field == nil {
			t.Fatalf("%s field not found", key)
		}
		if got := svc.GetValue(field); strings.Contains(got, "not configured") {
			t.Fatalf("%s = %q while the account row reports configured", key, got)
		}
	}
}

// A headless install configures Telegram through the environment, which no
// config ref records. Calling that unconfigured blocks login on a setup that
// works.
func TestTelegramConfiguredFromEnvironment(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	keyring.MockInit()
	t.Setenv("TELEGRAM_API_ID", "1234567")
	t.Setenv("TELEGRAM_API_HASH", "0123456789abcdef0123456789abcdef")

	cfg := config.Default()
	if !IsTelegramConfigured() {
		t.Fatal("credentials from the environment must count as configured")
	}

	svc := testService(cfg)
	field := findField(svc, "telegram.auth_status")
	if field == nil {
		t.Fatal("telegram.auth_status field not found")
	}
	if got := svc.GetValue(field); strings.Contains(got, "api_id") {
		t.Fatalf("expected no credentials prompt with the environment set, got %q", got)
	}
}

func TestTelegramBackfillDaysField(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "telegram.backfill_days")
	if field == nil {
		t.Fatal("telegram.backfill_days field not found")
	}
	if got := svc.GetValue(field); got != "90" {
		t.Fatalf("default backfill window = %q, want 90", got)
	}

	if err := svc.SetValue(field, "30"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if cfg.Telegram.BackfillDays != 30 {
		t.Fatalf("backfill days = %d", cfg.Telegram.BackfillDays)
	}

	if err := svc.SetValue(field, "0"); err == nil {
		t.Fatal("expected a zero backfill window to be rejected")
	}
}

func TestXAuthStatusField(t *testing.T) {
	cfg := config.Default()
	svc := testService(cfg)

	field := findField(svc, "x.auth_status")
	if field == nil {
		t.Fatal("x.auth_status field not found")
	}

	got := svc.GetValue(field)
	if !strings.Contains(got, "Not connected") && !strings.Contains(got, "Connected") {
		t.Errorf("unexpected auth status: %q", got)
	}
}

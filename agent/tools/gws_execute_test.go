package tools

import (
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/oauth/google"
)

func setupGWSTest(t *testing.T, approveAll bool, scopes map[string]bool) (*GWSExecuteTool, *mockInteractor, *mockRunner) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tokens.db")
	credPath := writeTestCreds(t, dir)

	store, err := google.NewTokenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := store.SaveToken("user@test.com", tok, []string{"openid", "email", "calendar"}); err != nil {
		t.Fatal(err)
	}
	store.Close()

	g := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     dbPath,
	})
	bridge := NewTokenBridge(g, "user@test.com")

	inter := &mockInteractor{approveAll: approveAll}
	runner := &mockRunner{outputs: make(map[string]string)}

	if scopes == nil {
		scopes = map[string]bool{"https://www.googleapis.com/auth/calendar": true}
	}
	checker := &mockScopeChecker{scopes: scopes}

	manifest := &skills.Manifest{
		Skills: map[string]skills.SkillEntry{
			"gws-calendar-list":   {Source: "gws", Scopes: []string{"calendar"}, Write: false},
			"gws-calendar-edit":   {Source: "gws", Scopes: []string{"calendar"}, Write: true},
			"gws-calendar-agenda": {Source: "gws", Scopes: []string{"calendar"}, Write: false},
			"gws-calendar-insert": {Source: "gws", Scopes: []string{"calendar"}, Write: true},
		},
	}

	tool := NewGWSExecuteTool(GWSToolConfig{
		Interactor:   inter,
		ScopeChecker: checker,
		Bridge:       bridge,
		ScopeWaiter:  google.NewScopeWaiter(),
		Google:       g,
		Account:      "user@test.com",
		Manifest:     manifest,
		Runner:       runner,
		AuthTimeout:  100 * time.Millisecond,
	})

	return tool, inter, runner
}

func TestGWSExecute_ReadPath(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, false, nil)
	runner.outputs["calendar events list"] = `{"items":[]}`

	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != `{"items":[]}` {
		t.Errorf("result = %q", result)
	}
	// No approval should have been requested.
	if len(inter.approvals) > 0 {
		t.Error("read path should not request approval")
	}
	// Token should be in env.
	if len(runner.ran) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runner.ran))
	}
	envStr := strings.Join(runner.ran[0].env, " ")
	if !strings.Contains(envStr, "GOOGLE_WORKSPACE_CLI_TOKEN=") {
		t.Errorf("token not injected in env: %v", runner.ran[0].env)
	}
}

func TestGWSExecute_WriteApproved(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, true, nil)
	runner.outputs["calendar +insert --json {}"] = "event created"

	input, _ := json.Marshal(gwsInput{Command: "calendar +insert --json {}"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "event created" {
		t.Errorf("result = %q", result)
	}
	if len(inter.approvals) != 1 {
		t.Errorf("expected 1 approval request, got %d", len(inter.approvals))
	}
	// Should have "Done." notification.
	hasNotification := false
	for _, n := range inter.notified {
		if n == "Done." {
			hasNotification = true
		}
	}
	if !hasNotification {
		t.Errorf("missing Done notification, got %v", inter.notified)
	}
}

func TestGWSExecute_WriteDenied(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, false, nil)

	input, _ := json.Marshal(gwsInput{Command: "calendar +insert --json {}"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "denied_by_user" {
		t.Errorf("result = %q, want denied_by_user", result)
	}
	if len(runner.ran) > 0 {
		t.Error("command should not have been executed when denied")
	}
	hasNotification := false
	for _, n := range inter.notified {
		if n == "Action not performed." {
			hasNotification = true
		}
	}
	if !hasNotification {
		t.Errorf("missing denial notification, got %v", inter.notified)
	}
	_ = inter
}

func TestGWSExecute_MissingScopeReturnsAuthMessage(t *testing.T) {
	tool, inter, _ := setupGWSTest(t, false, map[string]bool{})

	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != authPendingMessage {
		t.Errorf("result = %q, want auth pending message", result)
	}
	if len(inter.links) == 0 {
		t.Error("expected auth link to be sent")
	}
}

func TestGWSExecute_MissingScopeRetrySucceeds(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, false, map[string]bool{})
	runner.outputs["calendar events list"] = `{"items":[]}`

	// First call: scopes missing → returns auth pending message.
	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute (1st): %v", err)
	}
	if result != authPendingMessage {
		t.Errorf("result = %q, want auth pending message", result)
	}
	if len(inter.links) == 0 {
		t.Error("expected auth link to be sent")
	}

	// Simulate: user completes OAuth, scopes now present.
	tool.scopeChecker = &mockScopeChecker{scopes: map[string]bool{
		"https://www.googleapis.com/auth/calendar": true,
	}}

	// Second call: scopes present → succeeds.
	result, err = tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute (2nd): %v", err)
	}
	if result != `{"items":[]}` {
		t.Errorf("result = %q", result)
	}
}

// toggleScopeChecker returns false on first call, true after.
type toggleScopeChecker struct {
	called         int
	hasAfterSignal bool
}

func (c *toggleScopeChecker) HasScopes(_ string, _ []string) (bool, error) {
	c.called++
	if c.called == 1 {
		return false, nil
	}
	return c.hasAfterSignal, nil
}

func TestGWSExecute_ReadOnlyHelperNotWrite(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, false, nil)
	runner.outputs["calendar +agenda"] = `{"events":[]}`

	input, _ := json.Marshal(gwsInput{Command: "calendar +agenda"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != `{"events":[]}` {
		t.Errorf("result = %q", result)
	}
	if len(inter.approvals) > 0 {
		t.Error("+agenda is read-only, should not trigger approval")
	}
}

func TestGWSExecute_KeywordNotWrite(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, false, nil)
	runner.outputs["calendar events delete --id 123"] = "deleted"

	input, _ := json.Marshal(gwsInput{Command: "calendar events delete --id 123"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "deleted" {
		t.Errorf("result = %q", result)
	}
	if len(inter.approvals) > 0 {
		t.Error("keyword 'delete' without '+' prefix should not trigger approval")
	}
}

func TestGWSExecute_ScopesForService(t *testing.T) {
	manifest := &skills.Manifest{
		Skills: map[string]skills.SkillEntry{
			"gws-docs-read":  {Source: "gws", Scopes: []string{"docs"}},
			"gws-drive-list": {Source: "gws", Scopes: []string{"drive"}},
		},
	}
	tool := &GWSExecuteTool{manifest: manifest}

	tests := []struct {
		service string
		want    string
	}{
		{"docs", "https://www.googleapis.com/auth/documents"},
		{"drive", "https://www.googleapis.com/auth/drive"},
		// gmail has no manifest entry but exists in serviceToScope — fallback works.
		{"gmail", "https://www.googleapis.com/auth/gmail.modify"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		scopes := tool.scopesForService(tt.service)
		if tt.want == "" {
			if scopes != nil {
				t.Errorf("scopesForService(%q) = %v, want nil", tt.service, scopes)
			}
		} else if len(scopes) != 1 || scopes[0] != tt.want {
			t.Errorf("scopesForService(%q) = %v, want [%s]", tt.service, scopes, tt.want)
		}
	}
}

func TestGWSExecute_StripGWSPrefix(t *testing.T) {
	tool, _, runner := setupGWSTest(t, false, nil)
	runner.outputs["calendar events list"] = `{"items":[]}`

	input, _ := json.Marshal(gwsInput{Command: "gws calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != `{"items":[]}` {
		t.Errorf("result = %q", result)
	}
	if len(runner.ran) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runner.ran))
	}
	if runner.ran[0].args[0] != "calendar" {
		t.Errorf("first arg = %q, want 'calendar'", runner.ran[0].args[0])
	}
}

func TestGWSExecute_StructuredParams(t *testing.T) {
	tool, _, runner := setupGWSTest(t, false, map[string]bool{
		"https://www.googleapis.com/auth/calendar": true,
		"https://www.googleapis.com/auth/drive":    true,
	})
	runner.outputs[`drive files list --params {"orderBy":"modifiedTime desc","pageSize":5,"q":"mimeType='application/vnd.google-apps.document'"}`] = `{"files":[]}`

	input, _ := json.Marshal(map[string]any{
		"command": "drive files list",
		"params": map[string]any{
			"q":        "mimeType='application/vnd.google-apps.document'",
			"orderBy":  "modifiedTime desc",
			"pageSize": 5,
		},
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != `{"files":[]}` {
		t.Errorf("result = %q", result)
	}
	if len(runner.ran) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runner.ran))
	}
	// Verify --params is a single arg with valid JSON.
	args := runner.ran[0].args
	paramsIdx := -1
	for i, a := range args {
		if a == "--params" {
			paramsIdx = i
			break
		}
	}
	if paramsIdx < 0 || paramsIdx+1 >= len(args) {
		t.Fatal("--params flag not found in args")
	}
	if !json.Valid([]byte(args[paramsIdx+1])) {
		t.Errorf("--params value is not valid JSON: %s", args[paramsIdx+1])
	}
}

func TestGWSExecute_StructuredBody(t *testing.T) {
	tool, interactor, runner := setupGWSTest(t, false, nil)
	interactor.approveAll = true
	// Body for a helper command (+insert) is now expanded to individual flags (sorted keys).
	runner.outputs[`calendar +insert --location Room 1 --summary Team meeting`] = `{"id":"abc"}`

	input, _ := json.Marshal(map[string]any{
		"command": "calendar +insert",
		"body": map[string]any{
			"summary":  "Team meeting",
			"location": "Room 1",
		},
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != `{"id":"abc"}` {
		t.Errorf("result = %q", result)
	}
	// Verify expanded args directly.
	args := runner.ran[0].args
	expected := []string{"calendar", "+insert", "--location", "Room 1", "--summary", "Team meeting"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestGWSExecute_TokenExpiredReturnsAuthMessage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tokens.db")
	credPath := writeTestCreds(t, dir)

	store, err := google.NewTokenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{
		AccessToken: "expired-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(-time.Hour),
	}
	if err := store.SaveToken("user@test.com", tok, []string{"openid", "email", "calendar"}); err != nil {
		t.Fatal(err)
	}
	store.Close()

	g := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     dbPath,
	})
	bridge := NewTokenBridge(g, "user@test.com")
	inter := &mockInteractor{approveAll: false}
	runner := &mockRunner{outputs: map[string]string{"calendar events list": `{"items":[]}`}}

	checker := &mockScopeChecker{scopes: map[string]bool{
		"https://www.googleapis.com/auth/calendar": true,
	}}

	tool := NewGWSExecuteTool(GWSToolConfig{
		Interactor:   inter,
		ScopeChecker: checker,
		Bridge:       bridge,
		ScopeWaiter:  google.NewScopeWaiter(),
		Google:       g,
		Account:      "user@test.com",
		Manifest:     &skills.Manifest{Skills: map[string]skills.SkillEntry{"gws-calendar-list": {Source: "gws", Scopes: []string{"calendar"}}}},
		Runner:       runner,
		AuthTimeout:  100 * time.Millisecond,
	})

	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != authPendingMessage {
		t.Errorf("result = %q, want auth pending message", result)
	}
	if len(inter.links) == 0 {
		t.Error("expected re-auth link to be sent")
	}
}

func TestGWSExecute_EmptyCommand(t *testing.T) {
	tool, _, _ := setupGWSTest(t, false, nil)
	input, _ := json.Marshal(gwsInput{Command: ""})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// writeTestCreds for gws_execute_test.go is defined in token_bridge_test.go
// since both are in the same package.

func TestGWSExecute_Name(t *testing.T) {
	dir := t.TempDir()
	credPath := writeTestCreds(t, dir)
	dbPath := filepath.Join(dir, "tokens.db")
	// Write an empty token db so NewTokenStore can init.
	os.WriteFile(dbPath, nil, 0600)

	g := google.New(google.Config{CredentialsFile: credPath, TokenDBPath: dbPath})
	tool := NewGWSExecuteTool(GWSToolConfig{
		Interactor:   &mockInteractor{},
		ScopeChecker: &mockScopeChecker{scopes: map[string]bool{}},
		Bridge:       NewTokenBridge(g, ""),
		ScopeWaiter:  google.NewScopeWaiter(),
		Google:       g,
		Manifest:     &skills.Manifest{Skills: map[string]skills.SkillEntry{}},
		Runner:       &mockRunner{outputs: map[string]string{}},
	})
	if tool.Name() != "gws_execute" {
		t.Errorf("Name() = %q", tool.Name())
	}
	if !json.Valid(tool.InputSchema()) {
		t.Error("invalid input schema")
	}
}

func TestGWSExecute_ServiceDisabledAnnotation(t *testing.T) {
	tool, inter, runner := setupGWSTest(t, false, nil)

	apiError := `{"error":{"code":403,"message":"Google Calendar API has not been used in project 123456 before or it is disabled. Enable it by visiting https://console.developers.google.com/apis/api/calendar-json.googleapis.com/overview?project=123456 then retry.","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","domain":"googleapis.com"}]}}`
	runner.outputs["calendar events list"] = apiError
	runner.errs = map[string]error{"calendar events list": fmt.Errorf("exit status 1")}

	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for disabled API")
	}
	if !strings.Contains(result, "SERVICE_DISABLED") {
		t.Error("result should contain original SERVICE_DISABLED error")
	}
	if !strings.Contains(result, "console.developers.google.com") {
		t.Error("result should preserve the activation URL")
	}
	if !strings.Contains(result, "The Google API for this service is not enabled") {
		t.Error("result should contain annotation note")
	}
	// No re-auth should be triggered (re-auth can't fix a disabled API).
	if len(inter.links) > 0 {
		t.Error("SERVICE_DISABLED should not trigger re-auth")
	}
	// Command should only run once (no retry).
	if len(runner.ran) != 1 {
		t.Errorf("expected 1 run, got %d", len(runner.ran))
	}
}

// scopeRetryRunner fails with ACCESS_TOKEN_SCOPE_INSUFFICIENT on first call,
// then succeeds on retry.
type scopeRetryRunner struct {
	calls int
	ran   []struct{ args, env []string }
}

func (r *scopeRetryRunner) Run(_ context.Context, args []string, env []string) (string, error) {
	r.ran = append(r.ran, struct{ args, env []string }{args, env})
	r.calls++
	if r.calls == 1 {
		return `{"error":{"code":403,"status":"PERMISSION_DENIED","details":[{"reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`, fmt.Errorf("exit status 1")
	}
	return `{"items":[]}`, nil
}

func TestGWSExecute_ScopeInsufficientReturnsAuthMessage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tokens.db")
	credPath := writeTestCreds(t, dir)

	store, err := google.NewTokenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := store.SaveToken("user@test.com", tok, []string{"openid", "email", "calendar"}); err != nil {
		t.Fatal(err)
	}
	store.Close()

	g := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     dbPath,
	})
	bridge := NewTokenBridge(g, "user@test.com")
	inter := &mockInteractor{approveAll: false}
	runner := &scopeRetryRunner{}

	checker := &mockScopeChecker{scopes: map[string]bool{
		"https://www.googleapis.com/auth/calendar": true,
	}}

	tool := NewGWSExecuteTool(GWSToolConfig{
		Interactor:   inter,
		ScopeChecker: checker,
		Bridge:       bridge,
		ScopeWaiter:  google.NewScopeWaiter(),
		Google:       g,
		Account:      "user@test.com",
		Manifest: &skills.Manifest{
			Skills: map[string]skills.SkillEntry{
				"gws-calendar-list": {Source: "gws", Scopes: []string{"calendar"}},
			},
		},
		Runner:      runner,
		AuthTimeout: 100 * time.Millisecond,
	})

	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != authPendingMessage {
		t.Errorf("result = %q, want auth pending message", result)
	}
	if runner.calls != 1 {
		t.Errorf("expected 1 runner call (no retry), got %d", runner.calls)
	}
	if len(inter.links) == 0 {
		t.Error("expected re-auth link to be sent")
	}
}

func TestGWSExecute_TruncatesLargeOutput(t *testing.T) {
	tool, _, runner := setupGWSTest(t, false, nil)
	// Mock runner returns 60KB output.
	bigOutput := strings.Repeat("line\n", 12000)
	runner.outputs["calendar events list"] = bigOutput

	input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "[truncated: showing 500+500 of") {
		t.Error("expected head+tail truncation marker")
	}
	if len(result) >= len(bigOutput) {
		t.Errorf("result should be truncated: got %d bytes, original %d", len(result), len(bigOutput))
	}
}

func TestGWSExecute_AuthRedirectWrapsURL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tokens.db")
	credPath := writeTestCreds(t, dir)

	store, err := google.NewTokenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := store.SaveToken("user@test.com", tok, []string{"openid", "email"}); err != nil {
		t.Fatal(err)
	}
	store.Close()

	g := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     dbPath,
	})

	linkCh := make(chan struct{ text, url string }, 1)
	inter := &mockInteractor{approveAll: false, linkCh: linkCh}

	waiter := google.NewScopeWaiter()
	checker := &mockScopeChecker{scopes: map[string]bool{}}

	tool := NewGWSExecuteTool(GWSToolConfig{
		Interactor:      inter,
		ScopeChecker:    checker,
		Bridge:          NewTokenBridge(g, "user@test.com"),
		ScopeWaiter:     waiter,
		Google:          g,
		Account:         "user@test.com",
		Manifest:        &skills.Manifest{Skills: map[string]skills.SkillEntry{"gws-calendar-list": {Source: "gws", Scopes: []string{"calendar"}}}},
		Runner:          &mockRunner{outputs: map[string]string{}},
		AuthTimeout:     100 * time.Millisecond,
		AuthRedirectURL: "https://example.ngrok-free.app/auth/redirect",
	})

	go func() {
		input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
		tool.Execute(context.Background(), input)
	}()

	var link struct{ text, url string }
	select {
	case link = <-linkCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for auth link")
	}

	if !strings.HasPrefix(link.url, "https://example.ngrok-free.app/auth/redirect?url=") {
		t.Fatalf("link should start with trampoline URL, got %q", link.url)
	}
	// The url param should decode to a Google OAuth URL.
	parsed, err := neturl.Parse(link.url)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	inner := parsed.Query().Get("url")
	if !strings.HasPrefix(inner, "https://accounts.google.com/") {
		t.Errorf("inner URL should be Google OAuth, got %q", inner)
	}
}

func TestGWSExecute_NoAuthRedirectDirectURL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tokens.db")
	credPath := writeTestCreds(t, dir)

	store, err := google.NewTokenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := store.SaveToken("user@test.com", tok, []string{"openid", "email"}); err != nil {
		t.Fatal(err)
	}
	store.Close()

	g := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     dbPath,
	})

	linkCh := make(chan struct{ text, url string }, 1)
	inter := &mockInteractor{approveAll: false, linkCh: linkCh}

	waiter := google.NewScopeWaiter()
	checker := &mockScopeChecker{scopes: map[string]bool{}}

	tool := NewGWSExecuteTool(GWSToolConfig{
		Interactor:   inter,
		ScopeChecker: checker,
		Bridge:       NewTokenBridge(g, "user@test.com"),
		ScopeWaiter:  waiter,
		Google:       g,
		Account:      "user@test.com",
		Manifest:     &skills.Manifest{Skills: map[string]skills.SkillEntry{"gws-calendar-list": {Source: "gws", Scopes: []string{"calendar"}}}},
		Runner:       &mockRunner{outputs: map[string]string{}},
		AuthTimeout:  100 * time.Millisecond,
		// AuthRedirectURL intentionally empty
	})

	go func() {
		input, _ := json.Marshal(gwsInput{Command: "calendar events list"})
		tool.Execute(context.Background(), input)
	}()

	var link struct{ text, url string }
	select {
	case link = <-linkCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for auth link")
	}

	if !strings.HasPrefix(link.url, "https://accounts.google.com/") {
		t.Fatalf("link should go directly to Google, got %q", link.url)
	}
}

func TestGWSExecute_FlagsField(t *testing.T) {
	tool, interactor, runner := setupGWSTest(t, false, map[string]bool{
		"https://www.googleapis.com/auth/documents": true,
	})
	interactor.approveAll = true

	// Manifest entry for docs +write so it's treated as write.
	tool.manifest.Skills["gws-docs-write"] = skills.SkillEntry{Source: "gws", Scopes: []string{"docs"}, Write: true}

	input, _ := json.Marshal(map[string]any{
		"command": "docs +write",
		"flags": map[string]any{
			"document": "DOC_ID",
			"text":     "Hello world",
		},
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := runner.ran[0].args
	expected := []string{"docs", "+write", "--document", "DOC_ID", "--text", "Hello world"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestGWSExecute_BodyExpandedForHelper(t *testing.T) {
	tool, interactor, runner := setupGWSTest(t, false, nil)
	interactor.approveAll = true

	input, _ := json.Marshal(map[string]any{
		"command": "calendar +insert",
		"body": map[string]any{
			"summary": "Standup",
			"start":   "2025-01-01T09:00:00Z",
			"end":     "2025-01-01T09:30:00Z",
		},
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := runner.ran[0].args
	// Keys sorted: end, start, summary.
	expected := []string{"calendar", "+insert", "--end", "2025-01-01T09:30:00Z", "--start", "2025-01-01T09:00:00Z", "--summary", "Standup"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestGWSExecute_BodyJsonForRawAPI(t *testing.T) {
	tool, _, runner := setupGWSTest(t, false, map[string]bool{
		"https://www.googleapis.com/auth/drive": true,
	})

	input, _ := json.Marshal(map[string]any{
		"command": "drive files create",
		"body": map[string]any{
			"name":     "test.txt",
			"mimeType": "text/plain",
		},
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := runner.ran[0].args
	// No + command → body passed as --json.
	jsonIdx := -1
	for i, a := range args {
		if a == "--json" {
			jsonIdx = i
			break
		}
	}
	if jsonIdx < 0 || jsonIdx+1 >= len(args) {
		t.Fatalf("expected --json flag in args: %v", args)
	}
	if !json.Valid([]byte(args[jsonIdx+1])) {
		t.Errorf("--json value is not valid JSON: %s", args[jsonIdx+1])
	}
}

func TestGWSExecute_FlagsArrayRepeated(t *testing.T) {
	tool, interactor, runner := setupGWSTest(t, false, nil)
	interactor.approveAll = true

	input, _ := json.Marshal(map[string]any{
		"command": "calendar +insert",
		"flags": map[string]any{
			"attendee": []string{"alice@example.com", "bob@example.com"},
			"summary":  "Team sync",
		},
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := runner.ran[0].args
	// Keys sorted: attendee (repeated), summary.
	expected := []string{"calendar", "+insert", "--attendee", "alice@example.com", "--attendee", "bob@example.com", "--summary", "Team sync"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestGWSExecute_FlagsBooleanBare(t *testing.T) {
	tool, _, runner := setupGWSTest(t, false, nil)

	input, _ := json.Marshal(map[string]any{
		"command": "calendar +agenda",
		"flags": map[string]any{
			"today":   true,
			"verbose": false,
		},
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := runner.ran[0].args
	// "today" true → bare flag, "verbose" false → omitted.
	expected := []string{"calendar", "+agenda", "--today"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestGWSExecute_FlagsHyphenatedKeys(t *testing.T) {
	tool, interactor, runner := setupGWSTest(t, false, map[string]bool{
		"https://www.googleapis.com/auth/spreadsheets": true,
	})
	interactor.approveAll = true

	// Add manifest entry for sheets +append.
	tool.manifest.Skills["gws-sheets-append"] = skills.SkillEntry{Source: "gws", Scopes: []string{"sheets"}, Write: true}

	input, _ := json.Marshal(map[string]any{
		"command": "sheets +append",
		"flags": map[string]any{
			"json-values": "[[1,2,3]]",
			"spreadsheet": "SHEET_ID",
		},
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := runner.ran[0].args
	// Keys sorted: json-values, spreadsheet.
	expected := []string{"sheets", "+append", "--json-values", "[[1,2,3]]", "--spreadsheet", "SHEET_ID"}
	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestSplitCommand_Quoted(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{
			input: `docs +write --text 'Hello world'`,
			want:  []string{"docs", "+write", "--text", "Hello world"},
		},
		{
			input: `docs +write --text "Hello world"`,
			want:  []string{"docs", "+write", "--text", "Hello world"},
		},
		{
			input: `drive +upload --name "my file.pdf" ./local.pdf`,
			want:  []string{"drive", "+upload", "--name", "my file.pdf", "./local.pdf"},
		},
		{
			input: `docs +write --document DOC_ID --text 'Line 1 and Line 2'`,
			want:  []string{"docs", "+write", "--document", "DOC_ID", "--text", "Line 1 and Line 2"},
		},
	}
	for _, tt := range tests {
		got := splitCommand(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCommand(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCommand(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestSplitCommand_NoQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "calendar events list",
			want:  []string{"calendar", "events", "list"},
		},
		{
			input: "drive files list --id abc",
			want:  []string{"drive", "files", "list", "--id", "abc"},
		},
		{
			input: "  calendar  +agenda  ",
			want:  []string{"calendar", "+agenda"},
		},
	}
	for _, tt := range tests {
		got := splitCommand(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCommand(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCommand(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

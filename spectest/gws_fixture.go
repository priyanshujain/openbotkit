package spectest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/73ai/openbotkit/agent"
	"github.com/73ai/openbotkit/agent/tools"
	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/oauth/google"
)

// GWSMockRunner records commands and returns canned responses.
// Matches are prefix-based: the key must be a prefix of the joined args.
type GWSMockRunner struct {
	mu        sync.Mutex
	Responses map[string]string
	Calls     []GWSCall
}

type GWSCall struct {
	Args []string
}

func (r *GWSMockRunner) Run(_ context.Context, args []string, _ []string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Calls = append(r.Calls, GWSCall{Args: args})

	// Normalize: expand dots to spaces so "files.list" matches "files list".
	joined := strings.Join(args, " ")
	normalized := strings.ReplaceAll(joined, ".", " ")
	for prefix, resp := range r.Responses {
		if strings.HasPrefix(joined, prefix) || strings.HasPrefix(normalized, prefix) {
			return resp, nil
		}
	}
	return fmt.Sprintf(`{"error":{"code":400,"message":"unrecognized command: %s","reason":"validationError"}}`, joined),
		fmt.Errorf("gws: exit status 1")
}

// GWSAgent creates an agent with gws_execute registered using a mock runner.
// GWS skills must be installed first via InstallGWSSkills.
func (f *LocalFixture) GWSAgent(t *testing.T, runner *GWSMockRunner) *agent.Agent {
	t.Helper()

	dir := filepath.Join(f.dir, "google")
	os.MkdirAll(dir, 0700)
	dbPath := filepath.Join(dir, "tokens.db")
	credPath := filepath.Join(dir, "credentials.json")

	os.WriteFile(credPath, []byte(gwsTestCredentials), 0600)

	store, err := google.NewTokenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	store.SaveToken("test@example.com", tok, []string{
		"openid", "email",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/spreadsheets",
	})
	store.Close()

	g := google.New(google.Config{
		CredentialsFile: credPath,
		TokenDBPath:     dbPath,
	})
	bridge := tools.NewTokenBridge(g, "test@example.com")

	manifest, _ := skills.LoadManifest()

	toolReg := tools.NewRegistry()
	toolReg.Register(tools.NewBashTool(30 * time.Second))
	toolReg.Register(&tools.FileReadTool{})
	toolReg.Register(&tools.LoadSkillsTool{})
	toolReg.Register(&tools.SearchSkillsTool{})
	toolReg.Register(tools.NewGWSExecuteTool(tools.GWSToolConfig{
		Interactor:   &autoApproveInteractor{},
		ScopeChecker: &allScopesChecker{},
		Bridge:       bridge,
		ScopeWaiter:  google.NewScopeWaiter(),
		Google:       g,
		Account:      "test@example.com",
		Manifest:     manifest,
		Runner:       runner,
	}))

	identity := "You are a personal AI assistant powered by OpenBotKit.\n"
	blocks := tools.BuildSystemBlocks(identity, toolReg)

	return agent.New(f.Provider, f.Model, toolReg,
		agent.WithSystemBlocks(blocks),
		agent.WithMaxIterations(15),
	)
}

// InstallGWSSkills writes minimal gws skill files for testing.
func (f *LocalFixture) InstallGWSSkills(t *testing.T) {
	t.Helper()

	gwsSkills := map[string]struct{ skill, reference string }{
		"gws-shared": {
			skill: "---\nname: gws-shared\nversion: 1.0.0\ndescription: \"gws CLI: Shared patterns for authentication, global flags, and output formatting.\"\n---\n\nRead REFERENCE.md for full instructions.\n",
			reference: `## CLI Syntax

` + "```bash" + `
gws <service> <resource> [sub-resource] <method> [flags]
` + "```" + `

### Method Flags

| Flag | Description |
|------|-------------|
| ` + "`--params '{\"key\": \"val\"}'`" + ` | URL/query parameters |
| ` + "`--json '{\"key\": \"val\"}'`" + ` | Request body |
| ` + "`--page-all`" + ` | Auto-paginate |
| ` + "`--format <FORMAT>`" + ` | Output format: json (default), table, yaml, csv |
`,
		},
		"gws-docs": {
			skill: "---\nname: gws-docs\nversion: 1.0.0\ndescription: \"Read and write Google Docs.\"\n---\n\nRead REFERENCE.md for full instructions.\n",
			reference: `> **PREREQUISITE:** Read ` + "`../gws-shared/SKILL.md`" + ` for auth, global flags, and security rules.

` + "```bash" + `
gws docs <resource> <method> [flags]
` + "```" + `

## API Resources

### documents

  - ` + "`batchUpdate`" + ` — Applies updates to the document.
  - ` + "`create`" + ` — Creates a blank document using the title given in the request.
  - ` + "`get`" + ` — Gets the latest version of the specified document.

## Discovering Commands

` + "```bash" + `
gws docs --help
gws schema docs.<resource>.<method>
` + "```" + `
`,
		},
		"gws-drive": {
			skill: "---\nname: gws-drive\nversion: 1.0.0\ndescription: \"Google Drive: Manage files, folders, and shared drives.\"\n---\n\nRead REFERENCE.md for full instructions.\n",
			reference: `> **PREREQUISITE:** Read ` + "`../gws-shared/SKILL.md`" + ` for auth, global flags, and security rules.

` + "```bash" + `
gws drive <resource> <method> [flags]
` + "```" + `

## API Resources

### files

  - ` + "`copy`" + ` — Creates a copy of a file.
  - ` + "`create`" + ` — Creates a file.
  - ` + "`get`" + ` — Gets a file's metadata or content by ID.
  - ` + "`list`" + ` — Lists the user's files. Accepts the ` + "`q`" + ` parameter for search queries. Use mimeType filter to find specific file types (e.g. ` + "`mimeType='application/vnd.google-apps.document'`" + ` for Google Docs).

## Discovering Commands

` + "```bash" + `
gws drive --help
gws schema drive.<resource>.<method>
` + "```" + `
`,
		},
		"gws-calendar": {
			skill: "---\nname: gws-calendar\nversion: 1.0.0\ndescription: \"Google Calendar: Manage calendars and events.\"\n---\n\nRead REFERENCE.md for full instructions.\n",
			reference: `> **PREREQUISITE:** Read ` + "`../gws-shared/SKILL.md`" + ` for auth, global flags, and security rules.

` + "```bash" + `
gws calendar <resource> <method> [flags]
` + "```" + `

## API Resources

### events

  - ` + "`list`" + ` — Returns events on the specified calendar.
  - ` + "`get`" + ` — Returns an event based on its Google Calendar ID.
  - ` + "`insert`" + ` — Creates an event.

## Discovering Commands

` + "```bash" + `
gws calendar --help
gws schema calendar.<resource>.<method>
` + "```" + `
`,
		},
	}

	skillsDir := filepath.Join(f.dir, "skills")
	for name, content := range gwsSkills {
		dir := filepath.Join(skillsDir, name)
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content.skill), 0600)
		os.WriteFile(filepath.Join(dir, "REFERENCE.md"), []byte(content.reference), 0600)
	}

	// Regenerate index to include gws skills.
	if err := skills.GenerateIndex(); err != nil {
		t.Fatalf("regenerate skill index: %v", err)
	}
}

type autoApproveInteractor struct{}

func (a *autoApproveInteractor) Notify(_ string) error                  { return nil }
func (a *autoApproveInteractor) NotifyLink(_, _ string) error           { return nil }
func (a *autoApproveInteractor) RequestApproval(_ string) (bool, error) { return true, nil }

type allScopesChecker struct{}

func (a *allScopesChecker) HasScopes(_ string, _ []string) (bool, error) { return true, nil }

const gwsTestCredentials = `{
	"installed": {
		"client_id": "test.apps.googleusercontent.com",
		"client_secret": "secret",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"redirect_uris": ["http://localhost"]
	}
}`

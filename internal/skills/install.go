package skills

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/provider/google"
	embeddedSkills "github.com/priyanshujain/openbotkit/skills"
)

// SkillMeta defines requirements for a built-in skill.
type SkillMeta struct {
	Scopes       []string
	RequiresAuth string
	Write        bool
}

var builtinSkills = map[string]SkillMeta{
	"email-read":      {Scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"}},
	"email-send":      {Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}, Write: true},
	"whatsapp-read":   {RequiresAuth: "whatsapp"},
	"whatsapp-send":   {RequiresAuth: "whatsapp", Write: true},
	"memory-read":     {},
	"applenotes-read": {RequiresAuth: "applenotes"},
}

// InstallResult tracks what changed during installation.
type InstallResult struct {
	Installed []string
	Removed   []string
	Skipped   []string
}

// Install performs declarative skill installation.
// It determines the desired state from current auth, diffs against the
// manifest, and adds/removes skills accordingly.
func Install(cfg *config.Config) (*InstallResult, error) {
	manifest, err := LoadManifest()
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}

	desired := make(map[string]SkillEntry)
	result := &InstallResult{}

	// Determine desired built-in skills.
	grantedGoogle := resolveGoogleScopes(cfg)
	sourceAuthed := resolveSourceAuth(cfg)

	for name, meta := range builtinSkills {
		if !isSkillEligible(meta, grantedGoogle, sourceAuthed) {
			result.Skipped = append(result.Skipped, name)
			continue
		}
		desired[name] = SkillEntry{
			Source:       "obk",
			Version:      "0.1.0",
			Scopes:       meta.Scopes,
			RequiresAuth: meta.RequiresAuth,
			Write:        meta.Write,
		}
	}

	// Determine desired gws skills.
	gwsDesired, gwsSkipped, gwsErr := resolveGWSSkills(cfg)
	if gwsErr != nil {
		fmt.Printf("  warning: gws: %v\n", gwsErr)
		// Keep existing gws skills in manifest.
		for name, entry := range manifest.Skills {
			if entry.Source == "gws" {
				desired[name] = entry
			}
		}
	} else {
		for name, entry := range gwsDesired {
			desired[name] = entry
		}
		result.Skipped = append(result.Skipped, gwsSkipped...)
	}

	// Diff: remove skills not in desired state.
	for name := range manifest.Skills {
		if _, ok := desired[name]; !ok {
			skillDir := filepath.Join(SkillsDir(), name)
			os.RemoveAll(skillDir)
			result.Removed = append(result.Removed, name)
		}
	}

	// Diff: add/update desired skills.
	for name, entry := range desired {
		if err := installSkill(name, entry); err != nil {
			return nil, fmt.Errorf("install skill %s: %w", name, err)
		}
		result.Installed = append(result.Installed, name)
	}

	// Write manifest.
	manifest.Skills = desired
	if err := SaveManifest(manifest); err != nil {
		return nil, fmt.Errorf("save manifest: %w", err)
	}

	return result, nil
}

func installSkill(name string, entry SkillEntry) error {
	destDir := filepath.Join(SkillsDir(), name)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return err
	}

	switch entry.Source {
	case "obk":
		return installBuiltinSkill(name, destDir)
	case "gws":
		// gws skills are already copied from temp dir during resolve.
		return nil
	}
	return nil
}

func installBuiltinSkill(name, destDir string) error {
	content, err := fs.ReadFile(embeddedSkills.FS, name+"/SKILL.md")
	if err != nil {
		return fmt.Errorf("read embedded skill: %w", err)
	}
	return os.WriteFile(filepath.Join(destDir, "SKILL.md"), content, 0600)
}

func resolveGoogleScopes(cfg *config.Config) map[string]bool {
	scopes := make(map[string]bool)
	tokenDB := cfg.GoogleTokenDBPath()
	if _, err := os.Stat(tokenDB); os.IsNotExist(err) {
		return scopes
	}

	store, err := google.NewTokenStore(tokenDB)
	if err != nil {
		return scopes
	}
	defer store.Close()

	accounts, err := store.ListAccounts()
	if err != nil {
		return scopes
	}

	for _, account := range accounts {
		_, granted, err := store.LoadToken(account)
		if err != nil {
			continue
		}
		for _, s := range granted {
			scopes[s] = true
		}
	}
	return scopes
}

// resolveSourceAuth checks which non-Google sources are authenticated.
func resolveSourceAuth(cfg *config.Config) map[string]bool {
	authed := make(map[string]bool)

	// WhatsApp: check if session.db exists and is non-empty.
	sessionDB := cfg.WhatsAppSessionDBPath()
	if info, err := os.Stat(sessionDB); err == nil && info.Size() > 0 {
		authed["whatsapp"] = true
	}

	// Apple Notes: check if linked via config.
	if config.IsSourceLinked("applenotes") {
		authed["applenotes"] = true
	}

	return authed
}

func isSkillEligible(meta SkillMeta, grantedGoogle map[string]bool, sourceAuthed map[string]bool) bool {
	// No requirements — always eligible.
	if len(meta.Scopes) == 0 && meta.RequiresAuth == "" {
		return true
	}

	if meta.RequiresAuth != "" {
		return sourceAuthed[meta.RequiresAuth]
	}

	// Check Google scopes. gmail.modify implies gmail.readonly.
	for _, required := range meta.Scopes {
		if !grantedGoogle[required] {
			if required == "https://www.googleapis.com/auth/gmail.readonly" && grantedGoogle["https://www.googleapis.com/auth/gmail.modify"] {
				continue
			}
			return false
		}
	}
	return true
}

// resolveGWSSkills determines which gws skills should be installed.
func resolveGWSSkills(cfg *config.Config) (map[string]SkillEntry, []string, error) {
	if cfg.Integrations == nil || cfg.Integrations.GWS == nil || !cfg.Integrations.GWS.Enabled {
		return nil, nil, nil
	}

	gwsPath, err := exec.LookPath("gws")
	if err != nil {
		return nil, nil, fmt.Errorf("gws not found on PATH — skipping gws skill update")
	}

	// Get actual granted scopes from gws.
	scopeMap, err := gwsAuthStatus(gwsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("gws auth status: %w", err)
	}

	desired := make(map[string]SkillEntry)
	var skipped []string

	// Always generate shared skills when gws is active.
	services := make([]string, len(cfg.Integrations.GWS.Services)+1)
	copy(services, cfg.Integrations.GWS.Services)
	services[len(services)-1] = "shared"

	tmpDir, err := os.MkdirTemp("", "gws-skills-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, svc := range services {
		if err := gwsGenerateSkills(gwsPath, tmpDir, svc); err != nil {
			fmt.Printf("  warning: gws generate-skills --filter %s: %v\n", svc, err)
			continue
		}
	}

	// Scan generated skills.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read generated skills: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		skillPath := filepath.Join(tmpDir, name, "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		isWrite := strings.Contains(string(content), "[!CAUTION]")
		svc := gwsServiceFromSkillName(name)
		accessLevel := scopeMap[svc]

		// Skip write skills for read-only services.
		if isWrite && accessLevel == "readonly" {
			skipped = append(skipped, name+" ("+svc+" is read-only)")
			continue
		}

		se := SkillEntry{
			Source:  "gws",
			Version: gwsVersion(gwsPath),
			Write:   isWrite,
		}
		if svc != "shared" {
			se.Scopes = []string{svc}
		}
		desired[name] = se

		// Copy to skills dir.
		destDir := filepath.Join(SkillsDir(), name)
		if err := os.MkdirAll(destDir, 0700); err != nil {
			return nil, nil, fmt.Errorf("create skill dir %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(destDir, "SKILL.md"), content, 0600); err != nil {
			return nil, nil, fmt.Errorf("write skill %s: %w", name, err)
		}
	}

	return desired, skipped, nil
}

// scopeMapping maps Google API scope suffixes to service names.
// Order matters: .readonly must come before the broader scope so that
// if only .readonly is present we don't misclassify as readwrite.
var scopeMapping = []struct {
	suffix  string
	service string
	access  string
}{
	{"googleapis.com/auth/calendar.readonly", "calendar", "readonly"},
	{"googleapis.com/auth/calendar", "calendar", "readwrite"},
	{"googleapis.com/auth/drive.readonly", "drive", "readonly"},
	{"googleapis.com/auth/drive", "drive", "readwrite"},
	{"googleapis.com/auth/tasks.readonly", "tasks", "readonly"},
	{"googleapis.com/auth/tasks", "tasks", "readwrite"},
	{"googleapis.com/auth/documents.readonly", "docs", "readonly"},
	{"googleapis.com/auth/documents", "docs", "readwrite"},
	{"googleapis.com/auth/spreadsheets.readonly", "sheets", "readonly"},
	{"googleapis.com/auth/spreadsheets", "sheets", "readwrite"},
	{"googleapis.com/auth/contacts.readonly", "people", "readonly"},
	{"googleapis.com/auth/contacts", "people", "readwrite"},
}

func gwsAuthStatus(gwsPath string) (map[string]string, error) {
	cmd := exec.Command(gwsPath, "auth", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run gws auth status: %w", err)
	}
	return parseScopeMap(string(out)), nil
}

// parseScopeMap extracts service:access pairs from gws auth status output.
// Handles both one-scope-per-line and comma/space-separated formats.
func parseScopeMap(output string) map[string]string {
	scopeMap := make(map[string]string)
	for _, m := range scopeMapping {
		if strings.Contains(output, m.suffix) {
			// Only set if not already set (readonly is checked before
			// readwrite, so if .readonly matched, don't overwrite).
			if _, exists := scopeMap[m.service]; !exists {
				scopeMap[m.service] = m.access
			}
		}
	}
	return scopeMap
}

func gwsGenerateSkills(gwsPath, outputDir, filter string) error {
	cmd := exec.Command(gwsPath, "generate-skills", "--output-dir", outputDir, "--filter", filter)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gwsVersion(gwsPath string) string {
	cmd := exec.Command(gwsPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func gwsServiceFromSkillName(name string) string {
	// gws skill names follow pattern: gws-<service> or gws-<service>-<helper>
	name = strings.TrimPrefix(name, "gws-")
	parts := strings.SplitN(name, "-", 2)
	return parts[0]
}

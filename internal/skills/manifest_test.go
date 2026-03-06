package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("OBK_CONFIG_DIR", tmp)
	defer os.Unsetenv("OBK_CONFIG_DIR")

	original := &Manifest{
		Skills: map[string]SkillEntry{
			"email-read": {
				Source:  "obk",
				Version: "0.1.0",
				Scopes:  []string{"gmail.readonly"},
			},
			"gws-calendar": {
				Source:  "gws",
				Version: "0.7.0",
				Scopes:  []string{"calendar"},
				Write:   true,
			},
		},
	}

	if err := SaveManifest(original); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(tmp, "skills", "manifest.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest file not created: %v", err)
	}

	loaded, err := LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if len(loaded.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(loaded.Skills))
	}

	email := loaded.Skills["email-read"]
	if email.Source != "obk" || email.Version != "0.1.0" {
		t.Errorf("email-read: got source=%q version=%q", email.Source, email.Version)
	}

	cal := loaded.Skills["gws-calendar"]
	if cal.Source != "gws" || !cal.Write {
		t.Errorf("gws-calendar: got source=%q write=%v", cal.Source, cal.Write)
	}
}

func TestLoadManifestMissing(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("OBK_CONFIG_DIR", tmp)
	defer os.Unsetenv("OBK_CONFIG_DIR")

	m, err := LoadManifest()
	if err != nil {
		t.Fatalf("load missing manifest: %v", err)
	}
	if m.Skills == nil {
		t.Fatal("Skills map should be initialized, got nil")
	}
	if len(m.Skills) != 0 {
		t.Fatalf("got %d skills, want 0", len(m.Skills))
	}
}

func TestLoadManifestEmpty(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("OBK_CONFIG_DIR", tmp)
	defer os.Unsetenv("OBK_CONFIG_DIR")

	// Write an empty YAML file.
	dir := filepath.Join(tmp, "skills")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("skills:\n"), 0600)

	m, err := LoadManifest()
	if err != nil {
		t.Fatalf("load empty manifest: %v", err)
	}
	if m.Skills == nil {
		t.Fatal("Skills map should be initialized, got nil")
	}
}

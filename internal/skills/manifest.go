package skills

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/priyanshujain/openbotkit/config"
	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Skills map[string]SkillEntry `yaml:"skills"`
}

type SkillEntry struct {
	Source       string   `yaml:"source"`
	Version      string   `yaml:"version"`
	Scopes       []string `yaml:"scopes,omitempty"`
	RequiresAuth string   `yaml:"requires_auth,omitempty"`
	Write        bool     `yaml:"write,omitempty"`
}

func ManifestPath() string {
	return filepath.Join(config.Dir(), "skills", "manifest.yaml")
}

func SkillsDir() string {
	return filepath.Join(config.Dir(), "skills")
}

func LoadManifest() (*Manifest, error) {
	data, err := os.ReadFile(ManifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Skills: make(map[string]SkillEntry)}, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Skills == nil {
		m.Skills = make(map[string]SkillEntry)
	}
	return &m, nil
}

func SaveManifest(m *Manifest) error {
	dir := filepath.Dir(ManifestPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(ManifestPath(), data, 0600)
}

package backup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Manifest struct {
	Version   int                     `json:"version"`
	ID        string                  `json:"id"`
	Timestamp time.Time               `json:"timestamp"`
	Hostname  string                  `json:"hostname"`
	Files     map[string]ManifestFile `json:"files"`
}

type ManifestFile struct {
	Hash           string `json:"hash"`
	Size           int64  `json:"size"`
	CompressedSize int64  `json:"compressed_size"`
}

func NewManifest(hostname string) *Manifest {
	now := time.Now().UTC()
	suffix := make([]byte, 4)
	rand.Read(suffix)
	return &Manifest{
		Version:   1,
		ID:        now.Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix),
		Timestamp: now,
		Hostname:  hostname,
		Files:     make(map[string]ManifestFile),
	}
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Files: make(map[string]ManifestFile)}, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Files == nil {
		m.Files = make(map[string]ManifestFile)
	}
	return &m, nil
}

func SaveManifest(path string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

type DiffResult struct {
	Changed []string // files that are new or have different hashes
	Removed []string // files in old manifest but not in new scan
}

func DiffManifest(old *Manifest, current map[string]string) DiffResult {
	var result DiffResult

	for path, hash := range current {
		prev, ok := old.Files[path]
		if !ok || prev.Hash != hash {
			result.Changed = append(result.Changed, path)
		}
	}

	for path := range old.Files {
		if _, ok := current[path]; !ok {
			result.Removed = append(result.Removed, path)
		}
	}

	return result
}

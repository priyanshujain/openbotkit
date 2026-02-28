package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultDirName = ".obk"
	ConfigFileName = "config.yaml"
)

// Dir returns the obk config directory.
// Checks OBK_CONFIG_DIR env var first, then falls back to ~/.obk/.
func Dir() string {
	if d := os.Getenv("OBK_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultDirName
	}
	return filepath.Join(home, DefaultDirName)
}

func FilePath() string {
	return filepath.Join(Dir(), ConfigFileName)
}

func SourceDir(sourceName string) string {
	return filepath.Join(Dir(), sourceName)
}

func EnsureDir() error {
	return os.MkdirAll(Dir(), 0700)
}

func EnsureSourceDir(sourceName string) error {
	return os.MkdirAll(SourceDir(sourceName), 0700)
}

func JobsDBPath() string {
	return filepath.Join(Dir(), "jobs.db")
}

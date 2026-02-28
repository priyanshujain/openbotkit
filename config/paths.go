package config

import (
	"os"
	"path/filepath"
)

const (
	// DefaultDirName is the directory name under the user's home directory.
	DefaultDirName = ".obk"
	// ConfigFileName is the config file name.
	ConfigFileName = "config.yaml"
)

// Dir returns the obk configuration directory.
// It checks OBK_CONFIG_DIR first, then falls back to ~/.obk/.
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

// FilePath returns the full path to the config file.
func FilePath() string {
	return filepath.Join(Dir(), ConfigFileName)
}

// SourceDir returns the data directory for a given source (e.g. ~/.obk/gmail/).
func SourceDir(sourceName string) string {
	return filepath.Join(Dir(), sourceName)
}

// EnsureDir creates the obk directory if it doesn't exist.
func EnsureDir() error {
	return os.MkdirAll(Dir(), 0700)
}

// EnsureSourceDir creates the data directory for a source.
func EnsureSourceDir(sourceName string) error {
	return os.MkdirAll(SourceDir(sourceName), 0700)
}

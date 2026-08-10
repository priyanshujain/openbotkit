package backup

import (
	"io/fs"
	"path/filepath"
	"strings"
)

var includePatterns = []string{
	"*/data.db",
	"whatsapp/session.db",
	"telegram/session.json",
	"config.yaml",
	"env",
	"ngrok.yml",
	"providers/**/*.json",
	"models/*.json",
	"applenotes/config.json",
	"applecontacts/config.json",
	"learnings/**/*.md",
	"skills/**",
}

var excludePatterns = []string{
	"*.db-wal",
	"*.db-shm",
	"daemon.log",
	"server.log",
	"bin/",
	"scratch/",
	"jobs.db",
	"*.lock",
	"backup/",
}

func ScanFiles(baseDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if isExcluded(rel) {
			return nil
		}
		if isIncluded(rel) {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func isIncluded(rel string) bool {
	for _, pattern := range includePatterns {
		if matchGlob(pattern, rel) {
			return true
		}
	}
	return false
}

func isExcluded(rel string) bool {
	for _, pattern := range excludePatterns {
		name := filepath.Base(rel)
		if matchGlob(pattern, rel) || matchGlob(pattern, name) {
			return true
		}
		if strings.HasSuffix(pattern, "/") {
			dir := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(rel, dir+"/") || rel == dir {
				return true
			}
		}
	}
	return false
}

func matchGlob(pattern, name string) bool {
	if strings.Contains(pattern, "**") {
		prefix := strings.Split(pattern, "**")[0]
		suffix := strings.Split(pattern, "**")[1]
		suffix = strings.TrimPrefix(suffix, "/")

		if prefix != "" && !strings.HasPrefix(name, prefix) {
			return false
		}
		if suffix == "" {
			return strings.HasPrefix(name, prefix)
		}
		rest := strings.TrimPrefix(name, prefix)
		parts := strings.Split(rest, "/")
		for i := range parts {
			tail := strings.Join(parts[i:], "/")
			if matched, _ := filepath.Match(suffix, tail); matched {
				return true
			}
		}
		return false
	}
	if strings.Contains(pattern, "/") {
		matched, _ := filepath.Match(pattern, name)
		return matched
	}
	matched, _ := filepath.Match(pattern, filepath.Base(name))
	return matched
}

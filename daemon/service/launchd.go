package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	launchdLabel = "com.openbotkit.obk"
	plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>`
)

type launchdManager struct{}

func (m *launchdManager) plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func (m *launchdManager) Install(cfg *ServiceConfig) error {
	path, err := m.plistPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	content := fmt.Sprintf(plistTemplate, launchdLabel, cfg.BinaryPath, cfg.LogPath, cfg.LogPath)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	if err := exec.Command("launchctl", "load", path).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}

	return nil
}

func (m *launchdManager) Uninstall() error {
	path, err := m.plistPath()
	if err != nil {
		return err
	}

	_ = exec.Command("launchctl", "unload", path).Run()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	return nil
}

func (m *launchdManager) Status() (string, error) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return "unknown", fmt.Errorf("launchctl list: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, launchdLabel) {
			return "running", nil
		}
	}

	return "not running", nil
}

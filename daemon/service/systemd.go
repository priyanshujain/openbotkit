package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const unitTemplate = `[Unit]
Description=OpenBotKit Daemon
After=network.target

[Service]
Type=simple
ExecStart=%s daemon
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

type systemdManager struct{}

func (m *systemdManager) unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", "obk.service"), nil
}

func (m *systemdManager) Install(cfg *ServiceConfig) error {
	path, err := m.unitPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	content := fmt.Sprintf(unitTemplate, cfg.BinaryPath)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "enable", "--now", "obk").Run(); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	return nil
}

func (m *systemdManager) Uninstall() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", "obk").Run()

	path, err := m.unitPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func (m *systemdManager) Status() (string, error) {
	out, err := exec.Command("systemctl", "--user", "is-active", "obk").Output()
	if err != nil {
		return "not running", nil
	}
	return strings.TrimSpace(string(out)), nil
}

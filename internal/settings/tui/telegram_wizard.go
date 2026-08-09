package tui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/settings"
)

// telegramLoginDoneMsg carries the result of the external login command.
type telegramLoginDoneMsg struct {
	err error
}

// enterTelegramLogin hands the terminal to `obk telegram auth login`. The QR
// flow lives in the browser and needs the terminal for its prompts, so running
// the real command is both simpler and better tested than reimplementing it
// inside the settings TUI.
func (m model) enterTelegramLogin() (tea.Model, tea.Cmd) {
	if !settings.IsTelegramConfigured(m.svc.Config()) {
		m.flash = "Set api_id and api_hash first"
		m.viewport.SetContent(m.renderTree())
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return flashMsg{} })
	}

	self, err := os.Executable()
	if err != nil {
		self = "obk"
	}

	cmd := exec.Command(self, "telegram", "auth", "login")
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return telegramLoginDoneMsg{err: err}
	})
}

func (m model) handleTelegramLoginResult(msg telegramLoginDoneMsg) (tea.Model, tea.Cmd) {
	flash := "Connected to Telegram!"
	if msg.err != nil {
		flash = fmt.Sprintf("Telegram login failed: %v", msg.err)
	} else if err := config.LinkSource("telegram"); err != nil {
		flash = fmt.Sprintf("Link source failed: %v", err)
	}

	m.svc.RebuildTree()
	m.rebuildRows()
	m.flash = flash
	m.viewport.SetContent(m.renderTree())
	return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return flashMsg{} })
}

package tui

import (
	"fmt"
	"time"

	"github.com/73ai/openbotkit/agent/audit"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/browser/cookies"
	xclient "github.com/73ai/openbotkit/source/twitter/client"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// --- X login wizard: browser select → spinner → save ---

func (m model) enterXLogin() (model, tea.Cmd) {
	m.state = stateXAuth
	m.wizardError = ""

	available := cookies.AvailableBrowsers()
	var opts []huh.Option[string]
	for _, b := range available {
		label := b
		if b == "Safari" {
			label = "Safari (requires Full Disk Access for terminal)"
		}
		opts = append(opts, huh.NewOption(label, b))
	}

	selected := ""
	m.wizardXBrowserSel = &selected

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which browser are you signed in to x.com on?").
				Options(opts...).
				Value(m.wizardXBrowserSel),
		),
	)
	return m, m.form.Init()
}

func (m model) updateXAuth(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m.exitXWizard("")
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		browser := *m.wizardXBrowserSel

		m.state = stateVerifying
		m.wizardXBrowser = true
		m.wizardError = ""
		return m, tea.Batch(
			m.wizardSpinner.Tick,
			xExtractCookiesCmd(browser),
		)
	}

	return m, cmd
}

func (m model) handleXLoginResult(msg xAuthResultMsg) (model, tea.Cmd) {
	if msg.err != nil {
		browser := ""
		if m.wizardXBrowserSel != nil {
			browser = *m.wizardXBrowserSel
		}

		errMsg := fmt.Sprintf("Failed: %v", msg.err)
		if browser == "Safari" && cookies.IsPermissionError(msg.err) {
			errMsg = "Safari access denied. Grant Full Disk Access to your terminal:\n" +
				"  System Settings > Privacy & Security > Full Disk Access"
		} else {
			errMsg += "\nMake sure you're signed in to x.com in that browser."
		}
		return m.exitXWizard(errMsg)
	}

	if msg.session == nil {
		return m.exitXWizard("Cookie extraction failed: no session returned")
	}

	if err := xclient.SaveSession(msg.session); err != nil {
		return m.exitXWizard(fmt.Sprintf("Save credentials failed: %v", err))
	}

	cfg := m.svc.Config()
	if cfg.X == nil {
		cfg.X = &config.XConfig{}
	}

	if err := m.svc.Save(); err != nil {
		return m.exitXWizard(fmt.Sprintf("Save config failed: %v", err))
	}

	if err := config.LinkSource("x"); err != nil {
		return m.exitXWizard(fmt.Sprintf("Link source failed: %v", err))
	}

	browser := ""
	if msg.browser != "" {
		browser = " (from " + msg.browser + ")"
	}

	return m.exitXWizard(fmt.Sprintf("Connected to X%s!", browser))
}

func (m model) exitXWizard(flash string) (model, tea.Cmd) {
	m.state = stateBrowse
	m.form = nil
	m.wizardXBrowser = false
	m.wizardXBrowserSel = nil
	m.wizardError = ""
	m.svc.RebuildTree()
	m.rebuildRows()
	m.viewport.SetContent(m.renderTree())

	if flash != "" {
		m.flash = flash
		m.viewport.SetContent(m.renderTree())
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return flashMsg{}
		})
	}
	return m, nil
}

func logXAudit(toolName, input, output, errMsg string) {
	audit.LogQuick(config.AuditJSONLPath(), "settings", toolName, input, output, errMsg)
}

func xExtractCookiesCmd(browser string) tea.Cmd {
	return func() tea.Msg {
		logXAudit("x.auth.login_browser", "browser="+browser, "extracting cookies", "")

		session, b, err := xclient.ExtractSessionFromBrowserByName(browser)
		if err != nil {
			logXAudit("x.auth.login_browser", "browser="+browser, "", err.Error())
			return xAuthResultMsg{err: err}
		}

		logXAudit("x.auth.login_browser", "browser="+browser, "login successful ("+b+")", "")
		return xAuthResultMsg{session: session, browser: b}
	}
}

package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	"github.com/73ai/openbotkit/settings"
	xclient "github.com/73ai/openbotkit/source/twitter/client"
)

type state int

const (
	stateBrowse        state = iota
	stateEdit
	stateProfileSelect
	stateProviderAuth
	stateVerifying
	stateModelSelect
	stateBackupDest
	stateBackupCreds
	stateXAuth
	stateXTFA
)

type flashMsg struct{}

// verifyResultMsg is returned by the async provider verification command.
type verifyResultMsg struct {
	provider string
	err      error
}

// verifyModelResultMsg is returned by async per-model verification.
type verifyModelResultMsg struct {
	tier  string
	model string
	err   error
}

// backupVerifyResultMsg is returned by async backup verification.
type backupVerifyResultMsg struct {
	folderID string // set for GDrive setup
	err      error
}

// backupTriggeredMsg is returned after an async backup trigger attempt.
type backupTriggeredMsg struct {
	err error
}

// xAuthResultMsg is returned by async X cookie extraction.
type xAuthResultMsg struct {
	session *xclient.Session
	browser string
	err     error
}

// modelsLoadedMsg is returned when background model loading completes.
type modelsLoadedMsg struct {
	provider string
	models   []provider.AvailableModel
	err      error
}

type model struct {
	svc      *settings.Service
	rows     []row
	expanded map[string]bool
	cursor   int
	state    state
	form     *huh.Form
	editField *settings.Field
	editStr  *string
	editBool *bool
	flash    string
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	// Wizard state
	wizardProfile   *string
	wizardProviders []string
	wizardProvIdx   int
	wizardTierIdx   int
	wizardTiers     [4]string // default, complex, fast, nano
	wizardSpinner   spinner.Model
	wizardAPIKey    *string
	wizardError     string

	// Backup wizard state
	wizardBackupDest     *string
	wizardBackupBucket   *string
	wizardBackupEndpoint *string
	wizardBackupAK       *string
	wizardBackupSK       *string
	wizardBackupSnapshot *config.BackupConfig // for transactional rollback

	// X login wizard state
	wizardXBrowser    bool
	wizardXBrowserSel *string
}

func newModel(svc *settings.Service) model {
	expanded := make(map[string]bool)
	rows := flattenTree(svc.Tree(), expanded)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	return model{
		svc:           svc,
		rows:          rows,
		expanded:      expanded,
		wizardSpinner: s,
	}
}

func (m model) Init() tea.Cmd {
	// Pre-warm model cache for configured providers.
	var cmds []tea.Cmd
	for _, provName := range configuredProviderNames(m.svc.Config()) {
		name := provName
		cmds = append(cmds, loadModelsInBackgroundCmd(name, m.svc))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateEdit:
		return m.updateEdit(msg)
	case stateProfileSelect:
		return m.updateProfileSelect(msg)
	case stateProviderAuth:
		return m.updateProviderAuth(msg)
	case stateVerifying:
		return m.updateVerifying(msg)
	case stateModelSelect:
		return m.updateModelSelect(msg)
	case stateBackupDest:
		return m.updateBackupDest(msg)
	case stateBackupCreds:
		return m.updateBackupCreds(msg)
	case stateXAuth:
		return m.updateXAuth(msg)
	default:
		return m.updateBrowse(msg)
	}
}

// --- Browse state ---

func (m model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.viewport = viewport.New(msg.Width, msg.Height-4)
		m.viewport.SetContent(m.renderTree())
		return m, nil

	case flashMsg:
		m.flash = ""
		m.viewport.SetContent(m.renderTree())
		return m, nil

	case telegramLoginDoneMsg:
		return m.handleTelegramLoginResult(msg)

	case backupTriggeredMsg:
		if msg.err != nil {
			m.flash = fmt.Sprintf("Backup failed: %v", msg.err)
		} else {
			m.flash = "Backup started"
		}
		m.viewport.SetContent(m.renderTree())
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return flashMsg{}
		})

	case modelsLoadedMsg:
		// Silently update cache, no UI change.
		if msg.err == nil && len(msg.models) > 0 {
			cache := provider.NewModelCache(config.ModelsDir())
			list := &provider.CachedModelList{
				Provider:  msg.provider,
				Models:    msg.models,
				FetchedAt: time.Now(),
			}
			if existing, err := cache.Load(msg.provider); err == nil && existing.VerifiedModels != nil {
				list.VerifiedModels = existing.VerifiedModels
			}
			_ = cache.Save(msg.provider, list)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			m.viewport.SetContent(m.renderTree())
			m.ensureCursorVisible()
			return m, nil
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			m.viewport.SetContent(m.renderTree())
			m.ensureCursorVisible()
			return m, nil
		case "enter":
			return m.handleEnter()
		}
	}
	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.rows) {
		return m, nil
	}
	r := m.rows[m.cursor]

	if r.node.Category != nil {
		key := r.node.Category.Key
		m.expanded[key] = !m.expanded[key]
		m.rebuildRows()
		m.viewport.SetContent(m.renderTree())
		return m, nil
	}

	if r.node.Field != nil {
		f := r.node.Field

		// X login wizard
		if f.Key == "x.auth_status" {
			return m.enterXLogin()
		}

		// Telegram login runs the QR flow in a browser, so hand over the terminal.
		if f.Key == "telegram.auth_status" {
			return m.enterTelegramLogin()
		}

		// Backup wizard — only when not yet configured.
		if f.Key == "backup.enabled" && f.ReadOnly != nil && f.ReadOnly(m.svc.Config()) {
			return m.enterBackupWizard()
		}

		// Destination change — transactional flow.
		if f.Key == "backup.destination" {
			return m.enterDestinationChange()
		}

		if f.ReadOnly != nil && f.ReadOnly(m.svc.Config()) {
			m.flash = "Locked — configure prerequisites first"
			m.viewport.SetContent(m.renderTree())
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return flashMsg{}
			})
		}

		// Profile field → enter profile wizard inline.
		if f.Key == "models.profile" {
			return m.enterProfileSelect()
		}

		current := m.svc.GetValue(f)
		form, strVal, boolVal := buildForm(f, current, m.svc)
		m.state = stateEdit
		m.form = form
		m.editField = f
		m.editStr = strVal
		m.editBool = boolVal
		return m, m.form.Init()
	}

	return m, nil
}

// --- Edit state (simple field edit) ---

func (m model) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = stateBrowse
			m.form = nil
			m.editField = nil
			m.viewport.SetContent(m.renderTree())
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		var value string
		if m.editBool != nil {
			value = strconv.FormatBool(*m.editBool)
		} else if m.editStr != nil {
			value = *m.editStr
		}

		if err := m.svc.SetValue(m.editField, value); err != nil {
			m.flash = fmt.Sprintf("Error: %v", err)
		} else {
			m.flash = "Saved!"
			if m.editField.AfterSet != nil {
				if msg := m.editField.AfterSet(m.svc); msg != "" {
					m.flash = "Saved! " + msg
				}
			}
		}

		// Rebuild tree when backup config changes (shows/hides R2 category).
		if strings.HasPrefix(m.editField.Key, "backup.") {
			m.svc.RebuildTree()
		}

		// Trigger backup when re-enabling (false → true).
		var triggerCmd tea.Cmd
		if m.editField.Key == "backup.enabled" && value == "true" {
			triggerCmd = triggerBackupIfStaleCmd(m.svc)
		}

		m.state = stateBrowse
		m.form = nil
		m.editField = nil
		m.rebuildRows()
		m.viewport.SetContent(m.renderTree())
		return m, tea.Batch(
			tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return flashMsg{}
			}),
			triggerCmd,
		)
	}

	return m, cmd
}

// --- Profile Select state ---

func (m model) enterProfileSelect() (model, tea.Cmd) {
	m.state = stateProfileSelect
	m.wizardError = ""

	// Build profile options.
	var opts []huh.Option[string]
	for _, name := range config.ProfileNames {
		p := config.Profiles[name]
		opts = append(opts, huh.NewOption(p.Label, name))
	}
	opts = append(opts, huh.NewOption("Custom (choose models manually)", "custom"))

	// Allocate on heap so pointer survives bubbletea value copies.
	selected := ""
	cfg := m.svc.Config()
	if cfg.Models != nil && cfg.Models.Profile != "" {
		selected = cfg.Models.Profile
	}
	m.wizardProfile = &selected

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How would you like to configure models?").
				DescriptionFunc(func() string {
					return settings.ProfilePreview(*m.wizardProfile)
				}, m.wizardProfile).
				Options(opts...).
				Value(m.wizardProfile),
		),
	)
	return m, m.form.Init()
}

func (m model) updateProfileSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = stateBrowse
			m.form = nil
			m.viewport.SetContent(m.renderTree())
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if *m.wizardProfile == "custom" {
			return m.enterCustomModelSelect()
		}
		return m.enterFixedProfileAuth()
	}

	return m, cmd
}

// --- Fixed profile: auth providers, then verify ---

func (m model) enterFixedProfileAuth() (model, tea.Cmd) {
	p := config.Profiles[*m.wizardProfile]
	m.wizardProviders = p.Providers

	// Find the first provider that needs auth.
	m.wizardProvIdx = 0
	return m.nextProviderAuth()
}

func (m model) nextProviderAuth() (model, tea.Cmd) {
	cfg := m.svc.Config()
	for m.wizardProvIdx < len(m.wizardProviders) {
		provName := m.wizardProviders[m.wizardProvIdx]
		pcfg := providerConfig(cfg, provName)
		if pcfg.APIKeyRef != "" || pcfg.AuthMethod == "vertex_ai" {
			m.wizardProvIdx++
			continue
		}
		return m.enterProviderAuth(provName)
	}
	// All providers authed — verify.
	return m.enterVerifyProviders()
}

func (m model) enterProviderAuth(provName string) (model, tea.Cmd) {
	m.state = stateProviderAuth
	m.wizardError = ""

	apiKey := ""
	m.wizardAPIKey = &apiKey

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter " + provName + " API key").
				EchoMode(huh.EchoModePassword).
				Value(m.wizardAPIKey),
		),
	)
	return m, m.form.Init()
}

func (m model) updateProviderAuth(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = stateBrowse
			m.form = nil
			m.viewport.SetContent(m.renderTree())
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		provName := m.wizardProviders[m.wizardProvIdx]

		if *m.wizardAPIKey == "" {
			m.wizardError = provName + " API key is required"
			return m.enterProviderAuth(provName)
		}

		// Store credential.
		ref := fmt.Sprintf("keychain:obk/%s", provName)
		if err := m.svc.StoreCredential(ref, *m.wizardAPIKey); err != nil {
			m.wizardError = fmt.Sprintf("store credential: %v", err)
			return m.enterProviderAuth(provName)
		}

		// Update config.
		settings.EnsureModels(m.svc.Config())
		cfg := m.svc.Config()
		if cfg.Models.Providers == nil {
			cfg.Models.Providers = make(map[string]config.ModelProviderConfig)
		}
		pc := cfg.Models.Providers[provName]
		pc.APIKeyRef = ref
		cfg.Models.Providers[provName] = pc

		// Verify this provider's key via ListModels.
		m.state = stateVerifying
		m.wizardError = ""
		return m, tea.Batch(
			m.wizardSpinner.Tick,
			verifyProviderCmd(m.svc, provName, *m.wizardAPIKey),
		)
	}

	return m, cmd
}

// --- Verify state ---

func (m model) enterVerifyProviders() (model, tea.Cmd) {
	m.state = stateVerifying
	m.wizardProvIdx = 0
	m.wizardError = ""

	// Find the first provider that needs verification.
	return m.verifyNextProvider()
}

func (m model) verifyNextProvider() (model, tea.Cmd) {
	cfg := m.svc.Config()
	for m.wizardProvIdx < len(m.wizardProviders) {
		provName := m.wizardProviders[m.wizardProvIdx]
		pcfg := providerConfig(cfg, provName)

		var apiKey string
		if pcfg.AuthMethod != "vertex_ai" {
			key, err := m.svc.LoadCredential(pcfg.APIKeyRef)
			if err != nil || key == "" {
				m.wizardProvIdx++
				continue
			}
			apiKey = key
		}

		return m, tea.Batch(
			m.wizardSpinner.Tick,
			verifyProviderCmd(m.svc, provName, apiKey),
		)
	}

	// All providers verified — save profile.
	return m.saveProfile()
}

func (m model) updateVerifying(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.wizardSpinner, cmd = m.wizardSpinner.Update(msg)
		return m, cmd

	case backupVerifyResultMsg:
		if msg.err != nil {
			m.flash = fmt.Sprintf("Backup verification failed: %v", msg.err)
			ret, _ := m.rollbackBackup()
			return ret, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return flashMsg{}
			})
		}
		if msg.folderID != "" {
			cfg := m.svc.Config()
			ensureBackup(cfg)
			if cfg.Backup.GDrive == nil {
				cfg.Backup.GDrive = &config.GDriveConfig{}
			}
			cfg.Backup.GDrive.FolderID = msg.folderID
		}
		return m.saveBackup()

	case xAuthResultMsg:
		return m.handleXLoginResult(msg)

	case verifyResultMsg:
		if msg.err != nil {
			m.wizardError = fmt.Sprintf("%s verification failed: %v", msg.provider, msg.err)
			m.state = stateBrowse
			m.flash = m.wizardError
			m.form = nil
			m.rebuildRows()
			m.viewport.SetContent(m.renderTree())
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return flashMsg{}
			})
		}

		m.wizardProvIdx++
		return m.verifyNextProvider()

	case tea.KeyMsg:
		if msg.String() == "esc" {
			if m.wizardBackupSnapshot != nil {
				return m.rollbackBackup()
			}
			m.state = stateBrowse
			m.form = nil
			m.viewport.SetContent(m.renderTree())
			return m, nil
		}
	}
	return m, nil
}

func (m model) saveProfile() (model, tea.Cmd) {
	cfg := m.svc.Config()
	settings.EnsureModels(cfg)

	if *m.wizardProfile == "custom" {
		cfg.Models.Profile = ""
		cfg.Models.Default = m.wizardTiers[0]
		cfg.Models.Complex = m.wizardTiers[1]
		cfg.Models.Fast = m.wizardTiers[2]
		cfg.Models.Nano = m.wizardTiers[3]
	} else {
		p := config.Profiles[*m.wizardProfile]
		cfg.Models.Profile = *m.wizardProfile
		cfg.Models.Default = p.Tiers.Default
		cfg.Models.Complex = p.Tiers.Complex
		cfg.Models.Fast = p.Tiers.Fast
		cfg.Models.Nano = p.Tiers.Nano
	}

	if err := m.svc.Save(); err != nil {
		m.flash = fmt.Sprintf("Error saving: %v", err)
	} else {
		m.flash = "Profile saved!"
	}

	m.state = stateBrowse
	m.form = nil
	m.svc.RebuildTree()
	m.rebuildRows()
	m.viewport.SetContent(m.renderTree())
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return flashMsg{}
	})
}

// --- Custom profile: model selection ---

func (m model) enterCustomModelSelect() (model, tea.Cmd) {
	cfg := m.svc.Config()
	configured := configuredProviderNames(cfg)

	custom := "custom"
	if len(configured) == 0 {
		m.wizardProviders = []string{"anthropic", "openai", "gemini", "groq", "openrouter"}
		m.wizardProvIdx = 0
		m.wizardProfile = &custom
		return m.enterProviderAuth(m.wizardProviders[0])
	}

	m.wizardProviders = configured
	m.wizardTierIdx = 0
	m.wizardProfile = &custom
	return m.nextTierSelect()
}

func (m model) nextTierSelect() (model, tea.Cmd) {
	tierNames := [4]string{"Default model", "Complex model", "Fast model", "Nano model"}

	if m.wizardTierIdx >= 4 {
		// All tiers selected — verify providers.
		needed := make(map[string]bool)
		for _, spec := range m.wizardTiers {
			if spec == "" {
				continue
			}
			parts := strings.SplitN(spec, "/", 2)
			if len(parts) >= 1 {
				needed[parts[0]] = true
			}
		}
		var providers []string
		for p := range needed {
			providers = append(providers, p)
		}
		m.wizardProviders = providers
		m.wizardProvIdx = 0
		return m.enterVerifyProviders()
	}

	m.state = stateModelSelect

	// Build options from cache.
	cache := provider.NewModelCache(config.ModelsDir())
	var opts []huh.Option[string]
	opts = append(opts, huh.NewOption("(none)", ""))
	for _, provName := range m.wizardProviders {
		list, err := cache.Load(provName)
		if err != nil || len(list.Models) == 0 {
			continue
		}
		for _, mod := range list.Models {
			spec := provName + "/" + mod.ID
			label := mod.DisplayName
			if label == "" {
				label = mod.ID
			}
			opts = append(opts, huh.NewOption(label+" ("+provName+")", spec))
		}
	}

	selected := m.wizardTiers[m.wizardTierIdx]
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(tierNames[m.wizardTierIdx]).
				Options(opts...).
				Value(&selected),
		),
	)
	m.editStr = &selected
	return m, m.form.Init()
}

func (m model) updateModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = stateBrowse
			m.form = nil
			m.viewport.SetContent(m.renderTree())
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if m.editStr != nil {
			m.wizardTiers[m.wizardTierIdx] = *m.editStr
		}
		m.wizardTierIdx++
		return m.nextTierSelect()
	}

	return m, cmd
}

// --- Helpers ---

func (m *model) rebuildRows() {
	m.rows = flattenTree(m.svc.Tree(), m.expanded)
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

func (m *model) ensureCursorVisible() {
	if m.cursor < m.viewport.YOffset {
		m.viewport.SetYOffset(m.cursor)
	} else if m.cursor >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
	}
}

func (m model) renderTree() string {
	var b strings.Builder
	for i, r := range m.rows {
		b.WriteString(renderRow(r, m.svc, i == m.cursor))
		b.WriteString("\n")
	}
	if m.flash != "" {
		b.WriteString("\n")
		b.WriteString(flashStyle.Render("  " + m.flash))
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) View() string {
	switch m.state {
	case stateEdit, stateProfileSelect, stateProviderAuth, stateModelSelect,
		stateBackupDest, stateBackupCreds, stateXAuth:
		if m.form != nil {
			var b strings.Builder
			if m.wizardError != "" {
				b.WriteString("\n")
				b.WriteString(errorStyle.Render("  " + m.wizardError))
				b.WriteString("\n")
			}
			b.WriteString("\n")
			b.WriteString(m.form.View())
			return b.String()
		}
	case stateVerifying:
		if m.wizardXBrowser {
			return fmt.Sprintf("\n  %s Checking browsers...\n", m.wizardSpinner.View())
		}
		if m.wizardBackupDest != nil && *m.wizardBackupDest != "" {
			label := "Verifying backup connection..."
			if *m.wizardBackupDest == "gdrive" {
				label = "Setting up Google Drive (check your browser)..."
			}
			return fmt.Sprintf("\n  %s %s\n", m.wizardSpinner.View(), label)
		}
		provName := ""
		if m.wizardProvIdx < len(m.wizardProviders) {
			provName = m.wizardProviders[m.wizardProvIdx]
		}
		return fmt.Sprintf("\n  %s Verifying %s...\n", m.wizardSpinner.View(), provName)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("  obk settings"))
	b.WriteString("\n")

	if m.ready {
		b.WriteString(m.viewport.View())
	} else {
		b.WriteString(m.renderTree())
	}

	b.WriteString(helpStyle.Render("  ↑↓ navigate  enter edit/expand  q quit"))
	b.WriteString("\n")
	return b.String()
}

// --- Async commands ---

func verifyProviderCmd(svc *settings.Service, name, apiKey string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := svc.Config()
		var pcfg config.ModelProviderConfig
		if cfg.Models != nil && cfg.Models.Providers != nil {
			pcfg = cfg.Models.Providers[name]
		}

		models, err := provider.ListModels(ctx, name, apiKey, pcfg)
		if err != nil {
			return verifyResultMsg{provider: name, err: err}
		}

		// Cache results.
		cache := provider.NewModelCache(config.ModelsDir())
		list := &provider.CachedModelList{
			Provider:  name,
			Models:    models,
			FetchedAt: time.Now(),
		}
		if existing, loadErr := cache.Load(name); loadErr == nil && existing.VerifiedModels != nil {
			list.VerifiedModels = existing.VerifiedModels
		}
		_ = cache.Save(name, list)

		return verifyResultMsg{provider: name, err: nil}
	}
}

func loadModelsInBackgroundCmd(provName string, svc *settings.Service) tea.Cmd {
	return func() tea.Msg {
		cache := provider.NewModelCache(config.ModelsDir())
		if !cache.IsStale(provName, 24*time.Hour) {
			return modelsLoadedMsg{provider: provName}
		}

		cfg := svc.Config()
		var pcfg config.ModelProviderConfig
		if cfg.Models != nil && cfg.Models.Providers != nil {
			pcfg = cfg.Models.Providers[provName]
		}

		var apiKey string
		if pcfg.AuthMethod != "vertex_ai" && pcfg.APIKeyRef != "" {
			key, err := svc.LoadCredential(pcfg.APIKeyRef)
			if err != nil || key == "" {
				return modelsLoadedMsg{provider: provName, err: fmt.Errorf("no key")}
			}
			apiKey = key
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		models, err := provider.ListModels(ctx, provName, apiKey, pcfg)
		return modelsLoadedMsg{provider: provName, models: models, err: err}
	}
}

func providerConfig(cfg *config.Config, name string) config.ModelProviderConfig {
	if cfg.Models != nil && cfg.Models.Providers != nil {
		return cfg.Models.Providers[name]
	}
	return config.ModelProviderConfig{}
}

func configuredProviderNames(cfg *config.Config) []string {
	if cfg.Models == nil || cfg.Models.Providers == nil {
		return nil
	}
	var names []string
	for name, pc := range cfg.Models.Providers {
		if pc.APIKeyRef != "" || pc.AuthMethod == "vertex_ai" {
			names = append(names, name)
		}
	}
	return names
}

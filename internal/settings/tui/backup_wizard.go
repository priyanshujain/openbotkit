package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	backupsvc "github.com/73ai/openbotkit/service/backup"
	"github.com/73ai/openbotkit/settings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// --- Backup wizard: destination → credentials → verify → save ---

// enterBackupWizard starts the first-time wizard (destination not yet configured).
func (m model) enterBackupWizard() (model, tea.Cmd) {
	m.wizardBackupSnapshot = cloneBackupConfig(m.svc.Config().Backup)

	cfg := m.svc.Config()
	dest := settings.BackupDest(cfg)
	if dest != "" && !m.svc.IsBackupDestConfigured(dest) {
		// Destination set but not authenticated — skip picker, go to auth.
		d := dest
		m.wizardBackupDest = &d
		return m.enterBackupAuth(dest)
	}
	return m.enterBackupDest()
}

// enterDestinationChange handles transactional destination change from settings tree.
func (m model) enterDestinationChange() (model, tea.Cmd) {
	m.wizardBackupSnapshot = cloneBackupConfig(m.svc.Config().Backup)
	return m.enterBackupDest()
}

func (m model) enterBackupDest() (model, tea.Cmd) {
	m.state = stateBackupDest
	m.wizardError = ""

	dest := ""
	cfg := m.svc.Config()
	if cfg.Backup != nil && cfg.Backup.Destination != "" {
		dest = cfg.Backup.Destination
	}
	m.wizardBackupDest = &dest

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where would you like to back up to?").
				Options(
					huh.NewOption("Cloudflare R2 (S3-compatible)", "r2"),
					huh.NewOption("Google Drive", "gdrive"),
				).
				Value(m.wizardBackupDest),
		),
	)
	return m, m.form.Init()
}

func (m model) updateBackupDest(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m.rollbackBackup()
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		newDest := *m.wizardBackupDest

		// If destination is already authenticated, commit immediately.
		if m.svc.IsBackupDestConfigured(newDest) {
			return m.commitDestinationSwap(newDest)
		}

		// Otherwise, start auth flow for the new destination.
		return m.enterBackupAuth(newDest)
	}

	return m, cmd
}

// enterBackupAuth routes to the correct auth flow for a destination.
func (m model) enterBackupAuth(dest string) (model, tea.Cmd) {
	switch dest {
	case "r2":
		return m.enterBackupR2Creds()
	case "gdrive":
		return m.enterBackupGDriveCreds()
	}
	return m.rollbackBackup()
}

// commitDestinationSwap saves the destination change and triggers backup if stale.
func (m model) commitDestinationSwap(dest string) (model, tea.Cmd) {
	cfg := m.svc.Config()
	ensureBackup(cfg)
	cfg.Backup.Destination = dest
	cfg.Backup.Enabled = true
	if cfg.Backup.Schedule == "" {
		cfg.Backup.Schedule = "6h"
	}

	if err := m.svc.Save(); err != nil {
		m.flash = fmt.Sprintf("Error saving: %v", err)
		return m.rollbackBackup()
	}

	if err := config.LinkSource("backup"); err != nil {
		m.flash = fmt.Sprintf("Error linking backup: %v", err)
		return m.rollbackBackup()
	}

	m.flash = "Destination updated!"
	m.wizardBackupSnapshot = nil
	m.state = stateBrowse
	m.form = nil
	m.wizardBackupDest = nil
	m.svc.RebuildTree()
	m.rebuildRows()
	m.viewport.SetContent(m.renderTree())
	return m, tea.Batch(
		tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return flashMsg{}
		}),
		triggerBackupIfStaleCmd(m.svc),
	)
}

func (m model) enterBackupR2Creds() (model, tea.Cmd) {
	m.state = stateBackupCreds
	m.wizardError = ""

	bucket := ""
	endpoint := ""
	ak := ""
	sk := ""

	cfg := m.svc.Config()
	if cfg.Backup != nil && cfg.Backup.R2 != nil {
		bucket = cfg.Backup.R2.Bucket
		endpoint = cfg.Backup.R2.Endpoint
	}

	m.wizardBackupBucket = &bucket
	m.wizardBackupEndpoint = &endpoint
	m.wizardBackupAK = &ak
	m.wizardBackupSK = &sk

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("R2 Bucket name").
				Description("Cloudflare Dashboard → R2 Object Storage → your bucket").
				Value(m.wizardBackupBucket),
			huh.NewInput().
				Title("R2 Endpoint").
				Description("Bucket → Settings → S3 API → copy the endpoint URL").
				Placeholder("https://<account-id>.r2.cloudflarestorage.com").
				Value(m.wizardBackupEndpoint),
			huh.NewInput().
				Title("Access Key ID").
				Description("R2 → Manage R2 API Tokens → Create API Token").
				Value(m.wizardBackupAK),
			huh.NewInput().
				Title("Secret Access Key").
				Description("Shown once when you create the API token above").
				EchoMode(huh.EchoModePassword).
				Value(m.wizardBackupSK),
		),
	)
	return m, m.form.Init()
}

func (m model) enterBackupGDriveCreds() (model, tea.Cmd) {
	cfg := m.svc.Config()
	ensureBackup(cfg)
	cfg.Backup.Destination = "gdrive"

	m.state = stateVerifying
	m.wizardError = ""
	return m, tea.Batch(
		m.wizardSpinner.Tick,
		setupGDriveCmd(m.svc, "obk-backup"),
	)
}

func (m model) updateBackupCreds(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m.rollbackBackup()
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		return m.handleR2CredsComplete()
	}

	return m, cmd
}

func (m model) handleR2CredsComplete() (model, tea.Cmd) {
	bucket := strings.TrimSpace(*m.wizardBackupBucket)
	endpoint := strings.TrimSpace(*m.wizardBackupEndpoint)
	ak := strings.TrimSpace(*m.wizardBackupAK)
	sk := strings.TrimSpace(*m.wizardBackupSK)

	if bucket == "" || endpoint == "" || ak == "" || sk == "" {
		m.wizardError = "All R2 fields are required"
		return m.enterBackupR2Creds()
	}

	akRef := "keychain:obk/r2-access-key"
	skRef := "keychain:obk/r2-secret-key"

	if err := m.svc.StoreCredential(akRef, ak); err != nil {
		m.wizardError = fmt.Sprintf("Store access key: %v", err)
		return m.enterBackupR2Creds()
	}
	if err := m.svc.StoreCredential(skRef, sk); err != nil {
		m.wizardError = fmt.Sprintf("Store secret key: %v", err)
		return m.enterBackupR2Creds()
	}

	cfg := m.svc.Config()
	ensureBackup(cfg)
	cfg.Backup.Destination = "r2"
	if cfg.Backup.R2 == nil {
		cfg.Backup.R2 = &config.R2Config{}
	}
	cfg.Backup.R2.Bucket = bucket
	cfg.Backup.R2.Endpoint = endpoint
	cfg.Backup.R2.AccessKeyRef = akRef
	cfg.Backup.R2.SecretKeyRef = skRef

	m.state = stateVerifying
	m.wizardError = ""
	return m, tea.Batch(
		m.wizardSpinner.Tick,
		verifyBackupCmd(m.svc, "r2"),
	)
}

// saveBackup completes the wizard: enables backup, saves, triggers if stale.
func (m model) saveBackup() (model, tea.Cmd) {
	cfg := m.svc.Config()
	cfg.Backup.Enabled = true
	if cfg.Backup.Schedule == "" {
		cfg.Backup.Schedule = "6h"
	}

	if err := m.svc.Save(); err != nil {
		m.flash = fmt.Sprintf("Error saving: %v", err)
		return m.rollbackBackup()
	}

	if err := config.LinkSource("backup"); err != nil {
		m.flash = fmt.Sprintf("Error linking backup: %v", err)
		return m.rollbackBackup()
	}

	m.flash = "Backup configured and enabled!"
	m.wizardBackupSnapshot = nil
	m.state = stateBrowse
	m.form = nil
	m.wizardBackupDest = nil
	m.svc.RebuildTree()
	m.rebuildRows()
	m.viewport.SetContent(m.renderTree())
	return m, tea.Batch(
		tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return flashMsg{}
		}),
		triggerBackupIfStaleCmd(m.svc),
	)
}

// rollbackBackup reverts config to the snapshot taken before the wizard started.
func (m model) rollbackBackup() (model, tea.Cmd) {
	m.svc.Config().Backup = m.wizardBackupSnapshot
	m.wizardBackupSnapshot = nil
	m.state = stateBrowse
	m.form = nil
	m.wizardBackupDest = nil
	m.svc.RebuildTree()
	m.rebuildRows()
	m.viewport.SetContent(m.renderTree())
	return m, nil
}

func verifyBackupCmd(svc *settings.Service, dest string) tea.Cmd {
	return func() tea.Msg {
		err := svc.VerifyBackup(dest, svc.Config())
		return backupVerifyResultMsg{err: err}
	}
}

func setupGDriveCmd(svc *settings.Service, folderName string) tea.Cmd {
	return func() tea.Msg {
		folderID, err := svc.SetupGDrive(svc.Config(), folderName)
		if err != nil {
			return backupVerifyResultMsg{err: err}
		}
		return backupVerifyResultMsg{folderID: folderID}
	}
}

// triggerBackupIfStaleCmd triggers a backup if the last one is older than the schedule.
func triggerBackupIfStaleCmd(svc *settings.Service) tea.Cmd {
	return func() tea.Msg {
		cfg := svc.Config()
		if cfg.Backup == nil || !cfg.Backup.Enabled {
			return nil
		}

		schedule := parseSchedule(cfg.Backup.Schedule)
		if schedule == 0 {
			return nil // manual only
		}

		manifest, err := backupsvc.LoadManifest(config.BackupLastManifestPath())
		if err != nil {
			return nil
		}

		if manifest.ID != "" && time.Since(manifest.Timestamp) < schedule {
			return nil // recent enough
		}

		err = svc.TriggerBackup()
		return backupTriggeredMsg{err: err}
	}
}

func parseSchedule(s string) time.Duration {
	switch s {
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	default:
		return 0
	}
}

func ensureBackup(c *config.Config) {
	if c.Backup == nil {
		c.Backup = &config.BackupConfig{}
	}
}

func cloneBackupConfig(b *config.BackupConfig) *config.BackupConfig {
	if b == nil {
		return nil
	}
	clone := *b
	if b.R2 != nil {
		r2 := *b.R2
		clone.R2 = &r2
	}
	if b.GDrive != nil {
		gd := *b.GDrive
		clone.GDrive = &gd
	}
	return &clone
}

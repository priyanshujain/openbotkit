package settings

import (
	"fmt"

	"github.com/73ai/openbotkit/config"
)

type FieldType int

const (
	TypeString FieldType = iota
	TypePassword
	TypeSelect
	TypeBool
	TypeNumber
)

type Option struct {
	Label string
	Value string
}

type Field struct {
	Key         string
	Label       string
	Description string
	Type        FieldType
	Options     []Option
	OptionsFunc func(*config.Config) []Option // dynamic options, overrides Options
	Get         func(*config.Config) string
	Set         func(*config.Config, string) error
	Validate    func(string) error
	AfterSet    func(*Service) string     // optional post-set message
	ReadOnly    func(*config.Config) bool // if true, field can't be edited
}

type Category struct {
	Key      string
	Label    string
	Children []Node
}

type Node struct {
	Category *Category
	Field    *Field
}

type Service struct {
	tree           []Node
	cfg            *config.Config
	saveFn         func(*config.Config) error
	storeCred      func(ref, value string) error
	loadCred       func(ref string) (string, error)
	verifyProvider func(name string, cfg config.ModelProviderConfig) error
	verifyBackup   func(dest string, cfg *config.Config) error
	setupGDrive    func(cfg *config.Config, folderName string) (string, error)
	triggerBackup  func(cfg *config.Config) error
}

type ServiceOption func(*Service)

func WithSaveFn(fn func(*config.Config) error) ServiceOption {
	return func(s *Service) { s.saveFn = fn }
}

func WithStoreCred(fn func(ref, value string) error) ServiceOption {
	return func(s *Service) { s.storeCred = fn }
}

func WithLoadCred(fn func(ref string) (string, error)) ServiceOption {
	return func(s *Service) { s.loadCred = fn }
}

func WithVerifyProvider(fn func(name string, cfg config.ModelProviderConfig) error) ServiceOption {
	return func(s *Service) { s.verifyProvider = fn }
}

func WithVerifyBackup(fn func(dest string, cfg *config.Config) error) ServiceOption {
	return func(s *Service) { s.verifyBackup = fn }
}

func WithSetupGDrive(fn func(cfg *config.Config, folderName string) (string, error)) ServiceOption {
	return func(s *Service) { s.setupGDrive = fn }
}

func WithTriggerBackup(fn func(cfg *config.Config) error) ServiceOption {
	return func(s *Service) { s.triggerBackup = fn }
}

func New(cfg *config.Config, opts ...ServiceOption) *Service {
	s := &Service{
		cfg:    cfg,
		saveFn: func(c *config.Config) error { return c.Save() },
	}
	for _, opt := range opts {
		opt(s)
	}
	s.tree = BuildTree(s)
	return s
}

func (s *Service) Tree() []Node           { return s.tree }
func (s *Service) Config() *config.Config { return s.cfg }

func (s *Service) RebuildTree() {
	s.tree = BuildTree(s)
}

func (s *Service) GetValue(f *Field) string {
	return f.Get(s.cfg)
}

func (s *Service) SetValue(f *Field, value string) error {
	if f.Validate != nil {
		if err := f.Validate(value); err != nil {
			return fmt.Errorf("validation: %w", err)
		}
	}
	if err := f.Set(s.cfg, value); err != nil {
		return fmt.Errorf("set: %w", err)
	}
	if err := s.saveFn(s.cfg); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func (s *Service) StoreCredential(ref, value string) error {
	if s.storeCred == nil {
		return fmt.Errorf("no credential store configured")
	}
	return s.storeCred(ref, value)
}

func (s *Service) LoadCredential(ref string) (string, error) {
	if s.loadCred == nil {
		return "", fmt.Errorf("no credential loader configured")
	}
	return s.loadCred(ref)
}

func (s *Service) Save() error {
	return s.saveFn(s.cfg)
}

func (s *Service) VerifyProvider(name string, cfg config.ModelProviderConfig) error {
	if s.verifyProvider == nil {
		return nil
	}
	return s.verifyProvider(name, cfg)
}

func (s *Service) VerifyBackup(dest string, cfg *config.Config) error {
	if s.verifyBackup == nil {
		return nil
	}
	return s.verifyBackup(dest, cfg)
}

func (s *Service) SetupGDrive(cfg *config.Config, folderName string) (string, error) {
	if s.setupGDrive == nil {
		return "", fmt.Errorf("Google Drive setup not available — run 'obk setup' instead")
	}
	return s.setupGDrive(cfg, folderName)
}

func (s *Service) TriggerBackup() error {
	if s.triggerBackup == nil {
		return nil
	}
	return s.triggerBackup(s.cfg)
}

// IsBackupDestConfigured returns true if the given destination has credentials configured.
func (s *Service) IsBackupDestConfigured(dest string) bool {
	switch dest {
	case "r2":
		return isR2Configured(s.cfg)
	case "gdrive":
		return isGDriveConfigured(s.cfg)
	}
	return false
}

// ResolvedOptions returns the options for a field, using OptionsFunc if set.
func (s *Service) ResolvedOptions(f *Field) []Option {
	if f.OptionsFunc != nil {
		return f.OptionsFunc(s.cfg)
	}
	return f.Options
}

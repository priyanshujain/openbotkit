package settings

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	telegramsrc "github.com/73ai/openbotkit/source/telegram"
	xclient "github.com/73ai/openbotkit/source/twitter/client"
)

func BuildTree(svc *Service) []Node {
	return []Node{
		{Category: generalCategory()},
		{Category: workspaceCategory()},
		{Category: modelsCategory(svc)},
		{Category: channelsCategory()},
		{Category: dataSourcesCategory(svc)},
		{Category: integrationsCategory()},
		{Category: backupCategory(svc)},
		{Category: advancedCategory()},
	}
}

func generalCategory() *Category {
	return &Category{
		Key:   "general",
		Label: "General",
		Children: []Node{
			{Field: &Field{
				Key:   "mode",
				Label: "Mode",
				Type:  TypeSelect,
				Options: []Option{
					{"Local", string(config.ModeLocal)},
					{"Remote", string(config.ModeRemote)},
					{"Server", string(config.ModeServer)},
				},
				Get: func(c *config.Config) string {
					return string(c.ResolvedMode())
				},
				Set: func(c *config.Config, v string) error {
					c.Mode = config.Mode(v)
					return nil
				},
			}},
			{Field: &Field{
				Key:         "timezone",
				Label:       "Timezone",
				Description: "IANA timezone (e.g. America/New_York)",
				Type:        TypeString,
				Get: func(c *config.Config) string {
					return c.Timezone
				},
				Set: func(c *config.Config, v string) error {
					c.Timezone = v
					return nil
				},
				Validate: func(v string) error {
					if v == "" {
						return nil
					}
					_, err := time.LoadLocation(v)
					if err != nil {
						return fmt.Errorf("invalid timezone: %w", err)
					}
					return nil
				},
			}},
		},
	}
}

func workspaceCategory() *Category {
	return &Category{
		Key:   "workspace",
		Label: "Workspace",
		Children: []Node{
			{Field: &Field{
				Key:         "workspace",
				Label:       "Directory",
				Description: "Directory for persistent research artifacts (default: ~/.obk/workspace)",
				Type:        TypeString,
				Get: func(c *config.Config) string {
					return c.ResolvedWorkspaceDir()
				},
				Set: func(c *config.Config, v string) error {
					c.Workspace = v
					return nil
				},
			}},
		},
	}
}

func modelsCategory(svc *Service) *Category {
	children := []Node{
		{Field: profileField(svc)},
		{Field: modelTierDisplay("models.default", "Default Model", func(c *config.Config) string {
			if c.Models == nil {
				return ""
			}
			return c.Models.Default
		})},
		{Field: modelTierDisplay("models.complex", "Complex Model", func(c *config.Config) string {
			if c.Models == nil {
				return ""
			}
			return c.Models.Complex
		})},
		{Field: modelTierDisplay("models.fast", "Fast Model", func(c *config.Config) string {
			if c.Models == nil {
				return ""
			}
			return c.Models.Fast
		})},
		{Field: modelTierDisplay("models.nano", "Nano Model", func(c *config.Config) string {
			if c.Models == nil {
				return ""
			}
			return c.Models.Nano
		})},
		{Field: &Field{
			Key:   "models.context_window",
			Label: "Context Window",
			Type:  TypeNumber,
			Get: func(c *config.Config) string {
				if c.Models == nil || c.Models.ContextWindow == 0 {
					return ""
				}
				return strconv.Itoa(c.Models.ContextWindow)
			},
			Set: func(c *config.Config, v string) error {
				ensureModels(c)
				if v == "" {
					c.Models.ContextWindow = 0
					return nil
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid number: %w", err)
				}
				c.Models.ContextWindow = n
				return nil
			},
			Validate: validateNumber,
		}},
		{Field: &Field{
			Key:         "models.compaction_threshold",
			Label:       "Compaction Threshold",
			Description: "0.0–1.0",
			Type:        TypeNumber,
			Get: func(c *config.Config) string {
				if c.Models == nil || c.Models.CompactionThreshold == 0 {
					return ""
				}
				return strconv.FormatFloat(c.Models.CompactionThreshold, 'f', -1, 64)
			},
			Set: func(c *config.Config, v string) error {
				ensureModels(c)
				if v == "" {
					c.Models.CompactionThreshold = 0
					return nil
				}
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return fmt.Errorf("invalid number: %w", err)
				}
				c.Models.CompactionThreshold = f
				return nil
			},
		}},
		{Category: providersCategory(svc)},
	}

	return &Category{
		Key:      "models",
		Label:    "LLM Models",
		Children: children,
	}
}

func profileField(svc *Service) *Field {
	return &Field{
		Key:   "models.profile",
		Label: "LLM Cost Profile",
		Type:  TypeString,
		Get: func(c *config.Config) string {
			if c.Models == nil || c.Models.Profile == "" {
				if c.Models != nil && c.Models.Default != "" {
					return "(custom)"
				}
				return "(not configured)"
			}
			name := c.Models.Profile
			if p, ok := config.Profiles[name]; ok {
				return p.Label
			}
			if c.Models.CustomProfiles != nil {
				if cp, ok := c.Models.CustomProfiles[name]; ok {
					label := cp.Label
					if label == "" {
						label = name
					}
					return label + " (custom)"
				}
			}
			return name
		},
		Set: func(c *config.Config, v string) error { return nil },
	}
}

// modelTierDisplay creates a display-only field for a model tier.
// Editable only when profile is custom (no fixed profile set).
func modelTierDisplay(key, label string, getter func(*config.Config) string) *Field {
	return &Field{
		Key:   key,
		Label: label,
		Type:  TypeSelect,
		Get:   getter,
		Set: func(c *config.Config, v string) error {
			ensureModels(c)
			switch key {
			case "models.default":
				c.Models.Default = v
			case "models.complex":
				c.Models.Complex = v
			case "models.fast":
				c.Models.Fast = v
			case "models.nano":
				c.Models.Nano = v
			}
			return nil
		},
		OptionsFunc: func(c *config.Config) []Option {
			return modelOptionsForTier(c, tierFromKey(key))
		},
		ReadOnly: func(c *config.Config) bool {
			if c.Models == nil || c.Models.Profile == "" {
				return false
			}
			if _, ok := config.Profiles[c.Models.Profile]; ok {
				return true
			}
			return false
		},
	}
}

func tierFromKey(key string) string {
	switch key {
	case "models.default":
		return "default"
	case "models.complex":
		return "complex"
	case "models.fast":
		return "fast"
	case "models.nano":
		return "nano"
	}
	return ""
}

// ProfilePreview returns a human-readable preview for a profile name.
func ProfilePreview(name string) string {
	if name == "custom" {
		return "Choose models manually for each tier."
	}
	p, ok := config.Profiles[name]
	if !ok {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", p.Description)
	fmt.Fprintf(&b, "  Default: %s\n", p.Tiers.Default)
	fmt.Fprintf(&b, "  Complex: %s\n", p.Tiers.Complex)
	fmt.Fprintf(&b, "  Fast:    %s\n", p.Tiers.Fast)
	fmt.Fprintf(&b, "  Nano:    %s\n\n", p.Tiers.Nano)
	fmt.Fprintf(&b, "  Providers: %s", strings.Join(p.Providers, ", "))
	return b.String()
}

// maskCredential loads a credential and returns a masked version like "sk-ant...4x2f".
func maskCredential(svc *Service, ref string) string {
	if svc.loadCred == nil {
		return "(configured)"
	}
	key, err := svc.loadCred(ref)
	if err != nil || key == "" {
		return "(configured)"
	}
	return MaskKey(key)
}

// MaskKey masks an API key showing first 6 and last 4 chars.
func MaskKey(key string) string {
	if len(key) <= 10 {
		return "****"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func providerConfig(cfg *config.Config, name string) config.ModelProviderConfig {
	if cfg.Models != nil && cfg.Models.Providers != nil {
		return cfg.Models.Providers[name]
	}
	return config.ModelProviderConfig{}
}

// modelOptionsForTier returns select options for a tier based on cached models.
func modelOptionsForTier(c *config.Config, tier string) []Option {
	configured := configuredProviders(c)
	if len(configured) == 0 {
		return []Option{{"(configure a provider first)", ""}}
	}

	cache := provider.NewModelCache(config.ModelsDir())
	opts := []Option{{"(none)", ""}}
	seen := make(map[string]bool)

	for _, provName := range configured {
		list, err := cache.Load(provName)
		if err != nil || len(list.Models) == 0 {
			opts = append(opts, Option{"(verify " + provName + " to see models)", ""})
			continue
		}
		for _, m := range list.Models {
			spec := provName + "/" + m.ID
			if seen[spec] {
				continue
			}
			seen[spec] = true
			label := m.DisplayName
			if label == "" {
				label = m.ID
			}
			opts = append(opts, Option{label + " (" + provName + ")", spec})
		}
	}
	return opts
}

// configuredProviders returns names of providers that have API keys or vertex_ai auth.
func configuredProviders(c *config.Config) []string {
	if c.Models == nil || c.Models.Providers == nil {
		return nil
	}
	var names []string
	for name, pc := range c.Models.Providers {
		if pc.APIKeyRef != "" || pc.AuthMethod == "vertex_ai" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func providersCategory(svc *Service) *Category {
	type providerDef struct {
		key        string
		label      string
		hasAuth    bool
		authOpts   []Option
		keychainID string
	}

	providers := []providerDef{
		{
			key: "anthropic", label: "Anthropic", hasAuth: true,
			authOpts:   []Option{{"API Key", "api_key"}, {"Vertex AI", "vertex_ai"}},
			keychainID: "obk/anthropic",
		},
		{key: "openai", label: "OpenAI", keychainID: "obk/openai"},
		{
			key: "gemini", label: "Gemini", hasAuth: true,
			authOpts:   []Option{{"API Key", "api_key"}, {"Vertex AI", "vertex_ai"}},
			keychainID: "obk/gemini",
		},
		{key: "groq", label: "Groq", keychainID: "obk/groq"},
		{key: "openrouter", label: "OpenRouter", keychainID: "obk/openrouter"},
		{key: "zai", label: "Z.AI", keychainID: "obk/zai"},
	}

	var children []Node
	for _, p := range providers {
		var fields []Node
		ref := "keychain:" + p.keychainID
		provKey := p.key

		fields = append(fields, Node{Field: &Field{
			Key:   "models.providers." + p.key + ".api_key",
			Label: "API Key",
			Type:  TypePassword,
			Get: func(c *config.Config) string {
				if c.Models == nil || c.Models.Providers == nil {
					return "not configured"
				}
				pc, ok := c.Models.Providers[provKey]
				if !ok || pc.APIKeyRef == "" {
					return "not configured"
				}
				return maskCredential(svc, pc.APIKeyRef)
			},
			Set: func(c *config.Config, v string) error {
				if v == "" {
					return nil
				}
				ensureModels(c)
				if c.Models.Providers == nil {
					c.Models.Providers = make(map[string]config.ModelProviderConfig)
				}
				if err := svc.StoreCredential(ref, v); err != nil {
					return fmt.Errorf("store credential: %w", err)
				}
				pc := c.Models.Providers[provKey]
				pc.APIKeyRef = ref
				c.Models.Providers[provKey] = pc
				return nil
			},
			AfterSet: func(s *Service) string {
				if s.cfg.Models == nil || s.cfg.Models.Providers == nil {
					return ""
				}
				pc, ok := s.cfg.Models.Providers[provKey]
				if !ok || pc.APIKeyRef == "" {
					return ""
				}
				err := s.VerifyProvider(provKey, pc)
				if err != nil {
					return fmt.Sprintf("Warning: verification failed: %v", err)
				}
				return "API key verified"
			},
		}})

		if p.hasAuth {
			fields = append(fields, Node{Field: &Field{
				Key:     "models.providers." + p.key + ".auth_method",
				Label:   "Auth Method",
				Type:    TypeSelect,
				Options: p.authOpts,
				Get: func(c *config.Config) string {
					if c.Models == nil || c.Models.Providers == nil {
						return "api_key"
					}
					pc, ok := c.Models.Providers[provKey]
					if !ok || pc.AuthMethod == "" {
						return "api_key"
					}
					return pc.AuthMethod
				},
				Set: func(c *config.Config, v string) error {
					ensureModels(c)
					if c.Models.Providers == nil {
						c.Models.Providers = make(map[string]config.ModelProviderConfig)
					}
					pc := c.Models.Providers[provKey]
					pc.AuthMethod = v
					c.Models.Providers[provKey] = pc
					return nil
				},
			}})
		}

		children = append(children, Node{Category: &Category{
			Key:      "models.providers." + p.key,
			Label:    p.label,
			Children: fields,
		}})
	}

	return &Category{
		Key:      "models.providers",
		Label:    "Providers",
		Children: children,
	}
}

func channelsCategory() *Category {
	return &Category{
		Key:   "channels",
		Label: "Channels",
		Children: []Node{
			{Category: &Category{
				Key:   "channels.telegram",
				Label: "Telegram",
				Children: []Node{
					{Field: &Field{
						Key:   "channels.telegram.bot_token",
						Label: "Bot Token",
						Type:  TypePassword,
						Get: func(c *config.Config) string {
							if c.Channels == nil || c.Channels.Telegram == nil || c.Channels.Telegram.BotToken == "" {
								return "not configured"
							}
							return "configured"
						},
						Set: func(c *config.Config, v string) error {
							if c.Channels == nil {
								c.Channels = &config.ChannelsConfig{}
							}
							if c.Channels.Telegram == nil {
								c.Channels.Telegram = &config.TelegramConfig{}
							}
							c.Channels.Telegram.BotToken = v
							return nil
						},
					}},
					{Field: &Field{
						Key:   "channels.telegram.owner_id",
						Label: "Owner ID",
						Type:  TypeNumber,
						Get: func(c *config.Config) string {
							if c.Channels == nil || c.Channels.Telegram == nil || c.Channels.Telegram.OwnerID == 0 {
								return ""
							}
							return strconv.FormatInt(c.Channels.Telegram.OwnerID, 10)
						},
						Set: func(c *config.Config, v string) error {
							if c.Channels == nil {
								c.Channels = &config.ChannelsConfig{}
							}
							if c.Channels.Telegram == nil {
								c.Channels.Telegram = &config.TelegramConfig{}
							}
							if v == "" {
								c.Channels.Telegram.OwnerID = 0
								return nil
							}
							n, err := strconv.ParseInt(v, 10, 64)
							if err != nil {
								return fmt.Errorf("invalid number: %w", err)
							}
							c.Channels.Telegram.OwnerID = n
							return nil
						},
						Validate: validateNumber,
					}},
				},
			}},
		},
	}
}

func dataSourcesCategory(svc *Service) *Category {
	return &Category{
		Key:   "datasources",
		Label: "Data Sources",
		Children: []Node{
			{Category: &Category{
				Key:   "datasources.gmail",
				Label: "Gmail",
				Children: []Node{
					{Field: &Field{
						Key:   "gmail.sync_days",
						Label: "Sync Days",
						Type:  TypeNumber,
						Get: func(c *config.Config) string {
							if c.Gmail == nil || c.Gmail.SyncDays == 0 {
								return ""
							}
							return strconv.Itoa(c.Gmail.SyncDays)
						},
						Set: func(c *config.Config, v string) error {
							if c.Gmail == nil {
								c.Gmail = &config.GmailConfig{}
							}
							if v == "" {
								c.Gmail.SyncDays = 0
								return nil
							}
							n, err := strconv.Atoi(v)
							if err != nil {
								return fmt.Errorf("invalid number: %w", err)
							}
							c.Gmail.SyncDays = n
							return nil
						},
						Validate: validateNumber,
					}},
					{Field: &Field{
						Key:   "gmail.download_attachments",
						Label: "Download Attachments",
						Type:  TypeBool,
						Get: func(c *config.Config) string {
							if c.Gmail == nil {
								return "false"
							}
							return strconv.FormatBool(c.Gmail.DownloadAttachments)
						},
						Set: func(c *config.Config, v string) error {
							if c.Gmail == nil {
								c.Gmail = &config.GmailConfig{}
							}
							b, err := strconv.ParseBool(v)
							if err != nil {
								return fmt.Errorf("invalid boolean: %w", err)
							}
							c.Gmail.DownloadAttachments = b
							return nil
						},
					}},
				},
			}},
			{Category: &Category{
				Key:   "datasources.websearch",
				Label: "Web Search",
				Children: []Node{
					{Field: &Field{
						Key:   "websearch.proxy",
						Label: "Proxy",
						Type:  TypeString,
						Get: func(c *config.Config) string {
							if c.WebSearch == nil {
								return ""
							}
							return c.WebSearch.Proxy
						},
						Set: func(c *config.Config, v string) error {
							if c.WebSearch == nil {
								c.WebSearch = &config.WebSearchConfig{}
							}
							c.WebSearch.Proxy = v
							return nil
						},
					}},
					{Field: &Field{
						Key:   "websearch.timeout",
						Label: "Timeout",
						Type:  TypeString,
						Get: func(c *config.Config) string {
							if c.WebSearch == nil {
								return ""
							}
							return c.WebSearch.Timeout
						},
						Set: func(c *config.Config, v string) error {
							if c.WebSearch == nil {
								c.WebSearch = &config.WebSearchConfig{}
							}
							c.WebSearch.Timeout = v
							return nil
						},
					}},
					{Field: &Field{
						Key:   "websearch.cache_ttl",
						Label: "Cache TTL",
						Type:  TypeString,
						Get: func(c *config.Config) string {
							if c.WebSearch == nil {
								return ""
							}
							return c.WebSearch.CacheTTL
						},
						Set: func(c *config.Config, v string) error {
							if c.WebSearch == nil {
								c.WebSearch = &config.WebSearchConfig{}
							}
							c.WebSearch.CacheTTL = v
							return nil
						},
					}},
				},
			}},
			{Category: &Category{
				Key:   "datasources.x",
				Label: "X",
				Children: []Node{
					{Field: &Field{
						Key:   "x.auth_status",
						Label: "Account",
						Type:  TypeString,
						Get: func(c *config.Config) string {
							session, err := xclient.LoadSession()
							if err != nil {
								return "Not connected (press Enter to sign in)"
							}
							if session.Username != "" {
								return "Connected as @" + session.Username
							}
							return "Connected"
						},
						Set:      func(c *config.Config, v string) error { return nil },
						ReadOnly: func(c *config.Config) bool { return true },
					}},
				},
			}},
			{Category: telegramSourceCategory(svc)},
		},
	}
}

// IsTelegramConfigured reports whether the Telegram app credentials are set.
// Signing in is pointless without them, so the login entry stays gated.
func IsTelegramConfigured(c *config.Config) bool {
	return c.Telegram != nil && c.Telegram.APIIDRef != "" && c.Telegram.APIHashRef != ""
}

func ensureTelegram(c *config.Config) {
	if c.Telegram == nil {
		c.Telegram = &config.TelegramSourceConfig{}
	}
	if c.Telegram.Storage.Driver == "" {
		c.Telegram.Storage.Driver = "sqlite"
	}
}

func telegramSourceCategory(svc *Service) *Category {
	return &Category{
		Key:   "datasources.telegram",
		Label: "Telegram",
		Children: []Node{
			{Field: &Field{
				Key:         "telegram.auth_status",
				Label:       "Account",
				Description: "Sign in by scanning a QR code in your browser",
				Type:        TypeString,
				Get: func(c *config.Config) string {
					if !IsTelegramConfigured(c) {
						return "Set api_id and api_hash first"
					}
					if telegramsrc.HasSession(c.TelegramSessionPath()) {
						return "Connected"
					}
					return "Not connected (press Enter to sign in)"
				},
				Set:      func(c *config.Config, v string) error { return nil },
				ReadOnly: func(c *config.Config) bool { return true },
			}},
			{Field: &Field{
				Key:         "telegram.api_id",
				Label:       "api_id",
				Description: "my.telegram.org -> API development tools",
				Type:        TypePassword,
				Get: func(c *config.Config) string {
					if c.Telegram == nil || c.Telegram.APIIDRef == "" {
						return "not configured"
					}
					return maskCredential(svc, c.Telegram.APIIDRef)
				},
				Set: func(c *config.Config, v string) error {
					if v == "" {
						return nil
					}
					ensureTelegram(c)
					if err := svc.StoreCredential(telegramsrc.APIIDRef, v); err != nil {
						return fmt.Errorf("store credential: %w", err)
					}
					c.Telegram.APIIDRef = telegramsrc.APIIDRef
					return nil
				},
				Validate: func(v string) error {
					if v == "" {
						return nil
					}
					if _, err := strconv.Atoi(strings.TrimSpace(v)); err != nil {
						return fmt.Errorf("api_id must be a number")
					}
					return nil
				},
			}},
			{Field: &Field{
				Key:         "telegram.api_hash",
				Label:       "api_hash",
				Description: "Shown next to the api_id on my.telegram.org",
				Type:        TypePassword,
				Get: func(c *config.Config) string {
					if c.Telegram == nil || c.Telegram.APIHashRef == "" {
						return "not configured"
					}
					return maskCredential(svc, c.Telegram.APIHashRef)
				},
				Set: func(c *config.Config, v string) error {
					if v == "" {
						return nil
					}
					ensureTelegram(c)
					if err := svc.StoreCredential(telegramsrc.APIHashRef, v); err != nil {
						return fmt.Errorf("store credential: %w", err)
					}
					c.Telegram.APIHashRef = telegramsrc.APIHashRef
					return nil
				},
			}},
			{Field: &Field{
				Key:         "telegram.backfill_days",
				Label:       "Backfill Window (days)",
				Description: "How far back to sync message history",
				Type:        TypeNumber,
				Get: func(c *config.Config) string {
					return strconv.Itoa(c.TelegramBackfillDays())
				},
				Set: func(c *config.Config, v string) error {
					days, err := strconv.Atoi(strings.TrimSpace(v))
					if err != nil {
						return fmt.Errorf("invalid number: %w", err)
					}
					if days <= 0 {
						return fmt.Errorf("backfill window must be at least 1 day")
					}
					ensureTelegram(c)
					c.Telegram.BackfillDays = days
					return nil
				},
			}},
		},
	}
}

func integrationsCategory() *Category {
	return &Category{
		Key:   "integrations",
		Label: "Integrations",
		Children: []Node{
			{Category: &Category{
				Key:   "integrations.gws",
				Label: "Google Workspace",
				Children: []Node{
					{Field: &Field{
						Key:   "integrations.gws.enabled",
						Label: "Enabled",
						Type:  TypeBool,
						Get: func(c *config.Config) string {
							if c.Integrations == nil || c.Integrations.GWS == nil {
								return "false"
							}
							return strconv.FormatBool(c.Integrations.GWS.Enabled)
						},
						Set: func(c *config.Config, v string) error {
							ensureGWS(c)
							b, err := strconv.ParseBool(v)
							if err != nil {
								return fmt.Errorf("invalid boolean: %w", err)
							}
							c.Integrations.GWS.Enabled = b
							return nil
						},
					}},
					{Field: &Field{
						Key:   "integrations.gws.callback_url",
						Label: "Callback URL",
						Type:  TypeString,
						Get: func(c *config.Config) string {
							if c.Integrations == nil || c.Integrations.GWS == nil {
								return ""
							}
							return c.Integrations.GWS.CallbackURL
						},
						Set: func(c *config.Config, v string) error {
							ensureGWS(c)
							c.Integrations.GWS.CallbackURL = v
							return nil
						},
					}},
					{Field: &Field{
						Key:   "integrations.gws.ngrok_domain",
						Label: "Ngrok Domain",
						Type:  TypeString,
						Get: func(c *config.Config) string {
							if c.Integrations == nil || c.Integrations.GWS == nil {
								return ""
							}
							return c.Integrations.GWS.NgrokDomain
						},
						Set: func(c *config.Config, v string) error {
							ensureGWS(c)
							c.Integrations.GWS.NgrokDomain = v
							return nil
						},
					}},
				},
			}},
		},
	}
}

func backupCategory(svc *Service) *Category {
	children := []Node{
		{Field: &Field{
			Key:   "backup.enabled",
			Label: "Enabled",
			Type:  TypeBool,
			Get: func(c *config.Config) string {
				if c.Backup == nil {
					return "false"
				}
				return strconv.FormatBool(c.Backup.Enabled)
			},
			Set: func(c *config.Config, v string) error {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return fmt.Errorf("invalid boolean: %w", err)
				}
				if b {
					if err := validateBackupReady(c); err != nil {
						return err
					}
				}
				ensureBackup(c)
				c.Backup.Enabled = b
				return nil
			},
			ReadOnly: func(c *config.Config) bool {
				if c.Backup != nil && c.Backup.Enabled {
					return false
				}
				return !isBackupDestinationConfigured(c)
			},
		}},
		{Field: &Field{
			Key:   "backup.destination",
			Label: "Destination",
			Type:  TypeSelect,
			Options: []Option{
				{"(not set)", ""},
				{"Cloudflare R2", "r2"},
				{"Google Drive", "gdrive"},
			},
			Get: func(c *config.Config) string {
				if c.Backup == nil {
					return ""
				}
				return c.Backup.Destination
			},
			Set: func(c *config.Config, v string) error {
				ensureBackup(c)
				if c.Backup.Destination != v {
					c.Backup.Enabled = false
				}
				c.Backup.Destination = v
				return nil
			},
		}},
		{Field: &Field{
			Key:   "backup.schedule",
			Label: "Schedule",
			Type:  TypeSelect,
			Options: []Option{
				{"Every 6 hours", "6h"},
				{"Every 12 hours", "12h"},
				{"Daily", "24h"},
				{"Manual only", ""},
			},
			Get: func(c *config.Config) string {
				if c.Backup == nil {
					return ""
				}
				return c.Backup.Schedule
			},
			Set: func(c *config.Config, v string) error {
				ensureBackup(c)
				c.Backup.Schedule = v
				return nil
			},
			ReadOnly: func(c *config.Config) bool {
				return c.Backup == nil || !c.Backup.Enabled
			},
		}},
	}

	// Only show R2 sub-category when destination is R2.
	if BackupDest(svc.cfg) == "r2" {
		children = append(children, Node{Category: &Category{
			Key:   "backup.r2",
			Label: "Cloudflare R2",
			Children: []Node{
				{Field: &Field{
					Key:         "backup.r2.bucket",
					Label:       "Bucket",
					Description: "Cloudflare Dashboard → R2 Object Storage → your bucket",
					Type:        TypeString,
					Get: func(c *config.Config) string {
						if c.Backup == nil || c.Backup.R2 == nil {
							return ""
						}
						return c.Backup.R2.Bucket
					},
					Set: func(c *config.Config, v string) error {
						ensureBackupR2(c)
						c.Backup.R2.Bucket = v
						return nil
					},
				}},
				{Field: &Field{
					Key:         "backup.r2.endpoint",
					Label:       "Endpoint",
					Description: "Bucket → Settings → S3 API → endpoint URL",
					Type:        TypeString,
					Get: func(c *config.Config) string {
						if c.Backup == nil || c.Backup.R2 == nil {
							return ""
						}
						return c.Backup.R2.Endpoint
					},
					Set: func(c *config.Config, v string) error {
						ensureBackupR2(c)
						c.Backup.R2.Endpoint = v
						return nil
					},
				}},
				{Field: &Field{
					Key:         "backup.r2.access_key",
					Label:       "Access Key ID",
					Description: "R2 → Manage R2 API Tokens → Create API Token",
					Type:        TypePassword,
					Get: func(c *config.Config) string {
						if c.Backup == nil || c.Backup.R2 == nil || c.Backup.R2.AccessKeyRef == "" {
							return "not configured"
						}
						return maskCredential(svc, c.Backup.R2.AccessKeyRef)
					},
					Set: func(c *config.Config, v string) error {
						if v == "" {
							return nil
						}
						ensureBackupR2(c)
						ref := "keychain:obk/r2-access-key"
						if err := svc.StoreCredential(ref, v); err != nil {
							return fmt.Errorf("store credential: %w", err)
						}
						c.Backup.R2.AccessKeyRef = ref
						return nil
					},
				}},
				{Field: &Field{
					Key:         "backup.r2.secret_key",
					Label:       "Secret Access Key",
					Description: "Shown once when you create the API token",
					Type:        TypePassword,
					Get: func(c *config.Config) string {
						if c.Backup == nil || c.Backup.R2 == nil || c.Backup.R2.SecretKeyRef == "" {
							return "not configured"
						}
						return maskCredential(svc, c.Backup.R2.SecretKeyRef)
					},
					Set: func(c *config.Config, v string) error {
						if v == "" {
							return nil
						}
						ensureBackupR2(c)
						ref := "keychain:obk/r2-secret-key"
						if err := svc.StoreCredential(ref, v); err != nil {
							return fmt.Errorf("store credential: %w", err)
						}
						c.Backup.R2.SecretKeyRef = ref
						return nil
					},
				}},
			},
		}})
	}

	return &Category{
		Key:      "backup",
		Label:    "Backup",
		Children: children,
	}
}

func BackupDest(c *config.Config) string {
	if c.Backup == nil {
		return ""
	}
	return c.Backup.Destination
}

func isR2Configured(c *config.Config) bool {
	if c.Backup == nil || c.Backup.R2 == nil {
		return false
	}
	r := c.Backup.R2
	return r.Bucket != "" && r.Endpoint != "" && r.AccessKeyRef != "" && r.SecretKeyRef != ""
}

func isGDriveConfigured(c *config.Config) bool {
	if c.Backup == nil || c.Backup.GDrive == nil {
		return false
	}
	return c.Backup.GDrive.FolderID != ""
}

func isBackupDestinationConfigured(c *config.Config) bool {
	switch BackupDest(c) {
	case "r2":
		return isR2Configured(c)
	case "gdrive":
		return isGDriveConfigured(c)
	default:
		return false
	}
}

func validateBackupReady(c *config.Config) error {
	dest := BackupDest(c)
	if dest == "" {
		return fmt.Errorf("select a destination first")
	}
	switch dest {
	case "r2":
		if !isR2Configured(c) {
			return fmt.Errorf("configure R2 bucket, endpoint, and credentials first")
		}
	case "gdrive":
		if !isGDriveConfigured(c) {
			return fmt.Errorf("configure Google Drive folder ID first")
		}
	}
	return nil
}

func ensureBackup(c *config.Config) {
	if c.Backup == nil {
		c.Backup = &config.BackupConfig{}
	}
}

func ensureBackupR2(c *config.Config) {
	ensureBackup(c)
	if c.Backup.R2 == nil {
		c.Backup.R2 = &config.R2Config{}
	}
}

func advancedCategory() *Category {
	return &Category{
		Key:   "advanced",
		Label: "Advanced",
		Children: []Node{
			{Category: &Category{
				Key:   "advanced.daemon",
				Label: "Daemon",
				Children: []Node{
					{Field: &Field{
						Key:         "daemon.gmail_sync_period",
						Label:       "Gmail Sync Period",
						Description: "e.g. 15m, 1h",
						Type:        TypeString,
						Get: func(c *config.Config) string {
							if c.Daemon == nil {
								return ""
							}
							return c.Daemon.GmailSyncPeriod
						},
						Set: func(c *config.Config, v string) error {
							if c.Daemon == nil {
								c.Daemon = &config.DaemonConfig{}
							}
							c.Daemon.GmailSyncPeriod = v
							return nil
						},
					}},
				},
			}},
		},
	}
}

func ensureModels(c *config.Config) {
	if c.Models == nil {
		c.Models = &config.ModelsConfig{}
	}
}

// EnsureModels initializes the Models config if nil.
func EnsureModels(c *config.Config) {
	ensureModels(c)
}

func ensureGWS(c *config.Config) {
	if c.Integrations == nil {
		c.Integrations = &config.IntegrationsConfig{}
	}
	if c.Integrations.GWS == nil {
		c.Integrations.GWS = &config.GWSConfig{}
	}
}

func validateNumber(v string) error {
	if v == "" {
		return nil
	}
	if _, err := strconv.ParseFloat(v, 64); err != nil {
		return fmt.Errorf("must be a number")
	}
	return nil
}

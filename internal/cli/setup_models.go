package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	_ "github.com/73ai/openbotkit/provider/anthropic"
	_ "github.com/73ai/openbotkit/provider/cerebras"
	_ "github.com/73ai/openbotkit/provider/gemini"
	_ "github.com/73ai/openbotkit/provider/groq"
	_ "github.com/73ai/openbotkit/provider/openai"
	_ "github.com/73ai/openbotkit/provider/openrouter"
	_ "github.com/73ai/openbotkit/provider/zai"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// providerInfo describes a configurable LLM provider.
type providerInfo struct {
	name        string
	label       string
	supportsVAI bool // supports Vertex AI
	models      []huh.Option[string]
}

var llmProviders = []providerInfo{
	{
		name:        "anthropic",
		label:       "Anthropic (Claude)",
		supportsVAI: true,
		models: []huh.Option[string]{
			huh.NewOption("claude-sonnet-4-6 (recommended)", "claude-sonnet-4-6"),
			huh.NewOption("claude-opus-4-6 (most capable)", "claude-opus-4-6"),
			huh.NewOption("claude-haiku-4-5 (fastest)", "claude-haiku-4-5"),
		},
	},
	{
		name:        "openai",
		label:       "OpenAI (GPT)",
		supportsVAI: false,
		models: []huh.Option[string]{
			huh.NewOption("gpt-4o (most capable)", "gpt-4o"),
			huh.NewOption("gpt-4o-mini (faster, cheaper)", "gpt-4o-mini"),
		},
	},
	{
		name:        "gemini",
		label:       "Google Gemini",
		supportsVAI: true,
		models: []huh.Option[string]{
			huh.NewOption("gemini-2.5-pro (most capable)", "gemini-2.5-pro"),
			huh.NewOption("gemini-2.5-flash (fast, good balance)", "gemini-2.5-flash"),
			huh.NewOption("gemini-2.5-flash-lite (fast, cheap)", "gemini-2.5-flash-lite"),
		},
	},
	{
		name:  "openrouter",
		label: "OpenRouter (500+ models)",
		models: []huh.Option[string]{
			huh.NewOption("anthropic/claude-sonnet-4-6", "anthropic/claude-sonnet-4-6"),
			huh.NewOption("anthropic/claude-haiku-4-5", "anthropic/claude-haiku-4-5"),
			huh.NewOption("google/gemini-2.5-flash-lite", "google/gemini-2.5-flash-lite"),
			huh.NewOption("mistralai/mistral-medium-3.1", "mistralai/mistral-medium-3.1"),
		},
	},
	{
		name:  "groq",
		label: "Groq (fast inference)",
		models: []huh.Option[string]{
			huh.NewOption("llama-3.1-8b-instant (fastest)", "llama-3.1-8b-instant"),
			huh.NewOption("llama-3.3-70b-versatile", "llama-3.3-70b-versatile"),
			huh.NewOption("llama-4-scout-17b-16e", "llama-4-scout-17b-16e"),
		},
	},
	{
		name:  "cerebras",
		label: "Cerebras (fast inference)",
		models: []huh.Option[string]{
			huh.NewOption("qwen-3-235b-a22b-instruct-2507 (most capable)", "qwen-3-235b-a22b-instruct-2507"),
			huh.NewOption("llama3.1-8b (fastest)", "llama3.1-8b"),
		},
	},
}

var vertexRegions = []huh.Option[string]{
	huh.NewOption("us-central1 (Iowa)", "us-central1"),
	huh.NewOption("us-east5 (Columbus)", "us-east5"),
	huh.NewOption("us-east4 (N. Virginia)", "us-east4"),
	huh.NewOption("us-west1 (Oregon)", "us-west1"),
	huh.NewOption("europe-west1 (Belgium)", "europe-west1"),
	huh.NewOption("europe-west4 (Netherlands)", "europe-west4"),
	huh.NewOption("asia-northeast1 (Tokyo)", "asia-northeast1"),
	huh.NewOption("asia-southeast1 (Singapore)", "asia-southeast1"),
}

var setupModelsCmd = &cobra.Command{
	Use:     "models",
	Short:   "Configure LLM model providers",
	Example: `  obk setup models`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if err := setupModels(cfg); err != nil {
			return err
		}
		return cfg.Save()
	},
}

func init() {
	setupCmd.AddCommand(setupModelsCmd)
}

func setupModels(cfg *config.Config) error {
	if cfg.Models == nil {
		cfg.Models = &config.ModelsConfig{}
	}
	if cfg.Models.Providers == nil {
		cfg.Models.Providers = make(map[string]config.ModelProviderConfig)
	}

	// Step 1: Choose between profile and custom configuration.
	var mode string
	var profileOptions []huh.Option[string]
	for _, name := range config.ProfileNames {
		p := config.Profiles[name]
		profileOptions = append(profileOptions, huh.NewOption(p.Label, name))
	}
	// Append custom profiles sorted alphabetically.
	if len(cfg.Models.CustomProfiles) > 0 {
		var customNames []string
		for n := range cfg.Models.CustomProfiles {
			customNames = append(customNames, n)
		}
		sort.Strings(customNames)
		for _, n := range customNames {
			cp := cfg.Models.CustomProfiles[n]
			label := cp.Label
			if label == "" {
				label = n
			}
			profileOptions = append(profileOptions, huh.NewOption(label+" (custom)", "custom:"+n))
		}
	}
	profileOptions = append(profileOptions, huh.NewOption("Custom (manual configuration)", "custom"))

	// Pre-select existing profile if configured.
	mode = "custom"
	if cfg.Models.Profile != "" {
		if _, ok := config.Profiles[cfg.Models.Profile]; ok {
			mode = cfg.Models.Profile
		} else if _, ok := cfg.Models.CustomProfiles[cfg.Models.Profile]; ok {
			mode = "custom:" + cfg.Models.Profile
		}
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How would you like to configure models?").
				Options(profileOptions...).
				Value(&mode),
		),
	).Run()
	if err != nil {
		return err
	}

	if mode == "custom" {
		return setupCustom(cfg)
	}
	// Handle custom profile selection ("custom:name").
	if strings.HasPrefix(mode, "custom:") {
		return setupWithCustomProfile(cfg, strings.TrimPrefix(mode, "custom:"))
	}
	return setupWithProfile(cfg, mode)
}

func setupWithProfile(cfg *config.Config, profileName string) error {
	return setupWithBuiltProfile(cfg, profileName, config.Profiles[profileName])
}

func setupWithCustomProfile(cfg *config.Config, name string) error {
	if cfg.Models == nil || cfg.Models.CustomProfiles == nil {
		return fmt.Errorf("custom profile %q not found", name)
	}
	cp, ok := cfg.Models.CustomProfiles[name]
	if !ok {
		return fmt.Errorf("custom profile %q not found", name)
	}
	// Convert to ModelProfile and delegate.
	profile := config.ModelProfile{
		Name:      name,
		Label:     cp.Label,
		Tiers:     cp.Tiers,
		Providers: cp.Providers,
	}
	if profile.Label == "" {
		profile.Label = name
	}
	return setupWithBuiltProfile(cfg, name, profile)
}

func setupWithBuiltProfile(cfg *config.Config, profileName string, profile config.ModelProfile) error {
	// Show tier→model mapping for confirmation.
	fmt.Printf("\n  Profile: %s\n", profile.Label)
	fmt.Printf("    Default: %s\n", profile.Tiers.Default)
	fmt.Printf("    Complex: %s\n", profile.Tiers.Complex)
	fmt.Printf("    Fast:    %s\n", profile.Tiers.Fast)
	fmt.Printf("    Nano:    %s\n\n", profile.Tiers.Nano)

	// Configure required providers.
	for _, provName := range profile.Providers {
		existing := cfg.Models.Providers[provName]
		fmt.Printf("  Configuring %s...\n", provName)
		if err := configureAPIKey(&existing, provName, existing); err != nil {
			return err
		}
		cfg.Models.Providers[provName] = existing
	}

	// Validate each provider with a test model.
	fmt.Println("\n  Validating providers...")
	testModels := profileTestModels(profile)
	for _, provName := range profile.Providers {
		model := testModels[provName]
		pcfg := cfg.Models.Providers[provName]
		if err := validateProvider(provName, pcfg, model); err != nil {
			fmt.Printf("  ✗ %s: %v\n", provName, err)
			var retry string
			retryErr := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(fmt.Sprintf("%s validation failed. What would you like to do?", provName)).
						Options(
							huh.NewOption("Skip (keep config anyway)", "skip"),
							huh.NewOption("Re-enter credentials", "retry"),
						).
						Value(&retry),
				),
			).Run()
			if retryErr != nil {
				return retryErr
			}
			if retry == "retry" {
				pcfg = config.ModelProviderConfig{}
				if err := configureAPIKey(&pcfg, provName, config.ModelProviderConfig{}); err != nil {
					return err
				}
				cfg.Models.Providers[provName] = pcfg
			}
		} else {
			fmt.Printf("  ✓ %s\n", provName)
		}
	}

	// Apply profile tier mappings.
	cfg.Models.Profile = profileName
	cfg.Models.Default = profile.Tiers.Default
	cfg.Models.Complex = profile.Tiers.Complex
	cfg.Models.Fast = profile.Tiers.Fast
	cfg.Models.Nano = profile.Tiers.Nano

	fmt.Println("\n  LLM model configuration complete!")
	return nil
}

// profileTestModels picks one model per provider from the profile for validation.
func profileTestModels(profile config.ModelProfile) map[string]string {
	result := make(map[string]string)
	for _, spec := range []string{profile.Tiers.Default, profile.Tiers.Complex, profile.Tiers.Fast, profile.Tiers.Nano} {
		if spec == "" {
			continue
		}
		parts := strings.SplitN(spec, "/", 2)
		if len(parts) == 2 {
			if _, ok := result[parts[0]]; !ok {
				result[parts[0]] = parts[1]
			}
		}
	}
	return result
}

func setupCustom(cfg *config.Config) error {
	cfg.Models.Profile = ""

	// Step 1: Provider selection.
	var selectedProviders []string
	var providerOptions []huh.Option[string]
	for _, p := range llmProviders {
		opt := huh.NewOption(p.label, p.name)
		if _, exists := cfg.Models.Providers[p.name]; exists {
			opt = opt.Selected(true)
		}
		providerOptions = append(providerOptions, opt)
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which LLM providers would you like to configure?").
				Options(providerOptions...).
				Value(&selectedProviders),
		),
	).Run()
	if err != nil {
		return err
	}

	if len(selectedProviders) == 0 {
		fmt.Println("  No providers selected.")
		return nil
	}

	// Step 2-4: Configure each provider.
	providerModels := make(map[string]string) // name → selected model
	for _, name := range selectedProviders {
		info := findProvider(name)
		if info == nil {
			continue
		}

		existing := cfg.Models.Providers[name]

		pcfg, model, err := configureProvider(info, existing)
		if err != nil {
			return err
		}
		cfg.Models.Providers[name] = pcfg
		providerModels[name] = model
	}

	// Step 5: Tier configuration.
	if err := configureTiers(cfg, providerModels); err != nil {
		return err
	}

	// Validate default model context window for skill support.
	warnDefaultContextWindow(cfg.Models.Default)

	// Step 6: Validation.
	fmt.Println("\n  Validating providers...")
	for _, name := range selectedProviders {
		pcfg := cfg.Models.Providers[name]
		model := providerModels[name]
		if err := validateProvider(name, pcfg, model); err != nil {
			fmt.Printf("  ✗ %s: %v\n", name, err)
			var retry string
			retryErr := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(fmt.Sprintf("%s validation failed. What would you like to do?", name)).
						Options(
							huh.NewOption("Skip (keep config anyway)", "skip"),
							huh.NewOption("Re-enter credentials", "retry"),
						).
						Value(&retry),
				),
			).Run()
			if retryErr != nil {
				return retryErr
			}
			if retry == "retry" {
				info := findProvider(name)
				if info != nil {
					pcfg, model, err := configureProvider(info, config.ModelProviderConfig{})
					if err != nil {
						return err
					}
					cfg.Models.Providers[name] = pcfg
					providerModels[name] = model
				}
			}
		} else {
			fmt.Printf("  ✓ %s\n", name)
		}
	}

	fmt.Println("\n  LLM model configuration complete!")
	return nil
}

func configureProvider(info *providerInfo, existing config.ModelProviderConfig) (config.ModelProviderConfig, string, error) {
	pcfg := existing

	// Auth method selection.
	if info.supportsVAI {
		authMethod := existing.AuthMethod
		if authMethod == "" {
			authMethod = "api_key"
		}
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("How would you like to authenticate with %s?", info.label)).
					Options(
						huh.NewOption("API Key", "api_key"),
						huh.NewOption("Google Vertex AI", "vertex_ai"),
					).
					Value(&authMethod),
			),
		).Run()
		if err != nil {
			return pcfg, "", err
		}
		pcfg.AuthMethod = authMethod
	} else {
		pcfg.AuthMethod = "api_key"
	}

	// Auth config.
	if pcfg.AuthMethod == "vertex_ai" {
		if err := configureVertexAI(&pcfg, existing); err != nil {
			return pcfg, "", err
		}
	} else {
		if err := configureAPIKey(&pcfg, info.name, existing); err != nil {
			return pcfg, "", err
		}
	}

	// Model selection.
	model := defaultModelForList(info.models)
	// Try to pre-select from existing tier configs.
	for _, opt := range info.models {
		if opt.Value == existingModelFor(info.name, model) {
			model = opt.Value
			break
		}
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Which %s model to use?", info.label)).
				Options(info.models...).
				Value(&model),
		),
	).Run()
	if err != nil {
		return pcfg, "", err
	}

	return pcfg, model, nil
}

func configureAPIKey(pcfg *config.ModelProviderConfig, name string, existing config.ModelProviderConfig) error {
	var apiKey string
	placeholder := ""
	if existing.APIKeyRef != "" {
		placeholder = "(leave blank to keep existing)"
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Enter your %s API key", name)).
				Placeholder(placeholder).
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	).Run()
	if err != nil {
		return err
	}

	apiKey = cleanPath(apiKey)
	if apiKey != "" {
		ref := fmt.Sprintf("keychain:obk/%s", name)
		if err := provider.StoreCredential(ref, apiKey); err != nil {
			return fmt.Errorf("store API key: %w", err)
		}
		pcfg.APIKeyRef = ref
		fmt.Printf("  API key stored securely as %s\n", ref)
	}
	return nil
}

func configureVertexAI(pcfg *config.ModelProviderConfig, existing config.ModelProviderConfig) error {
	// Account selection.
	accounts, err := gcloudAccounts()
	if err != nil {
		return fmt.Errorf("list gcloud accounts: %w", err)
	}

	account := existing.VertexAccount
	if len(accounts) > 0 {
		var accountOptions []huh.Option[string]
		for _, a := range accounts {
			accountOptions = append(accountOptions, huh.NewOption(a, a))
		}
		if account == "" {
			account = accounts[0]
		}
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select gcloud account for Vertex AI").
					Description("Account not listed? Run 'gcloud auth login' and restart setup.").
					Options(accountOptions...).
					Value(&account),
			),
		).Run()
	} else {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter gcloud account email for Vertex AI").
					Value(&account),
			),
		).Run()
	}
	if err != nil {
		return err
	}
	pcfg.VertexAccount = account

	// Project selection.
	projects, projErr := gcloudProjects(account)
	if projErr != nil {
		return fmt.Errorf("list gcloud projects: %w", projErr)
	}

	project := existing.VertexProject
	if len(projects) > 0 {
		var projectOptions []huh.Option[string]
		for _, p := range projects {
			projectOptions = append(projectOptions, huh.NewOption(p, p))
		}
		if project == "" {
			project = projects[0]
		}
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select GCP project").
					Options(projectOptions...).
					Value(&project),
			),
		).Run()
	} else {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter GCP project ID").
					Value(&project),
			),
		).Run()
	}
	if err != nil {
		return err
	}
	pcfg.VertexProject = project

	// Region selection.
	region := existing.VertexRegion
	if region == "" {
		region = "us-central1"
	}
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Vertex AI region").
				Options(vertexRegions...).
				Value(&region),
		),
	).Run()
	if err != nil {
		return err
	}
	pcfg.VertexRegion = region

	return nil
}

func configureTiers(cfg *config.Config, providerModels map[string]string) error {
	// Build the list of available model specs.
	var modelOptions []huh.Option[string]
	for name, model := range providerModels {
		spec := name + "/" + model
		modelOptions = append(modelOptions, huh.NewOption(spec, spec))
	}

	if len(modelOptions) == 0 {
		return nil
	}

	// Add "none" option for optional tiers.
	noneOption := huh.NewOption("(none — use default)", "")

	// Default tier.
	defaultSpec := cfg.Models.Default
	if defaultSpec == "" && len(modelOptions) > 0 {
		defaultSpec = modelOptions[0].Value
	}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select default model (required)").
				Options(modelOptions...).
				Value(&defaultSpec),
		),
	).Run()
	if err != nil {
		return err
	}
	cfg.Models.Default = defaultSpec

	if len(modelOptions) > 1 {
		optionalOptions := append([]huh.Option[string]{noneOption}, modelOptions...)

		// Fast tier.
		fastSpec := cfg.Models.Fast
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select fast model (optional)").
					Options(optionalOptions...).
					Value(&fastSpec),
			),
		).Run()
		if err != nil {
			return err
		}
		cfg.Models.Fast = fastSpec

		// Complex tier.
		complexSpec := cfg.Models.Complex
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select complex model (optional)").
					Options(optionalOptions...).
					Value(&complexSpec),
			),
		).Run()
		if err != nil {
			return err
		}
		cfg.Models.Complex = complexSpec

		// Nano tier.
		nanoSpec := cfg.Models.Nano
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select nano model (optional — for trivial tasks)").
					Options(optionalOptions...).
					Value(&nanoSpec),
			),
		).Run()
		if err != nil {
			return err
		}
		cfg.Models.Nano = nanoSpec
	}

	return nil
}

func warnDefaultContextWindow(defaultSpec string) {
	if defaultSpec == "" {
		return
	}
	parts := strings.SplitN(defaultSpec, "/", 2)
	if len(parts) != 2 {
		return
	}
	model := parts[1]
	window := provider.DefaultContextWindow(model)
	if window > 0 && window < 128000 {
		fmt.Printf("  Warning: %s has %dk context. 128k+ recommended for skill support.\n", model, window/1000)
	}
}

func validateProvider(name string, pcfg config.ModelProviderConfig, model string) error {
	var apiKey string
	if pcfg.AuthMethod != "vertex_ai" {
		envVar := provider.ProviderEnvVars[name]
		var err error
		apiKey, err = provider.ResolveAPIKey(pcfg.APIKeyRef, envVar)
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := provider.ListModels(ctx, name, apiKey, pcfg)
	if err != nil {
		return err
	}

	// Cache the results.
	cache := provider.NewModelCache(config.ModelsDir())
	list := &provider.CachedModelList{
		Provider:  name,
		Models:    models,
		FetchedAt: time.Now(),
	}
	_ = cache.Save(name, list)

	return nil
}

func findProvider(name string) *providerInfo {
	for i := range llmProviders {
		if llmProviders[i].name == name {
			return &llmProviders[i]
		}
	}
	return nil
}

func defaultModelForList(models []huh.Option[string]) string {
	if len(models) > 0 {
		return models[0].Value
	}
	return ""
}

// existingModelFor extracts the model from an existing tier spec like "anthropic/claude-sonnet-4-6".
func existingModelFor(providerName, fallback string) string {
	return fallback
}

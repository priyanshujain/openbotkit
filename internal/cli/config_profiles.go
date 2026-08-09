package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var configProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage model profiles",
}

var configProfilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all model profiles (built-in + custom)",
	Example: `  obk config profiles list
  obk config profiles list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		activeProfile := ""
		if cfg.Models != nil {
			activeProfile = cfg.Models.Profile
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			type profileJSON struct {
				Name      string   `json:"name"`
				Label     string   `json:"label"`
				Category  string   `json:"category"`
				Providers []string `json:"providers"`
				Active    bool     `json:"active"`
				Custom    bool     `json:"custom"`
			}
			var profiles []profileJSON
			for _, name := range config.ProfileNames {
				p := config.Profiles[name]
				profiles = append(profiles, profileJSON{
					Name: name, Label: p.Label, Category: p.Category,
					Providers: p.Providers, Active: name == activeProfile,
				})
			}
			if cfg.Models != nil {
				var names []string
				for n := range cfg.Models.CustomProfiles {
					names = append(names, n)
				}
				sort.Strings(names)
				for _, name := range names {
					cp := cfg.Models.CustomProfiles[name]
					label := cp.Label
					if label == "" {
						label = name
					}
					profiles = append(profiles, profileJSON{
						Name: name, Label: label, Category: "custom",
						Providers: cp.Providers, Active: name == activeProfile, Custom: true,
					})
				}
			}
			return json.NewEncoder(os.Stdout).Encode(profiles)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		fmt.Println("Built-in profiles:")
		fmt.Fprintln(w, "  \tNAME\tLABEL\tPROVIDERS")
		for _, name := range config.ProfileNames {
			p := config.Profiles[name]
			marker := " "
			if name == activeProfile {
				marker = "*"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", marker, name, p.Label, strings.Join(p.Providers, ", "))
		}
		w.Flush()

		if cfg.Models != nil && len(cfg.Models.CustomProfiles) > 0 {
			var names []string
			for n := range cfg.Models.CustomProfiles {
				names = append(names, n)
			}
			sort.Strings(names)

			fmt.Println("\nCustom profiles:")
			w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  \tNAME\tLABEL\tPROVIDERS")
			for _, name := range names {
				cp := cfg.Models.CustomProfiles[name]
				marker := " "
				if name == activeProfile {
					marker = "*"
				}
				label := cp.Label
				if label == "" {
					label = name
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", marker, name, label, strings.Join(cp.Providers, ", "))
			}
			w.Flush()
		}

		return nil
	},
}

var configProfilesShowCmd = &cobra.Command{
	Use:   "describe <name>",
	Short: "Describe a model profile",
	Example: `  obk config profiles describe claude-all
  obk config profiles describe my-custom-profile`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check built-in profiles first.
		if p, ok := config.Profiles[name]; ok {
			category := p.Category
			if category == "single" {
				category = "single (1 API key)"
			} else {
				category = fmt.Sprintf("multi (%d API keys)", len(p.Providers))
			}
			printProfileDetails(name, p.Label, p.Description, category, p.Providers, p.Tiers)
			return nil
		}

		// Check custom profiles.
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if cfg.Models != nil {
			if cp, ok := cfg.Models.CustomProfiles[name]; ok {
				label := cp.Label
				if label == "" {
					label = name
				}
				printProfileDetails(name, label, cp.Description, "custom", cp.Providers, cp.Tiers)
				return nil
			}
		}

		return fmt.Errorf("profile %q not found", name)
	},
}

func printProfileDetails(name, label, description, category string, providers []string, tiers config.ProfileTiers) {
	fmt.Printf("Name:        %s\n", name)
	fmt.Printf("Label:       %s\n", label)
	if description != "" {
		fmt.Printf("Description: %s\n", description)
	}
	fmt.Printf("Category:    %s\n", category)
	fmt.Printf("Providers:   %s\n", strings.Join(providers, ", "))
	fmt.Println()
	fmt.Println("Tiers:")
	fmt.Printf("  Default: %s\n", tiers.Default)
	fmt.Printf("  Complex: %s\n", tiers.Complex)
	fmt.Printf("  Fast:    %s\n", tiers.Fast)
	fmt.Printf("  Nano:    %s\n", tiers.Nano)
}

var configProfilesCreateCmd = &cobra.Command{
	Use:     "create",
	Short:   "Create a custom model profile",
	Example: `  obk config profiles create`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Print("\n  Create a custom model profile\n\n")

		// Profile name.
		var name string
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Profile name").
					Validate(func(s string) error {
						if err := config.ValidateProfileName(s); err != nil {
							return err
						}
						if cfg.Models != nil {
							if _, ok := cfg.Models.CustomProfiles[s]; ok {
								return fmt.Errorf("custom profile %q already exists", s)
							}
						}
						return nil
					}).
					Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}

		// Optional label.
		var label string
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Label (optional)").
					Value(&label),
			),
		).Run()
		if err != nil {
			return err
		}

		// Provider selection.
		var selectedProviders []string
		var providerOptions []huh.Option[string]
		for _, p := range llmProviders {
			providerOptions = append(providerOptions, huh.NewOption(p.label, p.name))
		}
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select providers").
					Options(providerOptions...).
					Validate(func(s []string) error {
						if len(s) == 0 {
							return fmt.Errorf("select at least one provider")
						}
						return nil
					}).
					Value(&selectedProviders),
			),
		).Run()
		if err != nil {
			return err
		}

		// Build model options from cache.
		tiers := config.ProfileTiers{}

		tierDefs := []struct {
			name  string
			title string
			dest  *string
		}{
			{"default", "Default tier (main conversation, skill execution)", &tiers.Default},
			{"complex", "Complex tier (strongest reasoning)", &tiers.Complex},
			{"fast", "Fast tier (latency-sensitive tasks)", &tiers.Fast},
			{"nano", "Nano tier (trivial tasks)", &tiers.Nano},
		}

		for _, td := range tierDefs {
			options := buildTierOptions(selectedProviders)
			if len(options) == 0 {
				return fmt.Errorf("no models available for %s tier from selected providers", td.name)
			}
			*td.dest = options[0].Value
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(td.title).
						Options(options...).
						Value(td.dest),
				),
			).Run()
			if err != nil {
				return err
			}
		}

		// Warn if default model has small context window.
		warnDefaultContextWindow(tiers.Default)

		// Save the custom profile.
		if cfg.Models == nil {
			cfg.Models = &config.ModelsConfig{}
		}
		if cfg.Models.CustomProfiles == nil {
			cfg.Models.CustomProfiles = make(map[string]config.CustomProfile)
		}
		cfg.Models.CustomProfiles[name] = config.CustomProfile{
			Label:     label,
			Tiers:     tiers,
			Providers: selectedProviders,
		}
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("\n  Profile %q saved!\n", name)
		fmt.Printf("    Default: %s\n", tiers.Default)
		fmt.Printf("    Complex: %s\n", tiers.Complex)
		fmt.Printf("    Fast:    %s\n", tiers.Fast)
		fmt.Printf("    Nano:    %s\n", tiers.Nano)
		fmt.Println("\n  Activate with: obk setup models")
		return nil
	},
}

// buildTierOptions creates huh.Options from cached models for the given providers.
func buildTierOptions(providers []string) []huh.Option[string] {
	cache := provider.NewModelCache(config.ModelsDir())
	seen := make(map[string]bool)
	var options []huh.Option[string]

	for _, provName := range providers {
		list, err := cache.Load(provName)
		if err != nil || len(list.Models) == 0 {
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
			options = append(options, huh.NewOption(label+" ("+provName+")", spec))
		}
	}
	return options
}

var configProfilesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a custom model profile",
	Example: `  obk config profiles delete my-custom-profile
  obk config profiles delete my-custom-profile --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if _, ok := config.Profiles[name]; ok {
			return fmt.Errorf("cannot delete built-in profile %q", name)
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("About to delete profile %q. Continue? (y/N): ", name)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.Models == nil || cfg.Models.CustomProfiles == nil {
			return fmt.Errorf("custom profile %q not found", name)
		}
		if _, ok := cfg.Models.CustomProfiles[name]; !ok {
			return fmt.Errorf("custom profile %q not found", name)
		}

		delete(cfg.Models.CustomProfiles, name)
		if len(cfg.Models.CustomProfiles) == 0 {
			cfg.Models.CustomProfiles = nil
		}

		if cfg.Models.Profile == name {
			cfg.Models.Profile = ""
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Deleted custom profile %q\n", name)
		return nil
	},
}

func init() {
	configProfilesListCmd.Flags().Bool("json", false, "Output as JSON")
	configProfilesCmd.AddCommand(configProfilesListCmd)
	configProfilesCmd.AddCommand(configProfilesShowCmd)
	configProfilesCmd.AddCommand(configProfilesCreateCmd)
	configProfilesDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	configProfilesCmd.AddCommand(configProfilesDeleteCmd)
	configCmd.AddCommand(configProfilesCmd)
}

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage obk configuration",
}

var configInitCmd = &cobra.Command{
	Use:     "init",
	Short:   "Create default configuration file",
	Example: `  obk config init`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.FilePath()
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s", path)
		}

		cfg := config.Default()
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Config created at %s\n", path)
		fmt.Println("\nNext steps:")
		fmt.Printf("  1. Place your Google OAuth credentials at %s\n", cfg.GoogleCredentialsFile())
		fmt.Println("  2. Run: obk gmail auth login --scopes gmail.readonly")
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all configuration properties",
	Example: `  obk config list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}

		fmt.Print(string(data))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Example: `  obk config set timezone America/New_York
  obk config set gmail.storage.driver postgres`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		key := args[0]
		value := args[1]

		// Special validation for timezone.
		if key == "timezone" {
			if _, err := time.LoadLocation(value); err != nil {
				return fmt.Errorf("invalid timezone %q: %w", value, err)
			}
		}

		if err := config.SetByPath(cfg, key, value); err != nil {
			return err
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:     "path",
	Short:   "Print the configuration directory path",
	Example: `  obk config path`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Dir())
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)

	rootCmd.AddCommand(configCmd)
}

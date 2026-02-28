package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage obk configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default configuration file",
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
		fmt.Printf("  1. Place your Google OAuth credentials at %s\n", cfg.Gmail.CredentialsFile)
		fmt.Println("  2. Run: obk gmail auth login")
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print resolved configuration",
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
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		key := args[0]
		value := args[1]

		switch strings.ToLower(key) {
		case "gmail.storage.driver":
			if value != "sqlite" && value != "postgres" {
				return fmt.Errorf("invalid driver: %s (must be 'sqlite' or 'postgres')", value)
			}
			cfg.Gmail.Storage.Driver = value
		case "gmail.storage.dsn":
			cfg.Gmail.Storage.DSN = value
		case "gmail.credentials_file":
			cfg.Gmail.CredentialsFile = value
		case "gmail.download_attachments":
			cfg.Gmail.DownloadAttachments = value == "true"
		default:
			return fmt.Errorf("unknown config key: %s", key)
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the configuration directory path",
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

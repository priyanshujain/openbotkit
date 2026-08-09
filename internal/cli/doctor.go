package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/daemon/service"
	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/provider"
)

type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of your obk installation",
	Example: `  obk doctor
  obk doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, cfgErr := config.Load()

		var results []checkResult
		results = append(results, checkConfig(cfg, cfgErr)...)
		if cfgErr == nil {
			results = append(results, checkAPIKeys(cfg)...)
			results = append(results, checkGoogleOAuth(cfg)...)
			results = append(results, checkWhatsAppSession(cfg)...)
			results = append(results, checkTelegramSession(cfg)...)
			results = append(results, checkDatabases(cfg)...)
		}
		results = append(results, checkServices()...)
		results = append(results, checkSkills())

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(results)
		}

		for _, r := range results {
			fmt.Fprintf(os.Stdout, "%-20s %-6s %s\n", r.Name, r.Status, r.Detail)
		}
		return nil
	},
}

func checkConfig(cfg *config.Config, cfgErr error) []checkResult {
	var results []checkResult

	_, err := os.Stat(config.FilePath())
	if err != nil {
		results = append(results, checkResult{"Config file", "FAIL", "not found: " + config.FilePath()})
		return results
	}
	results = append(results, checkResult{"Config file", "OK", config.FilePath()})

	if cfgErr != nil {
		results = append(results, checkResult{"Config parse", "FAIL", cfgErr.Error()})
		return results
	}

	if err := cfg.RequireSetup(); err != nil {
		results = append(results, checkResult{"LLM models", "FAIL", "run 'obk setup' to configure models"})
	} else {
		results = append(results, checkResult{"LLM models", "OK", "default: " + cfg.Models.Default})
	}

	return results
}

func checkAPIKeys(cfg *config.Config) []checkResult {
	if cfg.Models == nil {
		return nil
	}

	var results []checkResult
	for name := range cfg.Models.Providers {
		providerCfg := cfg.Models.Providers[name]
		if providerCfg.AuthMethod == "vertex_ai" {
			results = append(results, checkResult{"API key (" + name + ")", "OK", "vertex_ai auth"})
			continue
		}
		envVar := provider.ProviderEnvVars[name]
		_, err := provider.ResolveAPIKey(providerCfg.APIKeyRef, envVar)
		if err != nil {
			results = append(results, checkResult{"API key (" + name + ")", "WARN", "not found"})
		} else {
			results = append(results, checkResult{"API key (" + name + ")", "OK", "resolved"})
		}
	}
	return results
}

func checkGoogleOAuth(cfg *config.Config) []checkResult {
	path := cfg.GoogleTokenDBPath()
	if _, err := os.Stat(path); err != nil {
		return []checkResult{{"Google OAuth", "WARN", "no token DB"}}
	}
	return []checkResult{{"Google OAuth", "OK", path}}
}

func checkWhatsAppSession(cfg *config.Config) []checkResult {
	path := cfg.WhatsAppSessionDBPath()
	if _, err := os.Stat(path); err != nil {
		return []checkResult{{"WhatsApp session", "WARN", "no session DB"}}
	}
	return []checkResult{{"WhatsApp session", "OK", path}}
}

func checkTelegramSession(cfg *config.Config) []checkResult {
	path := cfg.TelegramSessionPath()
	if _, err := os.Stat(path); err != nil {
		return []checkResult{{"Telegram session", "WARN", "no session file"}}
	}
	return []checkResult{{"Telegram session", "OK", path}}
}

type dbCheck struct {
	name string
	path string
}

func checkDatabases(cfg *config.Config) []checkResult {
	dbs := []dbCheck{
		{"Gmail DB", cfg.GmailDataDSN()},
		{"WhatsApp DB", cfg.WhatsAppDataDSN()},
		{"Telegram DB", cfg.TelegramDataDSN()},
		{"History DB", cfg.HistoryDataDSN()},
		{"UserMemory DB", cfg.UserMemoryDataDSN()},
		{"AppleNotes DB", cfg.AppleNotesDataDSN()},
		{"Contacts DB", cfg.ContactsDataDSN()},
		{"WebSearch DB", cfg.WebSearchDataDSN()},
		{"iMessage DB", cfg.IMessageDataDSN()},
		{"Scheduler DB", cfg.SchedulerDataDSN()},
		{"Audit DB", config.AuditDBPath()},
		{"Jobs DB", cfg.JobsDBDSN()},
	}

	var results []checkResult
	for _, db := range dbs {
		if _, err := os.Stat(db.path); err != nil {
			results = append(results, checkResult{db.name, "WARN", "not found"})
		} else {
			results = append(results, checkResult{db.name, "OK", db.path})
		}
	}
	return results
}

func checkServices() []checkResult {
	var results []checkResult
	for _, name := range []string{"daemon", "server"} {
		mgr, err := service.NewManager(name)
		if err != nil {
			results = append(results, checkResult{name + " service", "WARN", err.Error()})
			continue
		}
		status, err := mgr.Status()
		if err != nil {
			results = append(results, checkResult{name + " service", "WARN", err.Error()})
			continue
		}
		results = append(results, checkResult{name + " service", "OK", status})
	}
	return results
}

func checkSkills() checkResult {
	idx, err := skills.LoadIndex()
	if err != nil {
		return checkResult{"Skills", "WARN", err.Error()}
	}
	if len(idx.Skills) == 0 {
		return checkResult{"Skills", "WARN", "no skills installed"}
	}
	return checkResult{"Skills", "OK", fmt.Sprintf("%d skills", len(idx.Skills))}
}

func init() {
	doctorCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(doctorCmd)
}

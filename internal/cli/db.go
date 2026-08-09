package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/73ai/openbotkit/config"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db <source> <sql>",
	Short: "Run a SQL query against a data source",
	Long:  "Execute a read-only SQL query against one of the data sources (gmail, whatsapp, telegram, history, user_memory, applenotes).",
	Example: `  obk db gmail "SELECT count(*) FROM emails"
  obk db whatsapp "SELECT * FROM messages ORDER BY timestamp DESC LIMIT 5"
  obk db telegram "SELECT count(*) FROM telegram_messages"
  obk db user_memory "SELECT * FROM memories"`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		query := args[1]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.IsRemote() {
			return dbRemote(cfg, source, query)
		}

		return dbLocal(cfg, source, query)
	},
}

func dbLocal(cfg *config.Config, source, query string) error {
	dsn, err := cfg.SourceDataDSN(source)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dsn); err != nil {
		return fmt.Errorf("database not found: %s", dsn)
	}

	sqlite3, err := exec.LookPath("sqlite3")
	if err != nil {
		return fmt.Errorf("sqlite3 not found in PATH — install it to use this command")
	}

	c := exec.Command(sqlite3, "-header", "-separator", "\t", dsn, query)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func dbRemote(cfg *config.Config, source, query string) error {
	client, err := remoteClient(cfg)
	if err != nil {
		return err
	}

	resp, err := client.DB(source, query)
	if err != nil {
		return err
	}

	if len(resp.Columns) == 0 {
		return nil
	}

	// Print header
	for i, col := range resp.Columns {
		if i > 0 {
			fmt.Print("\t")
		}
		fmt.Print(col)
	}
	fmt.Println()

	// Print rows
	for _, row := range resp.Rows {
		for i, val := range row {
			if i > 0 {
				fmt.Print("\t")
			}
			fmt.Print(val)
		}
		fmt.Println()
	}

	return nil
}

func init() {
	rootCmd.AddCommand(dbCmd)
}

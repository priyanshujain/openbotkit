package memory

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/priyanshujain/openbotkit/config"
	memorysrc "github.com/priyanshujain/openbotkit/source/memory"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture a conversation from a Claude Code transcript",
	Long:  "Reads capture input as JSON from stdin. Designed to be called by Claude Code hooks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var input memorysrc.CaptureInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return fmt.Errorf("decode stdin: %w", err)
		}

		if input.SessionID == "" || input.TranscriptPath == "" {
			return fmt.Errorf("session_id and transcript_path are required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := config.EnsureSourceDir("memory"); err != nil {
			return fmt.Errorf("ensure memory dir: %w", err)
		}

		dsn := cfg.MemoryDataDSN()
		db, err := store.Open(store.Config{
			Driver: cfg.Memory.Storage.Driver,
			DSN:    dsn,
		})
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		if err := memorysrc.Capture(db, input); err != nil {
			return fmt.Errorf("capture: %w", err)
		}

		return nil
	},
}

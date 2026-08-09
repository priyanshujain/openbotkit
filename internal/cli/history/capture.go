package history

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/73ai/openbotkit/config"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/spf13/cobra"
)

var captureCmd = &cobra.Command{
	Use:     "capture",
	Short:   "Capture a conversation from a Claude Code transcript",
	Long:    "Reads capture input as JSON from stdin. Designed to be called by Claude Code hooks.",
	Example: `  echo '{"session_id":"abc","transcript_path":"/tmp/t.json"}' | obk history capture`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input historysrc.CaptureInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return fmt.Errorf("decode stdin: %w", err)
		}

		if input.SessionID == "" || input.TranscriptPath == "" {
			return fmt.Errorf("session_id and transcript_path are required")
		}

		dir := config.HistoryDir()
		if err := historysrc.EnsureDir(dir); err != nil {
			return fmt.Errorf("ensure history dir: %w", err)
		}

		s := historysrc.NewStore(dir)
		if err := historysrc.Capture(s, input); err != nil {
			return fmt.Errorf("capture: %w", err)
		}

		return nil
	},
}

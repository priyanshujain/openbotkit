package cli

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/daemon"
)

var daemonMode string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the background daemon for continuous syncing and job processing",
	Long:  "Starts the OpenBotKit daemon which runs periodic Gmail syncs, processes scheduled jobs, and maintains a WhatsApp connection.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		mode := daemon.ParseMode(daemonMode)

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		d := daemon.New(cfg, mode)
		return d.Run(ctx)
	},
}

func init() {
	daemonCmd.Flags().StringVar(&daemonMode, "mode", "", "daemon mode: standalone or worker (default: standalone, env: OBK_MODE)")
}

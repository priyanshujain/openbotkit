package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/priyanshujain/openbotkit/daemon/service"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the obk background service",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install obk as a system service",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		cfg, err := service.DefaultConfig()
		if err != nil {
			return err
		}

		if err := mgr.Install(cfg); err != nil {
			return fmt.Errorf("install service: %w", err)
		}

		fmt.Printf("service installed (platform: %s)\n", service.DetectPlatform())
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the obk system service",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		if err := mgr.Uninstall(); err != nil {
			return fmt.Errorf("uninstall service: %w", err)
		}

		fmt.Println("service uninstalled")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the obk service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := service.NewManager()
		if err != nil {
			return err
		}

		status, err := mgr.Status()
		if err != nil {
			return fmt.Errorf("check status: %w", err)
		}

		fmt.Printf("service: %s\n", status)
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
}

package cli

import (
	"fmt"
	"os/exec"

	"github.com/priyanshujain/openbotkit/config"
	"github.com/priyanshujain/openbotkit/internal/skills"
	"github.com/spf13/cobra"
)

var updateSkillsOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update obk binary and reinstall skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !updateSkillsOnly {
			fmt.Println("Updating obk binary...")
			install := exec.Command("go", "install", "github.com/priyanshujain/openbotkit@latest")
			install.Stdout = cmd.OutOrStdout()
			install.Stderr = cmd.ErrOrStderr()
			if err := install.Run(); err != nil {
				fmt.Printf("  warning: binary update failed: %v\n", err)
			} else {
				fmt.Println("  obk binary updated")
			}
		}

		fmt.Println("Updating skills...")
		result, err := skills.Install(cfg)
		if err != nil {
			return fmt.Errorf("install skills: %w", err)
		}

		for _, name := range result.Installed {
			fmt.Printf("  + %s\n", name)
		}
		for _, name := range result.Skipped {
			fmt.Printf("  - %s (skipped)\n", name)
		}
		for _, name := range result.Removed {
			fmt.Printf("  x %s (removed)\n", name)
		}
		fmt.Printf("  %d skills installed, manifest updated\n", len(result.Installed))

		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateSkillsOnly, "skills-only", false, "Only update skills, skip binary self-update")
	rootCmd.AddCommand(updateCmd)
}

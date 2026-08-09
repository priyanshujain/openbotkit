package cli

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/73ai/openbotkit/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills",
}

var skillsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all installed skills",
	Example: `  obk skills list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := skills.ListSkills()
		if err != nil {
			return err
		}

		sort.Slice(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSOURCE\tDESCRIPTION")
		for _, s := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Source, s.Description)
		}
		return w.Flush()
	},
}

var skillsShowCmd = &cobra.Command{
	Use:     "show <name>",
	Short:   "Show a skill's content",
	Example: `  obk skills show email-read`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillMD, refMD, entry, err := skills.GetSkill(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Source: %s\n", entry.Source)
		if entry.Repo != "" {
			fmt.Printf("Repo: %s\n", entry.Repo)
		}
		fmt.Println("\n--- SKILL.md ---")
		fmt.Print(skillMD)
		if refMD != "" {
			fmt.Println("\n--- REFERENCE.md ---")
			fmt.Print(refMD)
		}
		return nil
	},
}

var skillsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a custom skill from files",
	Example: `  obk skills create my-skill --skill-file /path/SKILL.md --ref-file /path/REFERENCE.md
  obk skills create my-skill --skill-file /path/SKILL.md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillFile, _ := cmd.Flags().GetString("skill-file")
		refFile, _ := cmd.Flags().GetString("ref-file")

		if skillFile == "" {
			return fmt.Errorf("--skill-file is required")
		}

		skillContent, err := os.ReadFile(skillFile)
		if err != nil {
			return fmt.Errorf("read skill file: %w", err)
		}

		var refContent string
		if refFile != "" {
			data, err := os.ReadFile(refFile)
			if err != nil {
				return fmt.Errorf("read ref file: %w", err)
			}
			refContent = string(data)
		}

		if err := skills.InstallCustomSkill(args[0], string(skillContent), refContent); err != nil {
			return err
		}
		fmt.Printf("Created skill %q\n", args[0])
		return nil
	},
}

var skillsUpdateCmd = &cobra.Command{
	Use:     "update <name>",
	Short:   "Update an existing custom/external skill",
	Example: `  obk skills update my-skill --skill-file /path/SKILL.md --ref-file /path/REFERENCE.md`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillFile, _ := cmd.Flags().GetString("skill-file")
		refFile, _ := cmd.Flags().GetString("ref-file")

		if skillFile == "" && refFile == "" {
			return fmt.Errorf("at least one of --skill-file or --ref-file is required")
		}

		var skillContent, refContent string
		if skillFile != "" {
			data, err := os.ReadFile(skillFile)
			if err != nil {
				return fmt.Errorf("read skill file: %w", err)
			}
			skillContent = string(data)
		}
		if refFile != "" {
			data, err := os.ReadFile(refFile)
			if err != nil {
				return fmt.Errorf("read ref file: %w", err)
			}
			refContent = string(data)
		}

		if err := skills.UpdateCustomSkill(args[0], skillContent, refContent); err != nil {
			return err
		}
		fmt.Printf("Updated skill %q\n", args[0])
		return nil
	},
}

var skillsRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a custom/external skill",
	Example: `  obk skills remove my-skill`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := skills.RemoveCustomSkill(args[0]); err != nil {
			return err
		}
		fmt.Printf("Removed skill %q\n", args[0])
		return nil
	},
}

func init() {
	skillsCreateCmd.Flags().String("skill-file", "", "Path to SKILL.md file")
	skillsCreateCmd.Flags().String("ref-file", "", "Path to REFERENCE.md file")

	skillsUpdateCmd.Flags().String("skill-file", "", "Path to updated SKILL.md file")
	skillsUpdateCmd.Flags().String("ref-file", "", "Path to updated REFERENCE.md file")

	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
	skillsCmd.AddCommand(skillsCreateCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsCmd.AddCommand(skillsRemoveCmd)

	rootCmd.AddCommand(skillsCmd)
}

package memory

import (
	"context"
	"fmt"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	historysrc "github.com/73ai/openbotkit/service/history"
	"github.com/73ai/openbotkit/service/memory"
	"github.com/spf13/cobra"

	_ "github.com/73ai/openbotkit/provider/anthropic"
	_ "github.com/73ai/openbotkit/provider/gemini"
	_ "github.com/73ai/openbotkit/provider/openai"
)

var extractLast int

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract personal facts from conversation history",
	Example: `  obk memory extract
  obk memory extract --last 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.IsRemote() {
			client, err := newRemoteClient(cfg)
			if err != nil {
				return err
			}
			result, err := client.MemoryExtract(extractLast)
			if err != nil {
				return fmt.Errorf("extract: %w", err)
			}
			fmt.Printf("Added %d, Updated %d, Deleted %d, Skipped %d\n",
				result.Added, result.Updated, result.Deleted, result.Skipped)
			return nil
		}

		if err := cfg.RequireSetup(); err != nil {
			return err
		}

		// Load recent messages from history.
		histDir := config.HistoryDir()
		if err := historysrc.EnsureDir(histDir); err != nil {
			return fmt.Errorf("ensure history dir: %w", err)
		}
		histStore := historysrc.NewStore(histDir)
		messages, err := histStore.LoadRecentUserMessages(extractLast)
		if err != nil {
			return fmt.Errorf("load messages: %w", err)
		}

		if len(messages) == 0 {
			fmt.Println("No messages found in history.")
			return nil
		}

		// Open user_memory store.
		dir := config.UserMemoryDir()
		if err := memory.EnsureDir(dir); err != nil {
			return fmt.Errorf("ensure user_memory dir: %w", err)
		}
		memStore := memory.NewStore(dir)

		// Create LLM client.
		llm, err := buildLLM(cfg)
		if err != nil {
			return fmt.Errorf("build LLM: %w", err)
		}

		ctx := context.Background()

		// Extract candidate facts.
		candidates, err := memory.Extract(ctx, llm, messages)
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}

		if len(candidates) == 0 {
			fmt.Println("No personal facts found.")
			return nil
		}

		// Reconcile with existing memories.
		result, err := memory.Reconcile(ctx, memStore, llm, candidates)
		if err != nil {
			return fmt.Errorf("reconcile: %w", err)
		}

		fmt.Printf("Added %d, Updated %d, Deleted %d, Skipped %d\n",
			result.Added, result.Updated, result.Deleted, result.Skipped)
		return nil
	},
}

func buildLLM(cfg *config.Config) (memory.LLM, error) {
	registry, err := provider.NewRegistry(cfg.Models)
	if err != nil {
		return nil, fmt.Errorf("create provider registry: %w", err)
	}
	router := provider.NewRouter(registry, cfg.Models)
	return &memory.RouterLLM{Router: router, Tier: provider.TierFast}, nil
}

func init() {
	Cmd.AddCommand(extractCmd)
	extractCmd.Flags().IntVar(&extractLast, "last", 1, "number of recent sessions to extract from")
}

package telegram

import (
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/platform"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Backfill Telegram history, then optionally follow live updates",
	Example: `  obk telegram sync
  obk telegram sync --since 30d
  obk telegram sync --follow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if err := requireSession(cfg); err != nil {
			return err
		}

		since, _ := cmd.Flags().GetString("since")
		full, _ := cmd.Flags().GetBool("full")
		follow, _ := cmd.Flags().GetBool("follow")
		limit, _ := cmd.Flags().GetInt("limit")

		window, err := parseSince(since, cfg.TelegramBackfillDays())
		if err != nil {
			return err
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client, err := newClient(cfg, db)
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(context.Background(), platform.ShutdownSignals...)
		defer stop()

		if err := config.LinkSource("telegram"); err != nil {
			return fmt.Errorf("link source: %w", err)
		}

		opts := tgsrc.BackfillOptions{Since: window, PerChatLimit: limit, Full: full}

		err = client.Run(ctx, func(ctx context.Context) error {
			fmt.Println("Backfilling Telegram history...")
			result, err := tgsrc.Backfill(ctx, tgsrc.NewAPIFetcher(client.API()), db, opts)
			if err != nil {
				return err
			}
			fmt.Printf("Backfill complete: %d chats, %d messages", result.Chats, result.Messages)
			if result.Errors > 0 {
				fmt.Printf(", %d errors", result.Errors)
			}
			fmt.Println()
			return nil
		})
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		if !follow {
			return nil
		}

		fmt.Println("Following live updates (Ctrl+C to stop)...")
		if err := tgsrc.Live(ctx, client, db, tgsrc.LiveOptions{}); err != nil && ctx.Err() == nil {
			return fmt.Errorf("live sync failed: %w", err)
		}
		return nil
	},
}

// parseSince accepts "30d", "12h" or a YYYY-MM-DD date. An empty value falls
// back to the configured backfill window.
func parseSince(since string, defaultDays int) (time.Time, error) {
	since = strings.TrimSpace(since)
	if since == "" {
		return time.Now().UTC().AddDate(0, 0, -defaultDays), nil
	}
	if since == "all" {
		return time.Time{}, nil
	}

	if n, ok := strings.CutSuffix(since, "d"); ok {
		days, err := strconv.Atoi(n)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q", since)
		}
		// A negative window lands in the future, so every message is older than
		// it and the backfill stops instantly reporting nothing.
		if days < 0 {
			return time.Time{}, fmt.Errorf("--since %q is in the future; drop the minus sign", since)
		}
		return time.Now().UTC().AddDate(0, 0, -days), nil
	}

	if d, err := time.ParseDuration(since); err == nil {
		if d < 0 {
			return time.Time{}, fmt.Errorf("--since %q is in the future; drop the minus sign", since)
		}
		return time.Now().UTC().Add(-d), nil
	}

	t, err := time.Parse("2006-01-02", since)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q (want 30d, 12h, YYYY-MM-DD or all)", since)
	}
	return t.UTC(), nil
}

func init() {
	syncCmd.Flags().String("since", "", "How far back to backfill: 30d, 12h, YYYY-MM-DD or all")
	syncCmd.Flags().Bool("full", false, "Ignore stored progress and restart from the newest message")
	syncCmd.Flags().Bool("follow", false, "Keep running and stream live updates after the backfill")
	syncCmd.Flags().Int("limit", 0, "Maximum messages to store per chat (0 for no limit)")
}

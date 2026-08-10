package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
	usagesrc "github.com/73ai/openbotkit/service/usage"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show LLM token usage and estimated costs",
	Example: `  obk usage
  obk usage --since 2025-03-01 --json`,
	RunE: runUsageDaily,
}

var usageDailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Show daily usage breakdown",
	Example: `  obk usage daily --since 2025-03-01
  obk usage daily --model claude-sonnet-4-6 --json`,
	RunE: runUsageDaily,
}

var usageMonthlyCmd = &cobra.Command{
	Use:   "monthly",
	Short: "Show monthly usage breakdown",
	Example: `  obk usage monthly
  obk usage monthly --json`,
	RunE: runUsageMonthly,
}

var (
	usageSince string
	usageUntil string
	usageModel string
	usageJSON  bool
)

func init() {
	for _, cmd := range []*cobra.Command{usageCmd, usageDailyCmd, usageMonthlyCmd} {
		cmd.Flags().StringVar(&usageSince, "since", "", "Start date (YYYY-MM-DD)")
		cmd.Flags().StringVar(&usageUntil, "until", "", "End date (YYYY-MM-DD)")
		cmd.Flags().StringVar(&usageModel, "model", "", "Filter by model name")
		cmd.Flags().BoolVar(&usageJSON, "json", false, "Output as JSON")
	}
	usageCmd.AddCommand(usageDailyCmd)
	usageCmd.AddCommand(usageMonthlyCmd)
	rootCmd.AddCommand(usageCmd)
}

func runUsageDaily(cmd *cobra.Command, args []string) error {
	return runUsageQuery("daily")
}

func runUsageMonthly(cmd *cobra.Command, args []string) error {
	return runUsageQuery("monthly")
}

func runUsageQuery(groupBy string) error {
	_, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	path := config.UsageJSONLPath()
	if err := usagesrc.EnsureDir(path); err != nil {
		return fmt.Errorf("ensure usage dir: %w", err)
	}

	opts := usagesrc.QueryOpts{
		GroupBy: groupBy,
		Model:   usageModel,
	}

	if usageSince != "" {
		t, err := time.Parse("2006-01-02", usageSince)
		if err != nil {
			return fmt.Errorf("invalid --since date: %w", err)
		}
		opts.Since = &t
	} else {
		t := time.Now().AddDate(0, 0, -30)
		opts.Since = &t
	}

	if usageUntil != "" {
		t, err := time.Parse("2006-01-02", usageUntil)
		if err != nil {
			return fmt.Errorf("invalid --until date: %w", err)
		}
		opts.Until = &t
	}

	results, err := usagesrc.Query(path, opts)
	if err != nil {
		return fmt.Errorf("query usage: %w", err)
	}

	if usageJSON {
		return json.NewEncoder(os.Stdout).Encode(results)
	}

	if len(results) == 0 {
		fmt.Println("No usage data found for the selected period.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Date\tModel\tInput\tOutput\tCache Read\tCache Write\tCalls\tEst. Cost\n")
	fmt.Fprintf(w, "----\t-----\t-----\t------\t----------\t-----------\t-----\t---------\n")

	var totalCost float64
	for _, r := range results {
		cost := estimateCost(r)
		totalCost += cost
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t$%.2f\n",
			r.Date, r.Model,
			formatTokens(r.InputTokens), formatTokens(r.OutputTokens),
			formatTokens(r.CacheReadTokens), formatTokens(r.CacheWriteTokens),
			r.CallCount, cost)
	}
	fmt.Fprintf(w, "TOTAL\t\t\t\t\t\t\t$%.2f\n", totalCost)
	w.Flush()

	return nil
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func estimateCost(r usagesrc.AggregatedUsage) float64 {
	return provider.EstimateCost(r.Model, provider.Usage{
		InputTokens:      int(r.InputTokens),
		OutputTokens:     int(r.OutputTokens),
		CacheReadTokens:  int(r.CacheReadTokens),
		CacheWriteTokens: int(r.CacheWriteTokens),
	})
}

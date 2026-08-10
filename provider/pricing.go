package provider

import "math"

// ModelPricing maps model names to per-million-token rates:
// [input, output, cache_read]. Cache writes are charged at input rate.
var ModelPricing = map[string][3]float64{
	"claude-sonnet-4-6":              {3.0, 15.0, 0.30},
	"claude-sonnet-4-20250514":       {3.0, 15.0, 0.30},
	"claude-haiku-4-5":               {0.80, 4.0, 0.08},
	"claude-opus-4-6":                {15.0, 75.0, 1.50},
	"gpt-4o":                         {2.50, 10.0, 1.25},
	"gpt-4o-mini":                    {0.15, 0.60, 0.075},
	"gpt-4.1":                        {2.00, 8.00, 0.50},
	"gpt-4.1-mini":                   {0.40, 1.60, 0.10},
	"gpt-4.1-nano":                   {0.10, 0.40, 0.025},
	"gemini-2.5-pro":                 {1.25, 10.0, 0.3125},
	"gemini-2.5-flash":               {0.15, 0.60, 0.0375},
	"gemini-2.5-flash-lite":          {0, 0, 0},
	"qwen-3-235b-a22b-instruct-2507": {0, 0, 0},
	"llama3.1-8b":                    {0, 0, 0},
}

// EstimateCost calculates the estimated cost for a model usage.
// Returns 0 for unknown models. Uses prefix matching for versioned names.
func EstimateCost(model string, usage Usage) float64 {
	pricing, ok := ModelPricing[model]
	if !ok {
		bestLen := 0
		for prefix, p := range ModelPricing {
			if len(prefix) > bestLen && len(model) >= len(prefix) && model[:len(prefix)] == prefix {
				pricing = p
				bestLen = len(prefix)
				ok = true
			}
		}
	}
	if !ok {
		return 0
	}

	inputRate := pricing[0] / 1_000_000
	outputRate := pricing[1] / 1_000_000
	cacheReadRate := pricing[2] / 1_000_000

	nonCachedInput := int64(usage.InputTokens) - int64(usage.CacheReadTokens)
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}

	cost := float64(nonCachedInput)*inputRate +
		float64(usage.OutputTokens)*outputRate +
		float64(usage.CacheReadTokens)*cacheReadRate +
		float64(usage.CacheWriteTokens)*inputRate

	return math.Round(cost*100) / 100
}

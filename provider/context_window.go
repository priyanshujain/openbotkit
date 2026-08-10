package provider

import "strings"

var contextWindows = map[string]int{
	"claude-opus-4-6":                200000,
	"claude-sonnet-4-6":              200000,
	"claude-haiku-4-5":               200000,
	"gpt-4o":                         128000,
	"gpt-4o-mini":                    128000,
	"gemini-2.5-pro":                 1048576,
	"gemini-2.5-flash":               1048576,
	"gemini-2.0-flash":               1048576,
	"gemini-2.5-flash-lite":          1048576,
	"mistral-medium-3.1":             131072,
	"qwen-3-235b-a22b-instruct-2507": 65536,
	"llama3.1-8b":                    131072,
	"llama-3.1-8b-instant":           131072,
	"llama-3.3-70b-versatile":        131072,
	"llama-4-scout-17b-16e":          131072,
	"llama-4-maverick-17b-128e":      131072,
	"glm-4.5-flash":                  128000,
	"glm-4.5-air":                    128000,
	"glm-4.5":                        128000,
	"glm-4.6":                        128000,
	"glm-4.7":                        200000,
	"glm-4.7-flash":                  200000,
	"glm-4.7-flashx":                 200000,
	"glm-5":                          200000,
	"glm-5-turbo":                    200000,
}

// DefaultContextWindow returns the context window size for a model.
// It first tries an exact match, then the last path segment (for nested
// IDs like "anthropic/claude-haiku-4-5"), then prefix matching so that
// versioned model IDs like "claude-opus-4-6-20260301" resolve.
func DefaultContextWindow(model string) int {
	if w, ok := contextWindows[model]; ok {
		return w
	}
	// Try last path segment for nested model IDs (e.g. OpenRouter).
	if i := strings.LastIndex(model, "/"); i >= 0 {
		base := model[i+1:]
		if w, ok := contextWindows[base]; ok {
			return w
		}
	}
	for prefix, w := range contextWindows {
		if strings.HasPrefix(model, prefix) {
			return w
		}
	}
	return 0
}

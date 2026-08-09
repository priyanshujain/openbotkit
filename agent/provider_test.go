package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/73ai/openbotkit/agent"
	"github.com/73ai/openbotkit/agent/tools"
	"github.com/73ai/openbotkit/internal/testutil"
	"github.com/73ai/openbotkit/provider"
	"github.com/73ai/openbotkit/provider/anthropic"
	"github.com/73ai/openbotkit/provider/gemini"
	"github.com/73ai/openbotkit/provider/openai"
)

// providerTestCase holds a provider instance and model name for provider conformance tests.
type providerTestCase struct {
	name     string
	provider provider.Provider
	model    string
}

// availableProviders returns provider instances for all API keys that are set.
func availableProviders(t *testing.T) []providerTestCase {
	t.Helper()
	testutil.LoadEnv(t)
	var providers []providerTestCase

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		providers = append(providers, providerTestCase{
			name:     "anthropic",
			provider: anthropic.New(key),
			model:    "claude-sonnet-4-6",
		})
	}
	if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
		region := os.Getenv("GOOGLE_CLOUD_REGION")
		if region == "" {
			region = "us-east5"
		}
		model := os.Getenv("VERTEX_CLAUDE_MODEL")
		if model == "" {
			model = "claude-sonnet-4@20250514"
		}
		account := os.Getenv("GOOGLE_CLOUD_ACCOUNT")
		providers = append(providers, providerTestCase{
			name:     "anthropic-vertex",
			provider: anthropic.New("", anthropic.WithVertexAI(project, region), anthropic.WithTokenSource(provider.GcloudTokenSource(account))),
			model:    model,
		})
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		providers = append(providers, providerTestCase{
			name:     "openai",
			provider: openai.New(key),
			model:    "gpt-4o-mini",
		})
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		providers = append(providers, providerTestCase{
			name:     "gemini",
			provider: gemini.New(key),
			model:    "gemini-2.5-flash",
		})
	}
	if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
		region := os.Getenv("GOOGLE_CLOUD_REGION")
		if region == "" {
			region = "us-east5"
		}
		account := os.Getenv("GOOGLE_CLOUD_ACCOUNT")
		providers = append(providers, providerTestCase{
			name:     "gemini-vertex",
			provider: gemini.New("", gemini.WithVertexAI(project, region), gemini.WithTokenSource(provider.GcloudTokenSource(account))),
			model:    "gemini-2.5-flash",
		})
	}

	if len(providers) == 0 {
		t.Skip("no API keys set (ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY) — skipping provider tests")
	}
	return providers
}

// TestProvider_AgentToolExecution verifies each provider can drive the agent
// loop: user request → tool call → tool execution → text response.
func TestProvider_AgentToolExecution(t *testing.T) {
	for _, tc := range availableProviders(t) {
		t.Run(tc.name, func(t *testing.T) {
			reg := tools.NewRegistry()
			reg.Register(tools.NewBashTool(10 * time.Second))

			a := agent.New(tc.provider, tc.model, reg,
				agent.WithSystem("You are a test assistant. When asked to run a command, use the bash tool. Be concise."),
				agent.WithMaxIterations(5),
			)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := a.Run(ctx, "Please run the command: echo integration-test-ok")
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			if !strings.Contains(result, "integration-test-ok") {
				t.Errorf("expected response to contain 'integration-test-ok', got: %q", result)
			}
		})
	}
}

// TestProvider_ToolUseRoundtrip verifies each provider correctly handles the
// tool_use → tool_result → text response cycle against the real API.
func TestProvider_ToolUseRoundtrip(t *testing.T) {
	for _, tc := range availableProviders(t) {
		t.Run(tc.name, func(t *testing.T) {
			toolSchema := provider.Tool{
				Name:        "get_weather",
				Description: "Get the current weather for a city",
				InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"city": {"type": "string", "description": "City name"}
					},
					"required": ["city"]
				}`),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Step 1: Send a message that should trigger tool use.
			resp1, err := tc.provider.Chat(ctx, provider.ChatRequest{
				Model:     tc.model,
				System:    "You have a get_weather tool. Always use it when asked about weather. Be concise.",
				Messages:  []provider.Message{provider.NewTextMessage(provider.RoleUser, "What's the weather in Tokyo?")},
				Tools:     []provider.Tool{toolSchema},
				MaxTokens: 1024,
			})
			if err != nil {
				t.Fatalf("Chat step 1: %v", err)
			}

			if resp1.StopReason != provider.StopToolUse {
				t.Fatalf("expected StopToolUse, got %q (text: %q)", resp1.StopReason, resp1.TextContent())
			}

			calls := resp1.ToolCalls()
			if len(calls) == 0 {
				t.Fatal("no tool calls in response")
			}
			if calls[0].Name != "get_weather" {
				t.Fatalf("expected get_weather tool call, got %q", calls[0].Name)
			}

			// Step 2: Feed back a fake tool result and get the final response.
			messages := []provider.Message{
				provider.NewTextMessage(provider.RoleUser, "What's the weather in Tokyo?"),
				{Role: provider.RoleAssistant, Content: resp1.Content},
				{
					Role: provider.RoleUser,
					Content: []provider.ContentBlock{
						{
							Type: provider.ContentToolResult,
							ToolResult: &provider.ToolResult{
								ToolUseID: calls[0].ID,
								Name:      calls[0].Name,
								Content:   `{"temperature": "22°C", "condition": "Sunny"}`,
							},
						},
					},
				},
			}

			resp2, err := tc.provider.Chat(ctx, provider.ChatRequest{
				Model:     tc.model,
				System:    "You have a get_weather tool. Be concise.",
				Messages:  messages,
				Tools:     []provider.Tool{toolSchema},
				MaxTokens: 256,
			})
			if err != nil {
				t.Fatalf("Chat step 2: %v", err)
			}

			if resp2.StopReason != provider.StopEndTurn {
				t.Fatalf("expected StopEndTurn, got %q", resp2.StopReason)
			}

			text := resp2.TextContent()
			if text == "" {
				t.Fatal("empty text response")
			}

			lower := strings.ToLower(text)
			if !strings.Contains(lower, "22") && !strings.Contains(lower, "sunny") {
				t.Errorf("response doesn't mention weather data: %q", text)
			}
		})
	}
}

// TestProvider_Streaming verifies each provider's streaming implementation
// delivers text deltas and a done event.
func TestProvider_Streaming(t *testing.T) {
	for _, tc := range availableProviders(t) {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			ch, err := tc.provider.StreamChat(ctx, provider.ChatRequest{
				Model:     tc.model,
				Messages:  []provider.Message{provider.NewTextMessage(provider.RoleUser, "Say 'test-stream-ok' and nothing else.")},
				MaxTokens: 256,
			})
			if err != nil {
				t.Fatalf("StreamChat: %v", err)
			}

			var text string
			var gotDone bool
			for event := range ch {
				switch event.Type {
				case provider.EventTextDelta:
					text += event.Text
				case provider.EventDone:
					gotDone = true
				case provider.EventError:
					t.Fatalf("stream error: %v", event.Error)
				}
			}

			if !gotDone {
				t.Error("never received Done event")
			}
			if !strings.Contains(strings.ToLower(text), "test-stream-ok") {
				t.Errorf("streamed text = %q, expected to contain 'test-stream-ok'", text)
			}
		})
	}
}

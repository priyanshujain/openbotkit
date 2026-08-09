package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
)

const defaultBaseURL = "https://api.openai.com"

// OpenAI implements provider.Provider using the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ provider.Provider = (*OpenAI)(nil)

// Option configures the OpenAI provider.
type Option func(*OpenAI)

func WithBaseURL(url string) Option {
	return func(o *OpenAI) { o.baseURL = url }
}

func WithHTTPClient(c *http.Client) Option {
	return func(o *OpenAI) { o.client = c }
}

func New(apiKey string, opts ...Option) *OpenAI {
	o := &OpenAI{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func init() {
	provider.RegisterFactory("openai", func(cfg config.ModelProviderConfig, apiKey string) provider.Provider {
		var opts []Option
		if cfg.BaseURL != "" {
			opts = append(opts, WithBaseURL(cfg.BaseURL))
		}
		return New(apiKey, opts...)
	})
}

// Chat sends a non-streaming request.
func (o *OpenAI) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	body := o.buildRequest(req, false)

	respBody, err := o.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(respBody).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("openai API error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	return o.parseResponse(&apiResp), nil
}

// StreamChat sends a streaming request.
func (o *OpenAI) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	body := o.buildRequest(req, true)

	respBody, err := o.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	ch := make(chan provider.StreamEvent, 64)
	go o.parseSSE(respBody, ch)
	return ch, nil
}

func (o *OpenAI) buildRequest(req provider.ChatRequest, stream bool) map[string]any {
	body := map[string]any{
		"model": req.Model,
	}
	if req.MaxTokens > 0 {
		body["max_completion_tokens"] = req.MaxTokens
	}
	if stream {
		body["stream"] = true
	}

	// Build messages. OpenAI uses system as a message role.
	var msgs []map[string]any
	if sysText := req.FullSystemText(); sysText != "" {
		msgs = append(msgs, map[string]any{
			"role":    "system",
			"content": sysText,
		})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, convertMessage(m)...)
	}
	body["messages"] = msgs

	// Convert tools.
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  json.RawMessage(t.InputSchema),
				},
			})
		}
		body["tools"] = tools
	}

	return body
}

// convertMessage translates a provider.Message into OpenAI message(s).
// Tool result messages become separate "tool" role messages in OpenAI.
func convertMessage(m provider.Message) []map[string]any {
	// Handle tool results: each result is a separate "tool" message.
	if m.Role == provider.RoleUser {
		hasToolResults := false
		for _, block := range m.Content {
			if block.Type == provider.ContentToolResult {
				hasToolResults = true
				break
			}
		}
		if hasToolResults {
			var msgs []map[string]any
			for _, block := range m.Content {
				if block.Type == provider.ContentToolResult && block.ToolResult != nil {
					msgs = append(msgs, map[string]any{
						"role":         "tool",
						"tool_call_id": block.ToolResult.ToolUseID,
						"content":      block.ToolResult.Content,
					})
				}
			}
			return msgs
		}
	}

	// Handle assistant messages with tool calls.
	if m.Role == provider.RoleAssistant {
		msg := map[string]any{"role": "assistant"}

		var textParts []string
		var toolCalls []map[string]any

		for _, block := range m.Content {
			switch block.Type {
			case provider.ContentText:
				textParts = append(textParts, block.Text)
			case provider.ContentToolUse:
				if block.ToolCall != nil {
					toolCalls = append(toolCalls, map[string]any{
						"id":   block.ToolCall.ID,
						"type": "function",
						"function": map[string]any{
							"name":      block.ToolCall.Name,
							"arguments": string(block.ToolCall.Input),
						},
					})
				}
			}
		}

		if len(textParts) > 0 {
			msg["content"] = strings.Join(textParts, "")
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		return []map[string]any{msg}
	}

	// Regular user message.
	var text string
	for _, block := range m.Content {
		if block.Type == provider.ContentText {
			text += block.Text
		}
	}
	return []map[string]any{{
		"role":    string(m.Role),
		"content": text,
	}}
}

func (o *OpenAI) doRequest(ctx context.Context, body map[string]any) (io.ReadCloser, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var apiErr apiResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != nil {
			return nil, fmt.Errorf("openai API error (HTTP %d): %s: %s", resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("openai API error: HTTP %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (o *OpenAI) parseResponse(resp *apiResponse) *provider.ChatResponse {
	result := &provider.ChatResponse{
		Usage: provider.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
	if resp.Usage.PromptTokensDetails != nil {
		result.Usage.CacheReadTokens = resp.Usage.PromptTokensDetails.CachedTokens
	}

	if len(resp.Choices) == 0 {
		result.StopReason = provider.StopEndTurn
		return result
	}

	choice := resp.Choices[0]

	switch choice.FinishReason {
	case "stop":
		result.StopReason = provider.StopEndTurn
	case "tool_calls":
		result.StopReason = provider.StopToolUse
	case "length":
		result.StopReason = provider.StopMaxTokens
	default:
		result.StopReason = provider.StopEndTurn
	}

	if choice.Message.Content != "" {
		result.Content = append(result.Content, provider.ContentBlock{
			Type: provider.ContentText,
			Text: choice.Message.Content,
		})
	}

	for _, tc := range choice.Message.ToolCalls {
		result.Content = append(result.Content, provider.ContentBlock{
			Type: provider.ContentToolUse,
			ToolCall: &provider.ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			},
		})
	}

	return result
}

func (o *OpenAI) parseSSE(body io.ReadCloser, ch chan<- provider.StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	currentToolCalls := make(map[int]*provider.ToolCall)
	currentDeltas := make(map[int]string)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- provider.StreamEvent{Type: provider.EventDone}
			return
		}

		var event sseChunk
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if len(event.Choices) == 0 {
			continue
		}

		delta := event.Choices[0].Delta
		finishReason := event.Choices[0].FinishReason

		if delta.Content != "" {
			ch <- provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: delta.Content,
			}
		}

		for _, tc := range delta.ToolCalls {
			if tc.Function.Name != "" {
				// New tool call starting.
				toolCall := &provider.ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				currentToolCalls[tc.Index] = toolCall
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallStart,
					ToolCall: toolCall,
				}
			}
			if tc.Function.Arguments != "" {
				currentDeltas[tc.Index] += tc.Function.Arguments
				ch <- provider.StreamEvent{
					Type:  provider.EventToolCallDelta,
					Delta: tc.Function.Arguments,
				}
			}
		}

		if finishReason == "tool_calls" || finishReason == "stop" {
			for idx := range currentToolCalls {
				if _, ok := currentDeltas[idx]; ok {
					ch <- provider.StreamEvent{Type: provider.EventToolCallEnd}
				}
			}
			ch <- provider.StreamEvent{Type: provider.EventDone}
			return
		}
	}
}

// API types for JSON marshaling.

type apiResponse struct {
	Choices []apiChoice `json:"choices"`
	Usage   apiUsage    `json:"usage"`
	Error   *apiError   `json:"error,omitempty"`
}

type apiChoice struct {
	Message      apiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type apiMessage struct {
	Role      string        `json:"role"`
	Content   string        `json:"content"`
	ToolCalls []apiToolCall `json:"tool_calls,omitempty"`
}

type apiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiUsage struct {
	PromptTokens        int                  `json:"prompt_tokens"`
	CompletionTokens    int                  `json:"completion_tokens"`
	PromptTokensDetails *promptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type promptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type sseChunk struct {
	Choices []sseChoice `json:"choices"`
}

type sseChoice struct {
	Delta        sseDelta `json:"delta"`
	FinishReason string   `json:"finish_reason"`
}

type sseDelta struct {
	Content   string        `json:"content"`
	ToolCalls []sseToolCall `json:"tool_calls,omitempty"`
}

type sseToolCall struct {
	Index    int         `json:"index"`
	ID       string      `json:"id"`
	Function apiFunction `json:"function"`
}

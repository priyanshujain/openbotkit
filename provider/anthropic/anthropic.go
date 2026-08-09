package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
)

const (
	defaultBaseURL   = "https://api.anthropic.com"
	apiVersion       = "2023-06-01"
	vertexAPIVersion = "vertex-2023-10-16"
)

type Anthropic struct {
	apiKey  string
	baseURL string
	client  *http.Client

	vertexProject string
	vertexRegion  string
	tokenOnce     sync.Once
	tokenSource   oauth2.TokenSource
}

var _ provider.Provider = (*Anthropic)(nil)

func init() {
	provider.RegisterFactory("anthropic", func(cfg config.ModelProviderConfig, apiKey string) provider.Provider {
		var opts []Option
		if cfg.BaseURL != "" {
			opts = append(opts, WithBaseURL(cfg.BaseURL))
		}
		if cfg.AuthMethod == "vertex_ai" {
			opts = append(opts, WithVertexAI(cfg.VertexProject, cfg.VertexRegion))
			opts = append(opts, WithTokenSource(provider.GcloudTokenSource(cfg.VertexAccount)))
		}
		return New(apiKey, opts...)
	})
}

type Option func(*Anthropic)

func WithBaseURL(url string) Option {
	return func(a *Anthropic) { a.baseURL = url }
}

func WithHTTPClient(c *http.Client) Option {
	return func(a *Anthropic) { a.client = c }
}

func WithVertexAI(project, region string) Option {
	return func(a *Anthropic) {
		a.vertexProject = project
		a.vertexRegion = region
	}
}

func WithTokenSource(ts oauth2.TokenSource) Option {
	return func(a *Anthropic) { a.tokenSource = ts }
}

func (a *Anthropic) isVertexAI() bool {
	return a.vertexProject != ""
}

func (a *Anthropic) vertexToken(ctx context.Context) (string, error) {
	if a.tokenSource == nil {
		var initErr error
		a.tokenOnce.Do(func() {
			a.tokenSource, initErr = google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
		})
		if initErr != nil {
			return "", fmt.Errorf("get google credentials: %w", initErr)
		}
	}
	token, err := a.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	return token.AccessToken, nil
}

func New(apiKey string, opts ...Option) *Anthropic {
	a := &Anthropic{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *Anthropic) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	body := a.buildRequest(req, false)

	respBody, err := a.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(respBody).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Type == "error" {
		return nil, fmt.Errorf("anthropic API error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	return a.parseResponse(&apiResp), nil
}

func (a *Anthropic) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	body := a.buildRequest(req, true)

	respBody, err := a.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	ch := make(chan provider.StreamEvent, 64)
	go a.parseSSE(respBody, ch)
	return ch, nil
}

func (a *Anthropic) buildRequest(req provider.ChatRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
	}
	if req.MaxTokens == 0 {
		body["max_tokens"] = 4096
	}
	if blocks := req.EffectiveSystemBlocks(); len(blocks) > 0 {
		var systemBlocks []map[string]any
		for _, block := range blocks {
			sb := map[string]any{"type": "text", "text": block.Text}
			if block.CacheControl != nil {
				sb["cache_control"] = map[string]any{"type": block.CacheControl.Type}
			}
			systemBlocks = append(systemBlocks, sb)
		}
		body["system"] = systemBlocks
	}
	if stream {
		body["stream"] = true
	}

	var msgs []map[string]any
	for _, m := range req.Messages {
		msgs = append(msgs, convertMessage(m))
	}
	body["messages"] = msgs

	if len(req.Tools) > 0 {
		var tools []map[string]any
		for i, t := range req.Tools {
			tool := map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": json.RawMessage(t.InputSchema),
			}
			if i == len(req.Tools)-1 {
				tool["cache_control"] = map[string]any{"type": "ephemeral"}
			}
			tools = append(tools, tool)
		}
		body["tools"] = tools
	}

	return body
}

func convertMessage(m provider.Message) map[string]any {
	msg := map[string]any{
		"role": string(m.Role),
	}

	var content []map[string]any
	for _, block := range m.Content {
		switch block.Type {
		case provider.ContentText:
			content = append(content, map[string]any{
				"type": "text",
				"text": block.Text,
			})
		case provider.ContentToolUse:
			if block.ToolCall != nil {
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    block.ToolCall.ID,
					"name":  block.ToolCall.Name,
					"input": json.RawMessage(block.ToolCall.Input),
				})
			}
		case provider.ContentToolResult:
			if block.ToolResult != nil {
				result := map[string]any{
					"type":        "tool_result",
					"tool_use_id": block.ToolResult.ToolUseID,
					"content":     block.ToolResult.Content,
				}
				if block.ToolResult.IsError {
					result["is_error"] = true
				}
				content = append(content, result)
			}
		}
	}

	// Single text block → use string content for simplicity.
	if len(content) == 1 && content[0]["type"] == "text" {
		msg["content"] = content[0]["text"]
	} else {
		msg["content"] = content
	}

	return msg
}

func (a *Anthropic) doRequest(ctx context.Context, body map[string]any) (io.ReadCloser, error) {
	var url string
	var bearerToken string

	if a.isVertexAI() {
		model, _ := body["model"].(string)
		delete(body, "model")
		body["anthropic_version"] = vertexAPIVersion

		endpoint := "rawPredict"
		if _, ok := body["stream"]; ok {
			endpoint = "streamRawPredict"
		}
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:%s",
			a.vertexRegion, a.vertexProject, a.vertexRegion, model, endpoint)

		var err error
		bearerToken, err = a.vertexToken(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		url = a.baseURL + "/v1/messages"
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if a.isVertexAI() {
		httpReq.Header.Set("Authorization", "Bearer "+bearerToken)
	} else {
		httpReq.Header.Set("x-api-key", a.apiKey)
		httpReq.Header.Set("anthropic-version", apiVersion)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var apiErr apiResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("anthropic API error (HTTP %d): %s: %s", resp.StatusCode, apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("anthropic API error: HTTP %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (a *Anthropic) parseResponse(resp *apiResponse) *provider.ChatResponse {
	result := &provider.ChatResponse{
		Usage: provider.Usage{
			InputTokens:      resp.Usage.InputTokens,
			OutputTokens:     resp.Usage.OutputTokens,
			CacheReadTokens:  resp.Usage.CacheReadInputTokens,
			CacheWriteTokens: resp.Usage.CacheCreationInputTokens,
		},
	}

	switch resp.StopReason {
	case "end_turn":
		result.StopReason = provider.StopEndTurn
	case "tool_use":
		result.StopReason = provider.StopToolUse
	case "max_tokens":
		result.StopReason = provider.StopMaxTokens
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content = append(result.Content, provider.ContentBlock{
				Type: provider.ContentText,
				Text: block.Text,
			})
		case "tool_use":
			result.Content = append(result.Content, provider.ContentBlock{
				Type: provider.ContentToolUse,
				ToolCall: &provider.ToolCall{
					ID:    block.ID,
					Name:  block.Name,
					Input: block.Input,
				},
			})
		}
	}

	return result
}

func (a *Anthropic) parseSSE(body io.ReadCloser, ch chan<- provider.StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var currentToolCall *provider.ToolCall

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event sseEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				currentToolCall = &provider.ToolCall{
					ID:   event.ContentBlock.ID,
					Name: event.ContentBlock.Name,
				}
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallStart,
					ToolCall: currentToolCall,
				}
			}

		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				ch <- provider.StreamEvent{
					Type: provider.EventTextDelta,
					Text: event.Delta.Text,
				}
			} else if event.Delta.Type == "input_json_delta" {
				ch <- provider.StreamEvent{
					Type:  provider.EventToolCallDelta,
					Delta: event.Delta.PartialJSON,
				}
			}

		case "content_block_stop":
			if currentToolCall != nil {
				ch <- provider.StreamEvent{
					Type: provider.EventToolCallEnd,
				}
				currentToolCall = nil
			}

		case "message_stop":
			ch <- provider.StreamEvent{
				Type: provider.EventDone,
			}
			return

		case "message_delta":
			// Contains stop_reason and usage — we emit Done.
			ch <- provider.StreamEvent{
				Type: provider.EventDone,
			}
			return

		case "error":
			ch <- provider.StreamEvent{
				Type:  provider.EventError,
				Error: fmt.Errorf("stream error: %s", event.Error.Message),
			}
			return
		}
	}
}

type apiResponse struct {
	Type       string       `json:"type"`
	Content    []apiContent `json:"content"`
	StopReason string       `json:"stop_reason"`
	Usage      apiUsage     `json:"usage"`
	Error      apiError     `json:"error"`
}

type apiContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type apiUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type sseEvent struct {
	Type         string     `json:"type"`
	ContentBlock apiContent `json:"content_block,omitempty"`
	Delta        sseDelta   `json:"delta,omitempty"`
	Error        apiError   `json:"error,omitempty"`
}

type sseDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

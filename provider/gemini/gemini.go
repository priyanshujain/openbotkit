package gemini

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/provider"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Gemini implements provider.Provider using the Google Gemini API.
// It supports both the direct Gemini API and Google Vertex AI.
type Gemini struct {
	apiKey  string
	baseURL string
	client  *http.Client

	// Vertex AI fields.
	vertexProject string
	vertexRegion  string
	tokenOnce     sync.Once
	tokenSource   oauth2.TokenSource

	// CachedContent lifecycle.
	cacheMu           sync.Mutex
	cachedContentName string
	cachedContentHash string
	cacheExpiry       time.Time
}

var _ provider.Provider = (*Gemini)(nil)

// Option configures the Gemini provider.
type Option func(*Gemini)

func WithBaseURL(url string) Option {
	return func(g *Gemini) { g.baseURL = url }
}

func WithHTTPClient(c *http.Client) Option {
	return func(g *Gemini) { g.client = c }
}

// WithVertexAI configures the provider to use Google Vertex AI.
// Uses Google Application Default Credentials unless a custom TokenSource is provided via WithTokenSource.
func WithVertexAI(project, region string) Option {
	return func(g *Gemini) {
		g.vertexProject = project
		g.vertexRegion = region
	}
}

// WithTokenSource sets a custom OAuth2 token source for Vertex AI authentication.
func WithTokenSource(ts oauth2.TokenSource) Option {
	return func(g *Gemini) { g.tokenSource = ts }
}

func (g *Gemini) isVertexAI() bool {
	return g.vertexProject != ""
}

func (g *Gemini) vertexToken(ctx context.Context) (string, error) {
	if g.tokenSource == nil {
		var initErr error
		g.tokenOnce.Do(func() {
			g.tokenSource, initErr = google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
		})
		if initErr != nil {
			return "", fmt.Errorf("get google credentials: %w", initErr)
		}
	}
	token, err := g.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	return token.AccessToken, nil
}

func New(apiKey string, opts ...Option) *Gemini {
	g := &Gemini{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func init() {
	provider.RegisterFactory("gemini", func(cfg config.ModelProviderConfig, apiKey string) provider.Provider {
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

// Chat sends a non-streaming request.
func (g *Gemini) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	body := g.buildRequest(req)
	g.applyCacheToBody(ctx, req, body)

	url, err := g.chatURL(ctx, req.Model, false)
	if err != nil {
		return nil, err
	}
	respBody, err := g.doRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(respBody).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("gemini API error: %d: %s", apiResp.Error.Code, apiResp.Error.Message)
	}

	return g.parseResponse(&apiResp), nil
}

// StreamChat sends a streaming request.
func (g *Gemini) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	body := g.buildRequest(req)
	g.applyCacheToBody(ctx, req, body)

	url, err := g.chatURL(ctx, req.Model, true)
	if err != nil {
		return nil, err
	}
	respBody, err := g.doRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}

	ch := make(chan provider.StreamEvent, 64)
	go g.parseSSE(respBody, ch)
	return ch, nil
}

// chatURL returns the endpoint URL, handling both direct API and Vertex AI.
func (g *Gemini) chatURL(ctx context.Context, model string, stream bool) (string, error) {
	if g.isVertexAI() {
		action := "generateContent"
		suffix := ""
		if stream {
			action = "streamGenerateContent"
			suffix = "?alt=sse"
		}
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:%s%s",
			g.vertexRegion, g.vertexProject, g.vertexRegion, model, action, suffix), nil
	}

	if stream {
		return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", g.baseURL, model, g.apiKey), nil
	}
	return fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", g.baseURL, model, g.apiKey), nil
}

func (g *Gemini) buildRequest(req provider.ChatRequest) map[string]any {
	body := map[string]any{}

	if sysText := req.FullSystemText(); sysText != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": sysText}},
		}
	}

	var contents []map[string]any
	for _, m := range req.Messages {
		contents = append(contents, convertMessage(m)...)
	}
	body["contents"] = contents

	if toolsVal := buildToolDeclarations(req.Tools); toolsVal != nil {
		body["tools"] = toolsVal
	}

	genConfig := map[string]any{}
	if req.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = req.MaxTokens
	}
	if req.DisableThinking {
		genConfig["thinkingConfig"] = map[string]any{"thinkingBudget": 0}
	}
	if len(genConfig) > 0 {
		body["generationConfig"] = genConfig
	}

	return body
}

// applyCacheToBody attempts to use a cached content resource. If successful,
// it removes systemInstruction and tools from the body and sets cachedContent.
func (g *Gemini) applyCacheToBody(ctx context.Context, req provider.ChatRequest, body map[string]any) {
	cacheName := g.ensureCache(ctx, req)
	if cacheName == "" {
		return
	}
	delete(body, "systemInstruction")
	delete(body, "tools")
	body["cachedContent"] = cacheName
}

// ensureCache creates or reuses a CachedContent resource for the system+tools prefix.
// Returns the resource name on success, or empty string if caching is unavailable.
func (g *Gemini) ensureCache(ctx context.Context, req provider.ChatRequest) string {
	sysText := req.FullSystemText()
	if sysText == "" && len(req.Tools) == 0 {
		return ""
	}

	hash := g.computeCacheHash(req)

	g.cacheMu.Lock()
	defer g.cacheMu.Unlock()

	if g.cachedContentName != "" && g.cachedContentHash == hash && time.Now().Before(g.cacheExpiry) {
		return g.cachedContentName
	}

	// Delete stale cache if any.
	if g.cachedContentName != "" {
		go g.deleteCache(context.Background(), g.cachedContentName)
		g.cachedContentName = ""
		g.cachedContentHash = ""
	}

	name, expiry, err := g.createCache(ctx, req)
	if err != nil {
		slog.Debug("gemini: cache creation failed, proceeding without cache", "error", err)
		return ""
	}

	g.cachedContentName = name
	g.cachedContentHash = hash
	g.cacheExpiry = expiry
	return name
}

func (g *Gemini) computeCacheHash(req provider.ChatRequest) string {
	h := sha256.New()
	h.Write([]byte(req.FullSystemText()))
	for _, t := range req.Tools {
		h.Write([]byte(t.Name))
		h.Write(t.InputSchema)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (g *Gemini) createCache(ctx context.Context, req provider.ChatRequest) (string, time.Time, error) {
	cacheBody := map[string]any{
		"model":       g.cacheModelName(req.Model),
		"displayName": "obk-prompt-cache",
		"expireTime":  time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
	}

	if sysText := req.FullSystemText(); sysText != "" {
		cacheBody["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": sysText}},
		}
	}

	if toolsVal := buildToolDeclarations(req.Tools); toolsVal != nil {
		cacheBody["tools"] = toolsVal
	}

	url, err := g.cacheURL(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	jsonBody, err := json.Marshal(cacheBody)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal cache body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create cache request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if g.isVertexAI() {
		token, err := g.vertexToken(ctx)
		if err != nil {
			return "", time.Time{}, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("send cache request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("cache creation failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var cacheResp struct {
		Name       string `json:"name"`
		ExpireTime string `json:"expireTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cacheResp); err != nil {
		return "", time.Time{}, fmt.Errorf("decode cache response: %w", err)
	}

	expiry, _ := time.Parse(time.RFC3339, cacheResp.ExpireTime)
	return cacheResp.Name, expiry, nil
}

func (g *Gemini) deleteCache(ctx context.Context, name string) {
	url, err := g.deleteCacheURL(ctx, name)
	if err != nil {
		return
	}
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return
	}
	if g.isVertexAI() {
		token, err := g.vertexToken(ctx)
		if err != nil {
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := g.client.Do(httpReq)
	if err == nil {
		resp.Body.Close()
	}
}

func (g *Gemini) cacheModelName(model string) string {
	if g.isVertexAI() {
		return fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s",
			g.vertexProject, g.vertexRegion, model)
	}
	return "models/" + model
}

func (g *Gemini) cacheURL(ctx context.Context) (string, error) {
	if g.isVertexAI() {
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/cachedContents",
			g.vertexRegion, g.vertexProject, g.vertexRegion), nil
	}
	return fmt.Sprintf("%s/v1beta/cachedContents?key=%s", g.baseURL, g.apiKey), nil
}

func (g *Gemini) deleteCacheURL(ctx context.Context, name string) (string, error) {
	if g.isVertexAI() {
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s",
			g.vertexRegion, name), nil
	}
	return fmt.Sprintf("%s/v1beta/%s?key=%s", g.baseURL, name, g.apiKey), nil
}

func buildToolDeclarations(tools []provider.Tool) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	var decls []map[string]any
	for _, t := range tools {
		decl := map[string]any{
			"name":        t.Name,
			"description": t.Description,
		}
		var schema map[string]any
		if err := json.Unmarshal(t.InputSchema, &schema); err == nil {
			decl["parameters"] = schema
		}
		decls = append(decls, decl)
	}
	return []map[string]any{{"functionDeclarations": decls}}
}

// convertMessage translates a provider.Message into Gemini content(s).
func convertMessage(m provider.Message) []map[string]any {
	role := string(m.Role)
	if role == "assistant" {
		role = "model"
	}

	// Check if this message contains tool results — Gemini requires
	// functionResponse parts in a separate "user" content with all
	// results grouped together.
	var funcResponseParts []map[string]any
	var otherParts []map[string]any

	for _, block := range m.Content {
		switch block.Type {
		case provider.ContentText:
			otherParts = append(otherParts, map[string]any{"text": block.Text})
		case provider.ContentToolUse:
			if block.ToolCall != nil {
				var args map[string]any
				_ = json.Unmarshal(block.ToolCall.Input, &args)
				otherParts = append(otherParts, map[string]any{
					"functionCall": map[string]any{
						"name": block.ToolCall.Name,
						"args": args,
					},
				})
			}
		case provider.ContentToolResult:
			if block.ToolResult != nil {
				var response map[string]any
				if err := json.Unmarshal([]byte(block.ToolResult.Content), &response); err != nil {
					response = map[string]any{"result": block.ToolResult.Content}
				}
				// Gemini matches functionResponse by function name, not by call ID.
				name := block.ToolResult.Name
				if name == "" {
					name = block.ToolResult.ToolUseID
				}
				funcResponseParts = append(funcResponseParts, map[string]any{
					"functionResponse": map[string]any{
						"name":     name,
						"response": response,
					},
				})
			}
		}
	}

	var result []map[string]any
	if len(otherParts) > 0 {
		result = append(result, map[string]any{
			"role":  role,
			"parts": otherParts,
		})
	}
	if len(funcResponseParts) > 0 {
		result = append(result, map[string]any{
			"role":  "user",
			"parts": funcResponseParts,
		})
	}
	if len(result) == 0 {
		result = append(result, map[string]any{
			"role":  role,
			"parts": []map[string]any{},
		})
	}

	return result
}

func (g *Gemini) doRequest(ctx context.Context, url string, body map[string]any) (io.ReadCloser, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if g.isVertexAI() {
		token, err := g.vertexToken(ctx)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var apiErr apiResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != nil {
			return nil, fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("gemini API error: HTTP %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (g *Gemini) parseResponse(resp *apiResponse) *provider.ChatResponse {
	result := &provider.ChatResponse{
		Usage: provider.Usage{
			InputTokens:     resp.UsageMetadata.PromptTokenCount,
			OutputTokens:    resp.UsageMetadata.CandidatesTokenCount,
			CacheReadTokens: resp.UsageMetadata.CachedContentTokenCount,
		},
	}

	if len(resp.Candidates) == 0 {
		result.StopReason = provider.StopEndTurn
		return result
	}

	candidate := resp.Candidates[0]
	hasToolCalls := false

	for i, part := range candidate.Content.Parts {
		if part.Text != "" {
			result.Content = append(result.Content, provider.ContentBlock{
				Type: provider.ContentText,
				Text: part.Text,
			})
		}
		if part.FunctionCall != nil {
			hasToolCalls = true
			argsJSON, _ := json.Marshal(part.FunctionCall.Args)
			result.Content = append(result.Content, provider.ContentBlock{
				Type: provider.ContentToolUse,
				ToolCall: &provider.ToolCall{
					ID:    fmt.Sprintf("call_%d", i),
					Name:  part.FunctionCall.Name,
					Input: argsJSON,
				},
			})
		}
	}

	if hasToolCalls {
		result.StopReason = provider.StopToolUse
	} else {
		switch candidate.FinishReason {
		case "MAX_TOKENS":
			result.StopReason = provider.StopMaxTokens
		default:
			result.StopReason = provider.StopEndTurn
		}
	}

	return result
}

func (g *Gemini) parseSSE(body io.ReadCloser, ch chan<- provider.StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var resp apiResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		if resp.Error != nil {
			ch <- provider.StreamEvent{
				Type:  provider.EventError,
				Error: fmt.Errorf("stream error: %s", resp.Error.Message),
			}
			return
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		candidate := resp.Candidates[0]
		for i, part := range candidate.Content.Parts {
			if part.Text != "" {
				ch <- provider.StreamEvent{
					Type: provider.EventTextDelta,
					Text: part.Text,
				}
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCall := &provider.ToolCall{
					ID:   fmt.Sprintf("call_%d", i),
					Name: part.FunctionCall.Name,
				}
				ch <- provider.StreamEvent{
					Type:     provider.EventToolCallStart,
					ToolCall: toolCall,
				}
				ch <- provider.StreamEvent{
					Type:  provider.EventToolCallDelta,
					Delta: string(argsJSON),
				}
				ch <- provider.StreamEvent{
					Type: provider.EventToolCallEnd,
				}
			}
		}

		if candidate.FinishReason == "STOP" || candidate.FinishReason == "MAX_TOKENS" {
			ch <- provider.StreamEvent{Type: provider.EventDone}
			return
		}
	}
}

// API types for JSON marshaling.

type apiResponse struct {
	Candidates    []apiCandidate `json:"candidates"`
	UsageMetadata apiUsage       `json:"usageMetadata"`
	Error         *apiError      `json:"error,omitempty"`
}

type apiCandidate struct {
	Content      apiContent `json:"content"`
	FinishReason string     `json:"finishReason"`
}

type apiContent struct {
	Role  string    `json:"role"`
	Parts []apiPart `json:"parts"`
}

type apiPart struct {
	Text         string       `json:"text,omitempty"`
	FunctionCall *apiFuncCall `json:"functionCall,omitempty"`
}

type apiFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type apiUsage struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

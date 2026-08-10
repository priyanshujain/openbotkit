package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/73ai/openbotkit/agent"
	"github.com/73ai/openbotkit/provider"
	"github.com/73ai/openbotkit/service/learnings"
)

// LearningsNotifier sends push notifications for learning events.
// This avoids importing the channel package (which imports tools).
type LearningsNotifier interface {
	Push(ctx context.Context, message string) error
}

type LearningsDeps struct {
	Store    *learnings.Store
	BaseURL  string
	Notifier LearningsNotifier
}

type LearningsExtractDeps struct {
	LearningsDeps
	Provider provider.Provider
	Model    string
}

// LearningSaveTool

type LearningSaveTool struct {
	deps LearningsDeps
}

func NewLearningSaveTool(deps LearningsDeps) *LearningSaveTool {
	return &LearningSaveTool{deps: deps}
}

func (t *LearningSaveTool) Name() string { return "learnings_save" }
func (t *LearningSaveTool) Description() string {
	return "Save learnings as bullet points under a topic"
}
func (t *LearningSaveTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"topic": {
				"type": "string",
				"description": "Topic name (e.g. Go, SQL, Docker)"
			},
			"bullets": {
				"type": "array",
				"items": {"type": "string"},
				"description": "List of learning bullet points"
			}
		},
		"required": ["topic", "bullets"]
	}`)
}

type learningSaveInput struct {
	Topic   string   `json:"topic"`
	Bullets []string `json:"bullets"`
}

func (t *LearningSaveTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in learningSaveInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if in.Topic == "" {
		return "", fmt.Errorf("topic is required")
	}
	if len(in.Bullets) == 0 {
		return "", fmt.Errorf("at least one bullet is required")
	}

	if err := t.deps.Store.Save(in.Topic, in.Bullets); err != nil {
		return "", fmt.Errorf("save learning: %w", err)
	}

	if t.deps.Notifier != nil {
		slug := t.deps.Store.Slug(in.Topic)
		msg := fmt.Sprintf("Saved a learning about %s", in.Topic)
		if t.deps.BaseURL != "" {
			msg += fmt.Sprintf("\n%s/learnings/%s", t.deps.BaseURL, slug)
		}
		if err := t.deps.Notifier.Push(ctx, msg); err != nil {
			slog.Warn("learnings: notification failed", "error", err)
		}
	}

	return fmt.Sprintf("Saved %d bullet(s) under topic %q.", len(in.Bullets), in.Topic), nil
}

// LearningReadTool

type LearningReadTool struct {
	deps LearningsDeps
}

func NewLearningReadTool(deps LearningsDeps) *LearningReadTool {
	return &LearningReadTool{deps: deps}
}

func (t *LearningReadTool) Name() string        { return "learnings_read" }
func (t *LearningReadTool) Description() string { return "Read a learning topic or list all topics" }
func (t *LearningReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"topic": {
				"type": "string",
				"description": "Topic name to read. Omit to list all topics."
			}
		}
	}`)
}

type learningReadInput struct {
	Topic string `json:"topic"`
}

func (t *LearningReadTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var in learningReadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if in.Topic == "" {
		topics, err := t.deps.Store.List()
		if err != nil {
			return "", err
		}
		if len(topics) == 0 {
			return "No learning topics saved yet.", nil
		}
		return "Topics:\n- " + strings.Join(topics, "\n- "), nil
	}

	content, err := t.deps.Store.Read(in.Topic)
	if err != nil {
		return "", err
	}
	return content, nil
}

// LearningSearchTool

type LearningSearchTool struct {
	deps LearningsDeps
}

func NewLearningSearchTool(deps LearningsDeps) *LearningSearchTool {
	return &LearningSearchTool{deps: deps}
}

func (t *LearningSearchTool) Name() string { return "learnings_search" }
func (t *LearningSearchTool) Description() string {
	return "Search across all saved learnings"
}
func (t *LearningSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query"
			}
		},
		"required": ["query"]
	}`)
}

type learningSearchInput struct {
	Query string `json:"query"`
}

func (t *LearningSearchTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var in learningSearchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if in.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	results, err := t.deps.Store.Search(in.Query)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "No results found.", nil
	}

	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "[%s] %s\n", r.Topic, r.Line)
	}
	return b.String(), nil
}

// LearningExtractTool

type LearningExtractTool struct {
	deps LearningsExtractDeps
}

func NewLearningExtractTool(deps LearningsExtractDeps) *LearningExtractTool {
	return &LearningExtractTool{deps: deps}
}

func (t *LearningExtractTool) Name() string { return "learnings_extract" }
func (t *LearningExtractTool) Description() string {
	return "Extract and save key learnings from source material (runs in background)"
}
func (t *LearningExtractTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"context": {
				"type": "string",
				"description": "Source material to extract learnings from"
			}
		},
		"required": ["context"]
	}`)
}

type learningExtractInput struct {
	Context string `json:"context"`
}

func (t *LearningExtractTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in learningExtractInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if in.Context == "" {
		return "", fmt.Errorf("context is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	go func() {
		defer cancel()
		t.run(ctx, in.Context)
	}()

	return "Extraction started. You'll get a notification when done.", nil
}

// trackingSaveTool wraps LearningSaveTool to record which topics were saved.
type trackingSaveTool struct {
	inner *LearningSaveTool
	mu    sync.Mutex
	slugs []string
}

func (t *trackingSaveTool) Name() string                 { return t.inner.Name() }
func (t *trackingSaveTool) Description() string          { return t.inner.Description() }
func (t *trackingSaveTool) InputSchema() json.RawMessage { return t.inner.InputSchema() }
func (t *trackingSaveTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in learningSaveInput
	if err := json.Unmarshal(input, &in); err == nil && in.Topic != "" {
		slug := t.inner.deps.Store.Slug(in.Topic)
		t.mu.Lock()
		t.slugs = append(t.slugs, slug)
		t.mu.Unlock()
	}
	return t.inner.Execute(ctx, input)
}

func (t *LearningExtractTool) run(ctx context.Context, material string) {
	if t.deps.Provider == nil {
		slog.Error("learnings: extraction skipped, no provider configured")
		return
	}

	subDeps := LearningsDeps{Store: t.deps.Store}
	tracker := &trackingSaveTool{inner: NewLearningSaveTool(subDeps)}
	toolReg := NewRegistry()
	toolReg.Register(tracker)
	toolReg.Register(NewLearningReadTool(subDeps))

	system := `You are a learning extraction assistant. Read the provided material and extract key learnings.
Learnings are knowledge, techniques, insights, or skills the user picked up. Things like how something works, a useful pattern, a gotcha to avoid.
Do NOT save: personal info (name, timezone, contacts), factual lookups (currency conversions, weather), trivial Q&A, or preferences. Those are not learnings.
If the material has nothing worth saving as a learning, just reply "Nothing to save here" and do not call any tools.
For each distinct topic, call learnings_save with a topic name and concise bullet points.
Use learnings_read first to check existing topics so you can append to them rather than creating duplicates.
Keep bullet points casual, concise, and useful. No emdashes.
After saving, reply with a short friendly 1-2 sentence summary of what you saved. If you saved nothing, say so. This message will be sent as a notification.`

	blocks := []provider.SystemBlock{
		{Text: system, CacheControl: &provider.CacheControl{Type: "ephemeral"}},
	}

	a := agent.New(t.deps.Provider, t.deps.Model, toolReg,
		agent.WithSystemBlocks(blocks),
		agent.WithMaxIterations(15),
	)

	result, err := a.Run(ctx, material)
	if err != nil {
		slog.Error("learnings: extraction failed", "error", err)
		return
	}

	if t.deps.Notifier != nil {
		msg := strings.TrimSpace(result)
		if msg == "" {
			msg = "Saved your learnings!"
		}
		if t.deps.BaseURL != "" {
			for _, slug := range tracker.slugs {
				msg += fmt.Sprintf("\n%s/learnings/%s", t.deps.BaseURL, slug)
			}
		}
		if err := t.deps.Notifier.Push(ctx, msg); err != nil {
			slog.Warn("learnings: extract notification failed", "error", err)
		}
	}

	slog.Info("learnings: extraction complete")
}

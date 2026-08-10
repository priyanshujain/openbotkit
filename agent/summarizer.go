package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/73ai/openbotkit/provider"
)

const summarizePrompt = `Summarize the conversation below. Preserve: decisions made, user preferences, unresolved tasks, key facts. Omit: greetings, filler, verbose tool output. Keep under 2000 characters.`

// LLMSummarizer summarizes conversation messages using an LLM.
type LLMSummarizer struct {
	Provider provider.Provider
	Model    string
}

func (s *LLMSummarizer) Summarize(ctx context.Context, messages []provider.Message) (string, error) {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		for _, c := range m.Content {
			if c.Text != "" {
				b.WriteString(c.Text)
			}
		}
		b.WriteByte('\n')
	}

	resp, err := s.Provider.Chat(ctx, provider.ChatRequest{
		Model:     s.Model,
		System:    summarizePrompt,
		Messages:  []provider.Message{provider.NewTextMessage(provider.RoleUser, b.String())},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}
	return resp.TextContent(), nil
}

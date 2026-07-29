package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/73ai/openbotkit/source/slack"
)

func TestSlackReadThreadTool_Execute(t *testing.T) {
	api := &mockSlackAPI{
		repliesResult: []slack.Message{
			{TS: "1785334150.236249", Text: "parent message"},
			{TS: "1785334200.111111", Text: "reply 1", ThreadTS: "1785334150.236249"},
		},
		channels: []slack.Channel{{ID: "C123", Name: "general"}},
	}
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: api})

	input, _ := json.Marshal(slackReadThreadInput{Channel: "C123", ThreadTS: "1785334150.236249"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "reply 1") {
		t.Errorf("result = %q", result)
	}
	if !strings.Contains(result, "p1785334150236249") {
		t.Errorf("expected message_id in output: %q", result)
	}
}

func TestSlackReadThreadTool_Permalink(t *testing.T) {
	api := &mockSlackAPI{
		repliesResult: []slack.Message{
			{TS: "1785334150.236249", Text: "parent message"},
			{TS: "1785334200.111111", Text: "reply 1", ThreadTS: "1785334150.236249"},
		},
	}
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: api})

	input, _ := json.Marshal(slackReadThreadInput{
		Permalink: "https://acme.slack.com/archives/C01Q9G9CD6X/p1785334150236249",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "C01Q9G9CD6X") {
		t.Errorf("result = %q", result)
	}
	if len(api.repliesTS) == 0 || api.repliesTS[0] != "1785334150.236249" {
		t.Errorf("requested ts = %v", api.repliesTS)
	}
}

// A permalink to a reply carries thread_ts; the whole thread comes back.
func TestSlackReadThreadTool_PermalinkToReply(t *testing.T) {
	api := &mockSlackAPI{
		repliesResult: []slack.Message{
			{TS: "1785319311.419309", Text: "the root"},
			{TS: "1785319451.036269", Text: "a reply", ThreadTS: "1785319311.419309"},
		},
	}
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: api})

	input, _ := json.Marshal(slackReadThreadInput{
		Permalink: "https://acme.slack.com/archives/C094FL95GDV/p1785319451036269?thread_ts=1785319311.419309",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.repliesTS) == 0 || api.repliesTS[0] != "1785319311.419309" {
		t.Errorf("should fetch the root, requested %v", api.repliesTS)
	}
	if !strings.Contains(result, "the root") || !strings.Contains(result, "a reply") {
		t.Errorf("result = %q", result)
	}
}

func TestSlackReadThreadTool_ResolvesNames(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &mockSlackAPI{
		repliesResult: []slack.Message{
			{TS: "1785334150.236249", User: "U1", Text: "hello <@U2>"},
		},
		userInfo: map[string]*slack.User{
			"U1": {ID: "U1", Profile: &slack.UserProfile{DisplayName: "rahul.support"}},
			"U2": {ID: "U2", Name: "priya"},
		},
	}
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: api, Workspace: "acme"})

	input, _ := json.Marshal(slackReadThreadInput{Channel: "C123", ThreadTS: "p1785334150236249"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "rahul.support") {
		t.Errorf("author name missing: %q", result)
	}
	if !strings.Contains(result, "priya") {
		t.Errorf("mentioned user missing from users map: %q", result)
	}
}

func TestSlackReadThreadTool_MissingParams(t *testing.T) {
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: &mockSlackAPI{}})

	input, _ := json.Marshal(slackReadThreadInput{Channel: "C123"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing thread_ts")
	}

	input, _ = json.Marshal(slackReadThreadInput{})
	if _, err := tool.Execute(context.Background(), input); err == nil {
		t.Fatal("expected error when nothing is provided")
	}
}

func TestSlackReadThreadTool_BadPermalink(t *testing.T) {
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	input, _ := json.Marshal(slackReadThreadInput{Permalink: "https://example.com/nope"})
	if _, err := tool.Execute(context.Background(), input); err == nil {
		t.Fatal("expected error for a non-Slack permalink")
	}
}

func TestSlackReadThreadTool_Name(t *testing.T) {
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	if tool.Name() != "slack_read_thread" {
		t.Errorf("Name() = %q", tool.Name())
	}
}

func TestSlackReadThreadTool_Metadata(t *testing.T) {
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	if tool.Name() != "slack_read_thread" {
		t.Errorf("Name() = %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("empty description")
	}
	if !json.Valid(tool.InputSchema()) {
		t.Error("invalid schema")
	}
}

func TestSlackReadThreadTool_InvalidJSON(t *testing.T) {
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSlackReadThreadTool_ResolveError(t *testing.T) {
	api := &mockSlackAPI{channels: []slack.Channel{}}
	tool := NewSlackReadThreadTool(SlackToolDeps{Client: api})
	input, _ := json.Marshal(slackReadThreadInput{Channel: "#nonexistent", ThreadTS: "111"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for unresolvable channel")
	}
}

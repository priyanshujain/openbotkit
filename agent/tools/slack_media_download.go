package tools

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/73ai/openbotkit/source/slack"
)

type SlackMediaDownloadTool struct {
	deps SlackToolDeps
}

func NewSlackMediaDownloadTool(deps SlackToolDeps) *SlackMediaDownloadTool {
	return &SlackMediaDownloadTool{deps: deps}
}

func (t *SlackMediaDownloadTool) Name() string { return "slack_media_download" }
func (t *SlackMediaDownloadTool) Description() string {
	return "Download a file attached to a Slack message and save it locally. Takes a url_private from a message's files[], or a /files/… permalink. Returns the saved path, which you can then read — screenshots often hold the actual error message."
}
func (t *SlackMediaDownloadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The file's url_private, or its /files/… permalink"
			},
			"filename": {
				"type": "string",
				"description": "Optional name for the saved file; defaults to the name in the URL"
			}
		},
		"required": ["url"]
	}`)
}

type slackMediaDownloadInput struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

func (t *SlackMediaDownloadTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in slackMediaDownloadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if in.URL == "" {
		return "", fmt.Errorf("url is required")
	}
	if t.deps.ScratchDir == "" {
		slog.Warn("slack_media_download: no scratch dir configured")
		return "", fmt.Errorf("no scratch directory configured, cannot save the file")
	}

	downloadURL, err := slack.ResolveFileURL(ctx, t.deps.Client, in.URL)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(t.deps.ScratchDir, 0700); err != nil {
		return "", fmt.Errorf("create scratch dir: %w", err)
	}
	path := filepath.Join(t.deps.ScratchDir, mediaFilename(in.Filename, downloadURL))

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if err := t.deps.Client.DownloadFile(ctx, downloadURL, f); err != nil {
		return "", err
	}

	info, err := f.Stat()
	if err != nil {
		return fmt.Sprintf("Saved to %s", path), nil
	}
	return fmt.Sprintf("Saved to %s (%d bytes). Use file_read to view it.", path, info.Size()), nil
}

var unsafeFilenameRe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// mediaFilename picks a safe base name, preferring the caller's, then the last
// path segment of the URL, then a random name.
func mediaFilename(requested, downloadURL string) string {
	name := requested
	if name == "" {
		trimmed, _, _ := strings.Cut(downloadURL, "?")
		name = filepath.Base(trimmed)
	}
	name = unsafeFilenameRe.ReplaceAllString(filepath.Base(name), "_")
	name = strings.TrimLeft(name, ".")
	if name == "" || name == "_" {
		name = "slack_file_" + rand.Text()
	}
	return "slack_" + name
}

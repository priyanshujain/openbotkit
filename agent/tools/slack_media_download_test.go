package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/73ai/openbotkit/source/slack"
)

func TestSlackMediaDownloadTool_Execute(t *testing.T) {
	dir := t.TempDir()
	api := &mockSlackAPI{downloadBody: "\x89PNG\r\n\x1a\nfake"}
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: api, ScratchDir: dir})

	input, _ := json.Marshal(slackMediaDownloadInput{
		URL: "https://files.slack.com/files-pri/T1-F1/shot.png",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "slack_shot.png")
	if !strings.Contains(result, path) {
		t.Errorf("result = %q, want the saved path", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != api.downloadBody {
		t.Errorf("file contents = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("mode = %v, want 0600", perm)
	}
}

func TestSlackMediaDownloadTool_ResolvesPermalink(t *testing.T) {
	dir := t.TempDir()
	api := &mockSlackAPI{
		downloadBody: "jpegdata",
		fileInfo: &slack.File{
			ID:         "F0BLMEDEJCS",
			URLPrivate: "https://files.slack.com/files-pri/T1-F0BLMEDEJCS/file.jpg",
		},
	}
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: api, ScratchDir: dir})

	input, _ := json.Marshal(slackMediaDownloadInput{
		URL: "https://acme.slack.com/files/U02CMP7DXU1/F0BLMEDEJCS/file.jpg",
	})
	if _, err := tool.Execute(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if api.downloadedURL != api.fileInfo.URLPrivate {
		t.Errorf("downloaded %q, want the url_private", api.downloadedURL)
	}
}

func TestSlackMediaDownloadTool_CustomFilename(t *testing.T) {
	dir := t.TempDir()
	api := &mockSlackAPI{downloadBody: "data"}
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: api, ScratchDir: dir})

	input, _ := json.Marshal(slackMediaDownloadInput{
		URL:      "https://files.slack.com/files-pri/T1-F1/shot.png",
		Filename: "customer_error.png",
	})
	if _, err := tool.Execute(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "slack_customer_error.png")); err != nil {
		t.Errorf("expected the custom name: %v", err)
	}
}

// A filename from Slack must not be able to escape the scratch directory.
func TestSlackMediaDownloadTool_FilenameStaysInScratch(t *testing.T) {
	dir := t.TempDir()
	api := &mockSlackAPI{downloadBody: "data"}
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: api, ScratchDir: dir})

	input, _ := json.Marshal(slackMediaDownloadInput{
		URL:      "https://files.slack.com/files-pri/T1-F1/shot.png",
		Filename: "../../escaped.png",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "..") {
		t.Errorf("path escaped the scratch dir: %q", result)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in scratch, got %d", len(entries))
	}
	if strings.Contains(entries[0].Name(), "/") || strings.HasPrefix(entries[0].Name(), ".") {
		t.Errorf("unsafe name %q", entries[0].Name())
	}
}

func TestSlackMediaDownloadTool_MissingParams(t *testing.T) {
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: &mockSlackAPI{}, ScratchDir: t.TempDir()})

	input, _ := json.Marshal(slackMediaDownloadInput{})
	if _, err := tool.Execute(context.Background(), input); err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestSlackMediaDownloadTool_NoScratchDir(t *testing.T) {
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: &mockSlackAPI{}})

	input, _ := json.Marshal(slackMediaDownloadInput{
		URL: "https://files.slack.com/files-pri/T1-F1/shot.png",
	})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error when no scratch dir is configured")
	}
	if !strings.Contains(err.Error(), "scratch") {
		t.Errorf("error = %q", err)
	}
}

func TestSlackMediaDownloadTool_Name(t *testing.T) {
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	if tool.Name() != "slack_media_download" {
		t.Errorf("Name() = %q", tool.Name())
	}
}

func TestSlackMediaDownloadTool_Metadata(t *testing.T) {
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	if tool.Description() == "" {
		t.Error("empty description")
	}
	if !json.Valid(tool.InputSchema()) {
		t.Error("invalid schema")
	}
}

func TestSlackMediaDownloadTool_InvalidJSON(t *testing.T) {
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: &mockSlackAPI{}})
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{bad`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSlackMediaDownloadTool_ResolveError(t *testing.T) {
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: &mockSlackAPI{}, ScratchDir: t.TempDir()})

	input, _ := json.Marshal(slackMediaDownloadInput{URL: "F0BLMEDEJCS"})
	if _, err := tool.Execute(context.Background(), input); err == nil {
		t.Fatal("expected error for a bare file ID")
	}
}

func TestSlackMediaDownloadTool_DownloadError(t *testing.T) {
	dir := t.TempDir()
	api := &mockSlackAPI{downloadErr: errors.New("got an HTML page instead of the file")}
	tool := NewSlackMediaDownloadTool(SlackToolDeps{Client: api, ScratchDir: dir})

	input, _ := json.Marshal(slackMediaDownloadInput{
		URL: "https://files.slack.com/files-pri/T1-F1/shot.png",
	})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTML page") {
		t.Errorf("error = %q", err)
	}
}

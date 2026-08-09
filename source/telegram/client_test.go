package telegram

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClientCreatesSessionDir(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "telegram", "session.json")

	client, err := NewClient(sessionPath, 12345, "hash", nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.API() == nil {
		t.Fatal("API() returned nil")
	}
	if client.Gaps() == nil {
		t.Fatal("Gaps() returned nil")
	}

	if _, err := os.Stat(filepath.Dir(sessionPath)); err != nil {
		t.Fatalf("session dir not created: %v", err)
	}
	if client.HasSession() {
		t.Fatal("a fresh client should not report a session")
	}
}

func TestNewClientRejectsEmptySessionPath(t *testing.T) {
	if _, err := NewClient("", 1, "hash", nil); err == nil {
		t.Fatal("expected an error for an empty session path")
	}
}

func TestHasSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	if HasSession(path) {
		t.Fatal("missing file should report no session")
	}

	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if HasSession(path) {
		t.Fatal("empty file should report no session")
	}

	if err := os.WriteFile(path, []byte(`{"Version":1}`), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !HasSession(path) {
		t.Fatal("non-empty file should report a session")
	}
}

func TestParseAPIID(t *testing.T) {
	if _, err := parseAPIID("not-a-number"); err == nil {
		t.Fatal("expected an error for a non-numeric api_id")
	}
	got, err := parseAPIID("1234567")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 1234567 {
		t.Fatalf("api_id = %d, want 1234567", got)
	}
}

package telegram

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gotd/td/tgerr"
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

func TestLogoutWithoutSession(t *testing.T) {
	client, err := NewClient(filepath.Join(t.TempDir(), "session.json"), 1, "hash", nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.Logout(context.Background()); err == nil {
		t.Fatal("expected an error when there is nothing to log out of")
	}
}

// A logout that never reached the server must leave the session file alone:
// the device stays authorised, so throwing away the local half would leave the
// user with no way to revoke it.
func TestLogoutKeepsSessionWhenRevocationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("not a session"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	client, err := NewClient(path, 1, "hash", nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Logout(ctx); err == nil {
		t.Fatal("expected the failed revocation to be reported")
	}
	if !HasSession(path) {
		t.Fatal("session file removed even though the server was never told")
	}
}

func TestSessionAlreadyRevoked(t *testing.T) {
	for _, code := range []string{"AUTH_KEY_UNREGISTERED", "SESSION_REVOKED", "USER_DEACTIVATED"} {
		if !sessionAlreadyRevoked(tgerr.New(401, code)) {
			t.Fatalf("%s should count as already revoked", code)
		}
	}
	if sessionAlreadyRevoked(tgerr.New(420, "FLOOD_WAIT")) {
		t.Fatal("a flood wait leaves the session alive")
	}
	if sessionAlreadyRevoked(errors.New("dial tcp: no route to host")) {
		t.Fatal("a network failure leaves the session alive")
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

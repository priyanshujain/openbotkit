package whatsapp

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewClient_CreatesSessionDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "session.db")

	ctx := context.Background()
	client, err := NewClient(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("client is nil")
	}

	if client.IsAuthenticated() {
		t.Error("new client should not be authenticated")
	}
}

func TestClient_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "session.db")

	ctx := context.Background()
	client, err := NewClient(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Close should not panic and should release the SQLite connection.
	client.Close()

	// After Close, creating a new client on the same DB should succeed
	// (proves the SQLite lock was released).
	client2, err := NewClient(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewClient after Close failed: %v", err)
	}
	client2.Close()
}

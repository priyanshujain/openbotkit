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

package daemon

import (
	"os"
	"testing"
)

func TestAcquireLock_Exclusive(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("OBK_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OBK_CONFIG_DIR")

	// First lock should succeed.
	f, err := acquireLock()
	if err != nil {
		t.Fatalf("first acquireLock failed: %v", err)
	}

	// Second lock should fail.
	_, err = acquireLock()
	if err == nil {
		t.Fatal("second acquireLock should have failed")
	}

	// Release first lock, third should succeed.
	releaseLock(f)

	f2, err := acquireLock()
	if err != nil {
		t.Fatalf("third acquireLock after release failed: %v", err)
	}
	releaseLock(f2)
}

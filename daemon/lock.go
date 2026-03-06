package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/priyanshujain/openbotkit/config"
)

func lockPath() string {
	return filepath.Join(config.Dir(), "daemon.lock")
}

// acquireLock takes an exclusive file lock to prevent multiple daemon instances.
// Returns the lock file which must be kept open for the lifetime of the daemon.
func acquireLock() (*os.File, error) {
	f, err := os.OpenFile(lockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon is already running")
	}

	return f, nil
}

// releaseLock releases the file lock and removes the lock file.
func releaseLock(f *os.File) {
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
	os.Remove(lockPath())
}

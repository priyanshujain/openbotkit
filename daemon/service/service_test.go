package service

import (
	"runtime"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	p := DetectPlatform()

	switch runtime.GOOS {
	case "darwin":
		if p != PlatformMacOS {
			t.Errorf("expected PlatformMacOS on darwin, got %s", p)
		}
	case "linux":
		if p != PlatformLinux {
			t.Errorf("expected PlatformLinux on linux, got %s", p)
		}
	default:
		if p != PlatformUnknown {
			t.Errorf("expected PlatformUnknown on %s, got %s", runtime.GOOS, p)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}

	if cfg.BinaryPath == "" {
		t.Error("BinaryPath is empty")
	}
	if cfg.LogPath == "" {
		t.Error("LogPath is empty")
	}
}

func TestNewManager(t *testing.T) {
	mgr, err := NewManager()

	switch runtime.GOOS {
	case "darwin", "linux":
		if err != nil {
			t.Fatalf("NewManager failed on %s: %v", runtime.GOOS, err)
		}
		if mgr == nil {
			t.Fatal("NewManager returned nil")
		}
	default:
		if err == nil {
			t.Error("NewManager should fail on unsupported platform")
		}
	}
}

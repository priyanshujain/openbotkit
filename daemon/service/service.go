package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Platform string

const (
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
	PlatformUnknown Platform = "unknown"
)

type ServiceConfig struct {
	BinaryPath string
	LogPath    string
}

func DetectPlatform() Platform {
	switch runtime.GOOS {
	case "darwin":
		return PlatformMacOS
	case "linux":
		return PlatformLinux
	default:
		return PlatformUnknown
	}
}

func DefaultConfig() (*ServiceConfig, error) {
	binPath, err := exec.LookPath("obk")
	if err != nil {
		binPath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("find obk binary: %w", err)
		}
	}
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return nil, fmt.Errorf("resolve binary path: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	return &ServiceConfig{
		BinaryPath: binPath,
		LogPath:    filepath.Join(home, ".obk", "daemon.log"),
	}, nil
}

type Manager interface {
	Install(cfg *ServiceConfig) error
	Uninstall() error
	Status() (string, error)
}

func NewManager() (Manager, error) {
	switch DetectPlatform() {
	case PlatformMacOS:
		return &launchdManager{}, nil
	case PlatformLinux:
		return &systemdManager{}, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

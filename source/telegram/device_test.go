package telegram

import (
	"runtime"
	"strings"
	"testing"
)

// gotd otherwise reports the Go runtime version as the device model and its own
// version as the app version, which shows up in Telegram's Devices screen as a
// meaningless "go1.26.4 / v0.161.0" entry.
func TestDeviceConfigDoesNotLeakGoInternals(t *testing.T) {
	d := deviceConfig()

	if d.DeviceModel == "" || d.SystemVersion == "" || d.AppVersion == "" {
		t.Fatalf("device config has empty fields: %+v", d)
	}
	if strings.Contains(d.DeviceModel, runtime.Version()) {
		t.Fatalf("device model leaks the Go version: %q", d.DeviceModel)
	}
	if d.SystemVersion == runtime.GOOS {
		t.Fatalf("system version should be a human-readable name, got %q", d.SystemVersion)
	}
	if d.Params == nil {
		t.Fatal("timezone params not set")
	}
}

func TestSetAppVersion(t *testing.T) {
	original := appVersion
	t.Cleanup(func() { appVersion = original })

	SetAppVersion("0.13.0")
	if got := deviceConfig().AppVersion; got != "0.13.0" {
		t.Fatalf("app version = %q, want 0.13.0", got)
	}

	// An empty stamp must not blank the reported version.
	SetAppVersion("")
	if got := deviceConfig().AppVersion; got != "0.13.0" {
		t.Fatalf("empty version overwrote the app version, got %q", got)
	}
}

func TestDeviceModelIsNotEmpty(t *testing.T) {
	got := deviceModel()
	if got == "" {
		t.Fatal("device model must not be empty")
	}
	// macOS hostnames arrive as "Some-MacBook-Pro.local"; the suffix is noise.
	if strings.HasSuffix(got, ".local") || strings.HasSuffix(got, ".") {
		t.Fatalf("device model keeps a hostname suffix: %q", got)
	}
}

func TestSystemVersionIsHumanReadable(t *testing.T) {
	got := systemVersion()
	if got == "" {
		t.Fatal("system version must not be empty")
	}
	if got == runtime.GOOS {
		t.Fatalf("GOOS %q should be mapped to a product name", got)
	}

	want := map[string]string{"darwin": "macOS", "linux": "Linux", "windows": "Windows"}[runtime.GOOS]
	if want != "" && got != want {
		t.Fatalf("system version = %q, want %q", got, want)
	}
}

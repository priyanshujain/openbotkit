package telegram

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
)

// appVersion is stamped at build time via internal/cli.Version and forwarded by
// SetAppVersion. It shows up next to the app name in Telegram's Devices screen.
var appVersion = "dev"

// SetAppVersion records the obk version to report to Telegram. Called once at
// startup; source packages cannot import internal/cli, which owns the
// ldflags-stamped value.
func SetAppVersion(v string) {
	if v != "" {
		appVersion = v
	}
}

// deviceConfig identifies this install in Telegram's Devices screen. Without it
// gotd reports the Go runtime version as the device model and its own version
// as the app version, which shows up as a meaningless "go1.26.4" entry.
//
// These values are deliberately honest. gotd also ships DeviceTDesktopWindows,
// which makes a session indistinguishable from Telegram Desktop, but passing
// ourselves off as an official client would make our own session unidentifiable
// in the user's device list.
func deviceConfig() telegram.DeviceConfig {
	return telegram.DeviceConfig{
		DeviceModel:    deviceModel(),
		SystemVersion:  systemVersion(),
		AppVersion:     appVersion,
		SystemLangCode: "en",
		LangCode:       "en",
		Params:         telegram.TimezoneParams(time.Local),
	}
}

// deviceModel names the machine, the way an official desktop client does, so
// several installs are distinguishable in the Devices list.
func deviceModel() string {
	host, err := os.Hostname()
	if err != nil {
		return "OpenBotKit"
	}
	// macOS hostnames arrive as "Some-MacBook-Pro.local".
	host = strings.TrimSuffix(strings.TrimSuffix(host, "."), ".local")
	host = strings.TrimSpace(host)
	if host == "" {
		return "OpenBotKit"
	}
	return host
}

// systemVersion reports the OS under a name a human recognises, rather than the
// Go GOOS spelling ("darwin").
func systemVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
	}
}

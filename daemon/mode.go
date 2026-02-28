package daemon

import "os"

type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeWorker     Mode = "worker"
)

// ParseMode returns the daemon mode from the given string.
// Falls back to OBK_MODE env var if s is empty.
// Defaults to ModeStandalone.
func ParseMode(s string) Mode {
	if s == "" {
		s = os.Getenv("OBK_MODE")
	}
	switch Mode(s) {
	case ModeWorker:
		return ModeWorker
	default:
		return ModeStandalone
	}
}

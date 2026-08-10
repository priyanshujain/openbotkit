package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open launches u in the user's default browser.
func Open(u string) error {
	name, args, err := openCommand(runtime.GOOS, u)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Start()
}

// openCommand maps a platform to the command that opens a URL. Split out from
// Open so the mapping is testable without launching a browser.
func openCommand(goos, u string) (name string, args []string, err error) {
	if u == "" {
		return "", nil, fmt.Errorf("empty URL")
	}
	switch goos {
	case "darwin":
		return "open", []string{u}, nil
	case "linux", "freebsd", "openbsd", "netbsd":
		return "xdg-open", []string{u}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", u}, nil
	default:
		return "", nil, fmt.Errorf("cannot open a browser on %s", goos)
	}
}

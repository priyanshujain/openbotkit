package daemon

import (
	"os"
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		env  string
		want Mode
	}{
		{name: "empty defaults to standalone", arg: "", env: "", want: ModeStandalone},
		{name: "explicit standalone", arg: "standalone", env: "", want: ModeStandalone},
		{name: "explicit worker", arg: "worker", env: "", want: ModeWorker},
		{name: "invalid defaults to standalone", arg: "invalid", env: "", want: ModeStandalone},
		{name: "env var worker", arg: "", env: "worker", want: ModeWorker},
		{name: "arg takes precedence over env", arg: "standalone", env: "worker", want: ModeStandalone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				os.Setenv("OBK_MODE", tt.env)
				defer os.Unsetenv("OBK_MODE")
			} else {
				os.Unsetenv("OBK_MODE")
			}

			got := ParseMode(tt.arg)
			if got != tt.want {
				t.Errorf("ParseMode(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

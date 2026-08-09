package browser

import "testing"

func TestOpenCommand(t *testing.T) {
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"darwin", "open", []string{"http://localhost:8086"}},
		{"linux", "xdg-open", []string{"http://localhost:8086"}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", "http://localhost:8086"}},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			name, args, err := openCommand(tt.goos, "http://localhost:8086")
			if err != nil {
				t.Fatalf("open command: %v", err)
			}
			if name != tt.wantName {
				t.Fatalf("name = %q, want %q", name, tt.wantName)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tt.wantArgs)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Fatalf("args = %v, want %v", args, tt.wantArgs)
				}
			}
		})
	}
}

func TestOpenCommandUnsupported(t *testing.T) {
	if _, _, err := openCommand("plan9", "http://localhost:8086"); err == nil {
		t.Fatal("expected an error on an unsupported platform")
	}
	if _, _, err := openCommand("darwin", ""); err == nil {
		t.Fatal("expected an error for an empty URL")
	}
}

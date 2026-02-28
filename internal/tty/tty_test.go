package tty

import "testing"

func TestRequireInteractiveMessage(t *testing.T) {
	// In test environment, stdin/stdout are not terminals,
	// so RequireInteractive should return an error.
	err := RequireInteractive("obk auth google login --scopes gmail.readonly")
	if err == nil {
		t.Skip("running in a real terminal, cannot test non-interactive path")
	}

	want := "this command requires an interactive terminal. Use: obk auth google login --scopes gmail.readonly"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestIsInteractiveReturnsFalseInTests(t *testing.T) {
	// Test processes don't have a real terminal attached.
	if IsInteractive() {
		t.Skip("running in a real terminal")
	}
}

package slack

import (
	"io"
	"strings"
	"testing"
)

func TestMediaDownloadCmd_RequiresOneURL(t *testing.T) {
	Cmd.SetOut(io.Discard)
	Cmd.SetErr(io.Discard)
	Cmd.SetArgs([]string{"media", "download"})
	t.Cleanup(func() { Cmd.SetArgs(nil) })

	err := Cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no URL is given")
	}
	if !strings.Contains(err.Error(), "arg") {
		t.Errorf("error = %q", err)
	}
}

func TestMediaCmd_Registered(t *testing.T) {
	var found bool
	for _, c := range Cmd.Commands() {
		if c.Name() == "media" {
			found = true
			for _, sub := range c.Commands() {
				if sub.Name() == "download" {
					return
				}
			}
			t.Fatal("media has no download subcommand")
		}
	}
	if !found {
		t.Fatal("media command not registered")
	}
}

// Under `go test` stdout is a pipe, so the binary-to-terminal guard stays off.
func TestStdoutIsTerminal_FalseWhenRedirected(t *testing.T) {
	if stdoutIsTerminal() {
		t.Error("stdout should not look like a terminal in tests")
	}
}

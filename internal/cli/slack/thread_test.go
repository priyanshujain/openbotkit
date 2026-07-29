package slack

import (
	"io"
	"strings"
	"testing"
)

// runThread executes the thread command with fresh flags and returns its error.
func runThread(t *testing.T, args ...string) error {
	t.Helper()
	threadCmd.Flags().Set("channel-id", "")
	threadCmd.Flags().Set("thread-id", "")
	threadCmd.Flags().Set("refresh-users", "false")
	Cmd.SetOut(io.Discard)
	Cmd.SetErr(io.Discard)
	Cmd.SetArgs(append([]string{"thread"}, args...))
	t.Cleanup(func() { Cmd.SetArgs(nil) })
	return Cmd.Execute()
}

func TestThreadCmd_RequiresAnAddress(t *testing.T) {
	err := runThread(t)
	if err == nil {
		t.Fatal("expected error when neither permalink nor flags are given")
	}
	if !strings.Contains(err.Error(), "--channel-id") {
		t.Errorf("error = %q", err)
	}
}

func TestThreadCmd_RejectsBothForms(t *testing.T) {
	err := runThread(t,
		"https://acme.slack.com/archives/C01ABC23DEF/p1785334150236249",
		"--channel-id", "C01ABC23DEF",
	)
	if err == nil {
		t.Fatal("expected error when both a permalink and flags are given")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("error = %q", err)
	}
}

func TestThreadCmd_RejectsPartialFlags(t *testing.T) {
	err := runThread(t, "--channel-id", "C01ABC23DEF")
	if err == nil {
		t.Fatal("expected error when --thread-id is missing")
	}
}

func TestThreadCmd_RejectsMalformedPermalink(t *testing.T) {
	err := runThread(t, "https://example.com/not-slack")
	if err == nil {
		t.Fatal("expected error for a non-Slack URL")
	}
	if !strings.Contains(err.Error(), "Slack permalink") {
		t.Errorf("error = %q", err)
	}
}

package slack

import (
	"io"
	"strings"
	"testing"
)

// runThread executes the thread command with fresh flags and returns its error.
// OBK_CONFIG_DIR points at an empty dir so the test never sees the developer's
// real Slack config and behaves the same here as in CI.
func runThread(t *testing.T, args ...string) error {
	t.Helper()
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
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

func TestThreadCmd_RejectsMalformedThreadID(t *testing.T) {
	err := runThread(t, "--channel-id", "C01ABC23DEF", "--thread-id", "p123")
	if err == nil {
		t.Fatal("expected error for a malformed message ID")
	}
	if !strings.Contains(err.Error(), "invalid message ID") {
		t.Errorf("error = %q", err)
	}
}

// Address parsing must not depend on credentials: with no workspace
// configured, malformed input still reports what is actually wrong.
func TestThreadCmd_AddressCheckedBeforeCredentials(t *testing.T) {
	err := runThread(t, "https://example.com/not-slack")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "no Slack workspace configured") {
		t.Errorf("credentials were loaded before validating input: %q", err)
	}
}

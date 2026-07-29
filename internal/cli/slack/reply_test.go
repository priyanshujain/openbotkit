package slack

import (
	"io"
	"strings"
	"testing"
)

// OBK_CONFIG_DIR points at an empty dir so the test never sees the developer's
// real Slack config and behaves the same here as in CI.
func runReply(t *testing.T, args ...string) error {
	t.Helper()
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	replyCmd.Flags().Set("channel-id", "")
	replyCmd.Flags().Set("thread-id", "")
	replyCmd.Flags().Set("text", "")
	Cmd.SetOut(io.Discard)
	Cmd.SetErr(io.Discard)
	Cmd.SetArgs(append([]string{"reply"}, args...))
	t.Cleanup(func() { Cmd.SetArgs(nil) })
	return Cmd.Execute()
}

func TestReplyCmd_RequiresAnAddress(t *testing.T) {
	err := runReply(t, "--text", "hello")
	if err == nil {
		t.Fatal("expected error when neither permalink nor flags are given")
	}
	if !strings.Contains(err.Error(), "--channel-id") {
		t.Errorf("error = %q", err)
	}
}

func TestReplyCmd_RejectsBothForms(t *testing.T) {
	err := runReply(t,
		"https://acme.slack.com/archives/C01ABC23DEF/p1785334150236249",
		"--channel-id", "C01ABC23DEF", "--text", "hello",
	)
	if err == nil {
		t.Fatal("expected error when both a permalink and flags are given")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("error = %q", err)
	}
}

// Under `go test` stdin is not a pipe, so an empty --text has no fallback.
func TestReplyCmd_RejectsEmptyBody(t *testing.T) {
	err := runReply(t, "https://acme.slack.com/archives/C01ABC23DEF/p1785334150236249")
	if err == nil {
		t.Fatal("expected error for an empty body")
	}
	if !strings.Contains(err.Error(), "empty message") {
		t.Errorf("error = %q", err)
	}
}

func TestReplyCmd_RejectsMalformedPermalink(t *testing.T) {
	err := runReply(t, "https://example.com/not-slack", "--text", "hello")
	if err == nil {
		t.Fatal("expected error for a non-Slack URL")
	}
	if !strings.Contains(err.Error(), "Slack permalink") {
		t.Errorf("error = %q", err)
	}
	if strings.Contains(err.Error(), "no Slack workspace configured") {
		t.Errorf("credentials were loaded before validating input: %q", err)
	}
}

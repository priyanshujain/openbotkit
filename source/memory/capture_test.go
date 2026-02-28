package memory

import (
	"os"
	"path/filepath"
	"testing"
)

const testTranscript = `{"type":"file-history-snapshot","messageId":"abc"}
{"type":"user","message":{"role":"user","content":"hello, how are you?"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","text":"let me think"},{"type":"text","text":"I'm doing well, thanks!"}]}}
{"type":"user","message":{"role":"user","content":"<local-command-stdout>some output</local-command-stdout>"}}
{"type":"user","message":{"role":"user","content":"what is 2+2?"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"2+2 equals 4."}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","text":"ok"}]}}
{"type":"progress","data":{"type":"hook_progress"}}
`

func writeTranscript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	return path
}

func TestCapture(t *testing.T) {
	db := testDB(t)
	path := writeTranscript(t, testTranscript)

	input := CaptureInput{
		SessionID:      "test-session",
		TranscriptPath: path,
		CWD:            "/tmp/project",
	}

	if err := Capture(db, input); err != nil {
		t.Fatalf("capture: %v", err)
	}

	count, err := CountConversations(db)
	if err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 conversation, got %d", count)
	}

	msgCount, err := MessageCountForSession(db, "test-session")
	if err != nil {
		t.Fatalf("count messages: %v", err)
	}
	// Should have: "hello, how are you?", "I'm doing well, thanks!", "what is 2+2?", "2+2 equals 4."
	// Skipped: file-history-snapshot, local-command-stdout, tool_result, progress
	if msgCount != 4 {
		t.Fatalf("expected 4 messages, got %d", msgCount)
	}
}

func TestCaptureIdempotent(t *testing.T) {
	db := testDB(t)
	path := writeTranscript(t, testTranscript)

	input := CaptureInput{
		SessionID:      "test-session",
		TranscriptPath: path,
		CWD:            "/tmp/project",
	}

	if err := Capture(db, input); err != nil {
		t.Fatalf("first capture: %v", err)
	}

	// Capture again — should be idempotent.
	if err := Capture(db, input); err != nil {
		t.Fatalf("second capture: %v", err)
	}

	msgCount, err := MessageCountForSession(db, "test-session")
	if err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if msgCount != 4 {
		t.Fatalf("expected 4 messages after idempotent capture, got %d", msgCount)
	}
}

func TestCaptureEmptyTranscript(t *testing.T) {
	db := testDB(t)
	path := writeTranscript(t, "")

	input := CaptureInput{
		SessionID:      "empty-session",
		TranscriptPath: path,
		CWD:            "/tmp",
	}

	if err := Capture(db, input); err != nil {
		t.Fatalf("capture empty: %v", err)
	}

	count, err := CountConversations(db)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 conversation (even empty), got %d", count)
	}

	msgCount, err := MessageCountForSession(db, "empty-session")
	if err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if msgCount != 0 {
		t.Fatalf("expected 0 messages, got %d", msgCount)
	}
}

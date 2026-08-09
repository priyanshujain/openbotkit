package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestExtractContentConversation(t *testing.T) {
	msg := &waE2E.Message{
		Conversation: proto.String("hello world"),
	}
	text, mediaType := extractContent(msg)
	if text != "hello world" {
		t.Fatalf("expected 'hello world', got %q", text)
	}
	if mediaType != "" {
		t.Fatalf("expected empty media type, got %q", mediaType)
	}
}

func TestExtractContentExtendedText(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("extended text here"),
		},
	}
	text, mediaType := extractContent(msg)
	if text != "extended text here" {
		t.Fatalf("expected 'extended text here', got %q", text)
	}
	if mediaType != "" {
		t.Fatalf("expected empty media type, got %q", mediaType)
	}
}

func TestExtractContentImageMessage(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("my photo"),
		},
	}
	text, mediaType := extractContent(msg)
	if text != "my photo" {
		t.Fatalf("expected 'my photo', got %q", text)
	}
	if mediaType != "image" {
		t.Fatalf("expected 'image', got %q", mediaType)
	}
}

func TestExtractContentVideoMessage(t *testing.T) {
	msg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption: proto.String("my video"),
		},
	}
	text, mediaType := extractContent(msg)
	if text != "my video" {
		t.Fatalf("expected 'my video', got %q", text)
	}
	if mediaType != "video" {
		t.Fatalf("expected 'video', got %q", mediaType)
	}
}

func TestExtractContentAudioMessage(t *testing.T) {
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{},
	}
	text, mediaType := extractContent(msg)
	if text != "" {
		t.Fatalf("expected empty text for audio, got %q", text)
	}
	if mediaType != "audio" {
		t.Fatalf("expected 'audio', got %q", mediaType)
	}
}

func TestExtractContentDocumentMessage(t *testing.T) {
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Caption: proto.String("report.pdf"),
		},
	}
	text, mediaType := extractContent(msg)
	if text != "report.pdf" {
		t.Fatalf("expected 'report.pdf', got %q", text)
	}
	if mediaType != "document" {
		t.Fatalf("expected 'document', got %q", mediaType)
	}
}

func TestExtractContentNilMessage(t *testing.T) {
	text, mediaType := extractContent(nil)
	if text != "" || mediaType != "" {
		t.Fatalf("expected empty for nil, got %q, %q", text, mediaType)
	}
}

func TestExtractContentEmptyMessage(t *testing.T) {
	msg := &waE2E.Message{}
	text, mediaType := extractContent(msg)
	if text != "" || mediaType != "" {
		t.Fatalf("expected empty for empty message, got %q, %q", text, mediaType)
	}
}

func TestGetContextInfoExtendedText(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("reply"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("original-msg-id"),
			},
		},
	}
	ci := getContextInfo(msg)
	if ci == nil {
		t.Fatal("expected context info")
	}
	if ci.GetStanzaID() != "original-msg-id" {
		t.Fatalf("expected stanza id 'original-msg-id', got %q", ci.GetStanzaID())
	}
}

func TestGetContextInfoImage(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("img-reply-id"),
			},
		},
	}
	ci := getContextInfo(msg)
	if ci == nil {
		t.Fatal("expected context info from image")
	}
	if ci.GetStanzaID() != "img-reply-id" {
		t.Fatalf("expected 'img-reply-id', got %q", ci.GetStanzaID())
	}
}

func TestGetContextInfoNil(t *testing.T) {
	msg := &waE2E.Message{
		Conversation: proto.String("plain message"),
	}
	ci := getContextInfo(msg)
	if ci != nil {
		t.Fatal("expected nil context info for plain conversation message")
	}
}

func TestTruncate(t *testing.T) {
	if r := truncate("short", 10); r != "short" {
		t.Fatalf("expected 'short', got %q", r)
	}
	if r := truncate("exactly ten", 11); r != "exactly ten" {
		t.Fatalf("expected 'exactly ten', got %q", r)
	}
	if r := truncate("this is a longer string", 10); r != "this is a ..." {
		t.Fatalf("expected truncated, got %q", r)
	}
	if r := truncate("", 5); r != "" {
		t.Fatalf("expected empty, got %q", r)
	}
}

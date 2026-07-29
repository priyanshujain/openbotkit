package slack

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessage_PreservesBlocksAndAttachments(t *testing.T) {
	raw := `{
		"ts": "1785334150.236249",
		"user": "U02ABC",
		"text": "Alert fired",
		"blocks": [{"type": "section", "text": {"type": "mrkdwn", "text": "*boom*"}}],
		"attachments": [{"color": "danger", "title": "PaymentFailed", "text": "500 from upstream"}],
		"edited": {"user": "U02ABC", "ts": "1785334200.000000"}
	}`

	var msg Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatal(err)
	}

	if len(msg.Blocks) == 0 {
		t.Fatal("blocks dropped")
	}
	if !strings.Contains(string(msg.Attachments), "PaymentFailed") {
		t.Errorf("attachments = %s", msg.Attachments)
	}
	if msg.Edited == nil || msg.Edited.TS != "1785334200.000000" {
		t.Errorf("edited = %+v", msg.Edited)
	}

	out, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"500 from upstream", "mrkdwn", `"edited"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("round trip lost %q: %s", want, out)
		}
	}
}

func TestMessage_OmitsDerivedAndEmptyFields(t *testing.T) {
	out, err := json.Marshal(Message{TS: "1.2", Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"blocks", "attachments", "edited", "message_id", "user_name"} {
		if strings.Contains(string(out), unwanted) {
			t.Errorf("expected %q to be omitted: %s", unwanted, out)
		}
	}
}

func TestFile_ParsesMetadata(t *testing.T) {
	raw := `{
		"id": "F123",
		"name": "screenshot.png",
		"filetype": "png",
		"mimetype": "image/png",
		"url_private": "https://files.slack.com/files-pri/T1-F123/screenshot.png",
		"url_private_download": "https://files.slack.com/files-pri/T1-F123/download/screenshot.png",
		"permalink": "https://okcredit.slack.com/files/U02ABC/F123/screenshot.png",
		"preview": "traceback...",
		"plain_text": "panic: nil map",
		"size": 4096
	}`

	var f File
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatal(err)
	}
	if f.Mimetype != "image/png" {
		t.Errorf("mimetype = %q", f.Mimetype)
	}
	if f.URLPrivateDownload == "" {
		t.Error("url_private_download dropped")
	}
	if f.Permalink == "" {
		t.Error("permalink dropped")
	}
	if f.Preview != "traceback..." {
		t.Errorf("preview = %q", f.Preview)
	}
	if f.PlainText != "panic: nil map" {
		t.Errorf("plain_text = %q", f.PlainText)
	}
}

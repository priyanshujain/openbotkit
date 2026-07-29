package slack

import "testing"

func TestParsePermalink_Basic(t *testing.T) {
	ref, err := ParsePermalink("https://okcredit.slack.com/archives/C01Q9G9CD6X/p1785334150236249")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ChannelID != "C01Q9G9CD6X" {
		t.Errorf("channel = %q", ref.ChannelID)
	}
	if ref.MessageID != "p1785334150236249" {
		t.Errorf("message = %q", ref.MessageID)
	}
	if ref.ThreadTS != "" {
		t.Errorf("thread_ts = %q, want empty", ref.ThreadTS)
	}
	if ref.Subdomain != "okcredit" {
		t.Errorf("subdomain = %q", ref.Subdomain)
	}

	root, err := ref.RootTS()
	if err != nil {
		t.Fatal(err)
	}
	if root != "1785334150.236249" {
		t.Errorf("root = %q", root)
	}
}

func TestParsePermalink_ReplyWithThreadTS(t *testing.T) {
	raw := "https://okcredit.slack.com/archives/C094FL95GDV/p1785319451036269?thread_ts=1785319311.419309&cid=C094FL95GDV"
	ref, err := ParsePermalink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ref.MessageID != "p1785319451036269" {
		t.Errorf("message = %q", ref.MessageID)
	}
	if ref.ThreadTS != "1785319311.419309" {
		t.Errorf("thread_ts = %q", ref.ThreadTS)
	}

	// The root is the thread_ts, not the message's own ts.
	root, err := ref.RootTS()
	if err != nil {
		t.Fatal(err)
	}
	if root != "1785319311.419309" {
		t.Errorf("root = %q", root)
	}
	ts, err := ref.TS()
	if err != nil {
		t.Fatal(err)
	}
	if ts != "1785319451.036269" {
		t.Errorf("ts = %q", ts)
	}
}

func TestParsePermalink_DM(t *testing.T) {
	ref, err := ParsePermalink("https://okcredit.slack.com/archives/D01ABC23DEF/p1785334150236249")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ChannelID != "D01ABC23DEF" {
		t.Errorf("channel = %q", ref.ChannelID)
	}
}

func TestParsePermalink_PrivateGroup(t *testing.T) {
	ref, err := ParsePermalink("https://okcredit.slack.com/archives/G01ABC23DEF/p1785334150236249")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ChannelID != "G01ABC23DEF" {
		t.Errorf("channel = %q", ref.ChannelID)
	}
}

func TestParsePermalink_Errors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"not a url", "hello world"},
		{"wrong host", "https://example.com/archives/C123/p1785334150236249"},
		{"no archives segment", "https://okcredit.slack.com/messages/C123/p1785334150236249"},
		{"missing message", "https://okcredit.slack.com/archives/C01Q9G9CD6X"},
		{"bad channel id", "https://okcredit.slack.com/archives/X01Q9G/p1785334150236249"},
		{"bad message id", "https://okcredit.slack.com/archives/C01Q9G9CD6X/pnotanumber"},
		{"message id too short", "https://okcredit.slack.com/archives/C01Q9G9CD6X/p12345"},
		{"bad thread_ts", "https://okcredit.slack.com/archives/C01Q9G9CD6X/p1785334150236249?thread_ts=abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePermalink(tc.raw); err == nil {
				t.Fatalf("expected error for %q", tc.raw)
			}
		})
	}
}

func TestMessageIDToTS(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"p1785334150236249", "1785334150.236249"},
		{"1785334150.236249", "1785334150.236249"},
		{"p1785334150.236249", "1785334150.236249"},
		{"1785334150236249", "1785334150.236249"},
	}
	for _, tc := range cases {
		got, err := MessageIDToTS(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("MessageIDToTS(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMessageIDToTS_Invalid(t *testing.T) {
	for _, in := range []string{"", "p", "pabc", "p12345", "1785334150.", ".236249", "17853x4150.236249"} {
		if got, err := MessageIDToTS(in); err == nil {
			t.Errorf("MessageIDToTS(%q) = %q, want error", in, got)
		}
	}
}

func TestTSToMessageID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1785334150.236249", "p1785334150236249"},
		{"p1785334150236249", "p1785334150236249"},
	}
	for _, tc := range cases {
		if got := TSToMessageID(tc.in); got != tc.want {
			t.Errorf("TSToMessageID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMessageIDTSRoundTrip(t *testing.T) {
	for _, id := range []string{"p1785334150236249", "p1785319451036269", "p1000000000000001"} {
		ts, err := MessageIDToTS(id)
		if err != nil {
			t.Fatalf("%q: %v", id, err)
		}
		if got := TSToMessageID(ts); got != id {
			t.Errorf("round trip %q -> %q -> %q", id, ts, got)
		}
	}
}

func TestMatchesWorkspace(t *testing.T) {
	cases := []struct {
		subdomain string
		workspace string
		want      bool
	}{
		{"okcredit", "okcredit", true},
		{"okcredit", "OkCredit", true},
		{"acmecorp", "Acme Corp", true},
		{"acme-corp", "Acme Corp", true},
		{"okcredit", "otherteam", false},
		{"", "okcredit", true},
		{"okcredit", "", true},
	}
	for _, tc := range cases {
		if got := MatchesWorkspace(tc.subdomain, tc.workspace); got != tc.want {
			t.Errorf("MatchesWorkspace(%q, %q) = %v, want %v", tc.subdomain, tc.workspace, got, tc.want)
		}
	}
}

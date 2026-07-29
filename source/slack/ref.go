package slack

import (
	"fmt"
	"net/url"
	"strings"
)

// MessageRef identifies a single Slack message parsed from a permalink.
type MessageRef struct {
	ChannelID string // C…/G…/D…
	MessageID string // p-form, e.g. p1785319451036269
	ThreadTS  string // dotted root ts from ?thread_ts=, empty if absent
	Subdomain string // "okcredit", from the permalink host
}

// TS returns the dotted timestamp of the referenced message.
func (r *MessageRef) TS() (string, error) {
	return MessageIDToTS(r.MessageID)
}

// RootTS returns the thread root timestamp: the permalink's thread_ts when
// present, otherwise the message's own timestamp.
func (r *MessageRef) RootTS() (string, error) {
	if r.ThreadTS != "" {
		return MessageIDToTS(r.ThreadTS)
	}
	return MessageIDToTS(r.MessageID)
}

// ParsePermalink parses a Slack message permalink into its parts.
func ParsePermalink(raw string) (*MessageRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty permalink")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse permalink %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("not a Slack permalink: %q", raw)
	}

	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".slack.com") {
		return nil, fmt.Errorf("not a Slack permalink: %q", raw)
	}
	subdomain, _, _ := strings.Cut(host, ".")

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "archives" {
		return nil, fmt.Errorf("permalink %q is missing /archives/<channel>/<message>", raw)
	}

	channelID := parts[1]
	if !isChannelID(channelID) {
		return nil, fmt.Errorf("invalid channel ID %q in permalink", channelID)
	}

	ts, err := MessageIDToTS(parts[2])
	if err != nil {
		return nil, fmt.Errorf("permalink %q: %w", raw, err)
	}

	ref := &MessageRef{
		ChannelID: channelID,
		MessageID: TSToMessageID(ts),
		Subdomain: subdomain,
	}

	if threadTS := u.Query().Get("thread_ts"); threadTS != "" {
		root, err := MessageIDToTS(threadTS)
		if err != nil {
			return nil, fmt.Errorf("permalink %q thread_ts: %w", raw, err)
		}
		ref.ThreadTS = root
	}
	return ref, nil
}

// MessageIDToTS converts a p-form message ID to a dotted timestamp:
// p1785319451036269 -> 1785319451.036269. A dotted timestamp is accepted
// and returned unchanged, so callers can pass either form.
func MessageIDToTS(id string) (string, error) {
	s := strings.TrimSpace(id)
	if s == "" {
		return "", fmt.Errorf("empty message ID")
	}

	if seconds, micros, ok := strings.Cut(s, "."); ok {
		seconds = strings.TrimPrefix(seconds, "p")
		if !allDigits(seconds) || !allDigits(micros) {
			return "", fmt.Errorf("invalid timestamp %q", id)
		}
		return seconds + "." + micros, nil
	}

	digits := strings.TrimPrefix(s, "p")
	if !allDigits(digits) || len(digits) <= 6 {
		return "", fmt.Errorf("invalid message ID %q (want p1785319451036269)", id)
	}
	return digits[:len(digits)-6] + "." + digits[len(digits)-6:], nil
}

// TSToMessageID converts a dotted timestamp to the p-form used in permalinks.
func TSToMessageID(ts string) string {
	s := strings.TrimPrefix(strings.TrimSpace(ts), "p")
	return "p" + strings.ReplaceAll(s, ".", "")
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// MatchesWorkspace reports whether a permalink subdomain plausibly refers to
// the given workspace name. Comparison ignores case and separators, so the
// team name "OkCredit" matches the subdomain "okcredit".
func MatchesWorkspace(subdomain, workspace string) bool {
	if subdomain == "" || workspace == "" {
		return true
	}
	return normalizeWorkspace(subdomain) == normalizeWorkspace(workspace)
}

func normalizeWorkspace(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		}
	}
	return b.String()
}

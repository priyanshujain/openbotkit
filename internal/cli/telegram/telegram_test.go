package telegram

import (
	"strings"
	"testing"
	"time"

	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/73ai/openbotkit/store"
)

func TestParseSince(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		in      string
		wantAgo time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"12h", 12 * time.Hour},
		{"90m", 90 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseSince(tt.in, 90)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			diff := now.Sub(got) - tt.wantAgo
			if diff < -time.Minute || diff > time.Minute {
				t.Fatalf("parseSince(%q) = %v, want about %v ago", tt.in, got, tt.wantAgo)
			}
		})
	}
}

func TestParseSinceDefaultsToConfiguredWindow(t *testing.T) {
	got, err := parseSince("", 45)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diff := time.Since(got) - 45*24*time.Hour
	if diff < -time.Minute || diff > time.Minute {
		t.Fatalf("empty --since should use the configured window, got %v", got)
	}
}

func TestParseSinceAbsoluteDate(t *testing.T) {
	got, err := parseSince("2026-01-15", 90)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseSinceAllMeansUnbounded(t *testing.T) {
	got, err := parseSince("all", 90)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("\"all\" should be unbounded, got %v", got)
	}
}

func TestParseSinceInvalid(t *testing.T) {
	for _, in := range []string{"yesterday", "30x", "2026-13-45"} {
		if _, err := parseSince(in, 90); err == nil {
			t.Fatalf("expected an error for %q", in)
		}
	}
}

// A negative window is a future timestamp, which makes every message "too old"
// and backfills nothing while reporting success.
func TestParseSinceRejectsNegativeWindows(t *testing.T) {
	for _, in := range []string{"-30d", "-12h"} {
		if _, err := parseSince(in, 90); err == nil {
			t.Fatalf("expected an error for %q", in)
		}
	}
}

func sendTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(store.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := tgsrc.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestResolveChatIDNumeric(t *testing.T) {
	db := sendTestDB(t)

	got, err := resolveChatID(db, "-1001234567890")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != -1001234567890 {
		t.Fatalf("chat id = %d", got)
	}
}

func TestResolveChatIDByUsernameAndTitle(t *testing.T) {
	db := sendTestDB(t)

	now := time.Now().UTC()
	if err := tgsrc.UpsertChat(db, &tgsrc.Chat{
		ChatID: tgsrc.ChatIDFromUser(11), Type: tgsrc.PeerUser,
		Title: "Ann Lee", Username: "ann", LastMessageAt: &now,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, in := range []string{"@ann", "ann", "Ann Lee"} {
		got, err := resolveChatID(db, in)
		if err != nil {
			t.Fatalf("resolve %q: %v", in, err)
		}
		if got != tgsrc.ChatIDFromUser(11) {
			t.Fatalf("resolve %q = %d", in, got)
		}
	}
}

func TestResolveChatIDNoMatch(t *testing.T) {
	db := sendTestDB(t)

	_, err := resolveChatID(db, "nobody")
	if err == nil {
		t.Fatal("expected an error when nothing matches")
	}
	if !strings.Contains(err.Error(), "no chat matches") {
		t.Fatalf("error = %v", err)
	}
}

// An ambiguous name must be reported, never guessed: sending to the wrong
// person is not recoverable.
func TestResolveChatIDAmbiguous(t *testing.T) {
	db := sendTestDB(t)

	now := time.Now().UTC()
	for _, c := range []*tgsrc.Chat{
		{ChatID: tgsrc.ChatIDFromUser(11), Type: tgsrc.PeerUser, Title: "Ann Lee", LastMessageAt: &now},
		{ChatID: tgsrc.ChatIDFromUser(12), Type: tgsrc.PeerUser, Title: "Ann Smith", LastMessageAt: &now},
	} {
		if err := tgsrc.UpsertChat(db, c); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	_, err := resolveChatID(db, "Ann")
	if err == nil {
		t.Fatal("expected an error for an ambiguous name")
	}
	for _, want := range []string{"matches 2 chats", "Ann Lee", "Ann Smith"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q is missing %q", err, want)
		}
	}
}

// "@ann" is an explicit username. A group titled "Announcements" contains
// "ann", and resolving to it would send the message to the wrong place with no
// confirmation, since a single match sends immediately.
func TestResolveChatIDUsernameNeverMatchesBySubstring(t *testing.T) {
	db := sendTestDB(t)

	now := time.Now().UTC()
	for _, c := range []*tgsrc.Chat{
		{ChatID: tgsrc.ChatIDFromUser(11), Type: tgsrc.PeerUser, Title: "Ann Lee", Username: "ann", LastMessageAt: &now},
		{ChatID: tgsrc.ChatIDFromChannel(22), Type: tgsrc.PeerChannel, Title: "Announcements", IsChannel: true, LastMessageAt: &now},
	} {
		if err := tgsrc.UpsertChat(db, c); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := resolveChatID(db, "@ann")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != tgsrc.ChatIDFromUser(11) {
		t.Fatalf("@ann resolved to %d, want the user with that username", got)
	}
}

// Without an exact username hit, a @name is an error rather than a guess.
func TestResolveChatIDUnknownUsername(t *testing.T) {
	db := sendTestDB(t)

	now := time.Now().UTC()
	if err := tgsrc.UpsertChat(db, &tgsrc.Chat{
		ChatID: tgsrc.ChatIDFromChannel(22), Type: tgsrc.PeerChannel,
		Title: "Announcements", IsChannel: true, LastMessageAt: &now,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := resolveChatID(db, "@ann")
	if err == nil {
		t.Fatal("expected an error rather than a substring match on the title")
	}
	if !strings.Contains(err.Error(), "no chat has the username") {
		t.Fatalf("error = %v", err)
	}
}

// An exact title beats a longer one that merely contains it.
func TestResolveChatIDPrefersExactTitle(t *testing.T) {
	db := sendTestDB(t)

	now := time.Now().UTC()
	for _, c := range []*tgsrc.Chat{
		{ChatID: tgsrc.ChatIDFromUser(11), Type: tgsrc.PeerUser, Title: "Ann", LastMessageAt: &now},
		{ChatID: tgsrc.ChatIDFromUser(12), Type: tgsrc.PeerUser, Title: "Ann Smith", LastMessageAt: &now},
	} {
		if err := tgsrc.UpsertChat(db, c); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := resolveChatID(db, "Ann")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != tgsrc.ChatIDFromUser(11) {
		t.Fatalf("resolved to %d, want the exactly titled chat", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("abcdefghij", 8); got != "abcde..." {
		t.Fatalf("got %q", got)
	}
	// Multi-byte text is cut by rune, not by byte.
	if got := truncate("héllo wörld", 8); got != "héllo..." {
		t.Fatalf("got %q", got)
	}
}

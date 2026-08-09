package telegram

import (
	"testing"
	"time"

	"github.com/gotd/td/tg"
)

func TestInputPeerFor(t *testing.T) {
	tests := []struct {
		name string
		chat *Chat
		want tg.InputPeerClass
	}{
		{
			name: "user",
			chat: &Chat{ChatID: ChatIDFromUser(11), AccessHash: 1111},
			want: &tg.InputPeerUser{UserID: 11, AccessHash: 1111},
		},
		{
			name: "group",
			chat: &Chat{ChatID: ChatIDFromGroup(31)},
			want: &tg.InputPeerChat{ChatID: 31},
		},
		{
			name: "channel",
			chat: &Chat{ChatID: ChatIDFromChannel(41), AccessHash: 4141},
			want: &tg.InputPeerChannel{ChannelID: 41, AccessHash: 4141},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InputPeerFor(tt.chat)
			if err != nil {
				t.Fatalf("input peer: %v", err)
			}
			if got.String() != tt.want.String() {
				t.Fatalf("peer = %v, want %v", got, tt.want)
			}
		})
	}
}

// Legacy chats have no access hash, but users and channels do; sending without
// one would fail server-side with a confusing error.
func TestInputPeerForMissingAccessHash(t *testing.T) {
	if _, err := InputPeerFor(&Chat{ChatID: ChatIDFromUser(11)}); err == nil {
		t.Fatal("expected an error for a user with no access hash")
	}
	if _, err := InputPeerFor(&Chat{ChatID: ChatIDFromChannel(41)}); err == nil {
		t.Fatal("expected an error for a channel with no access hash")
	}
	if _, err := InputPeerFor(&Chat{ChatID: ChatIDFromGroup(31)}); err != nil {
		t.Fatalf("legacy chats need no access hash: %v", err)
	}
}

func TestSentMessageFrom(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	sent := &tg.Message{ID: 42, Date: int(now.Unix())}

	tests := []struct {
		name    string
		updates tg.UpdatesClass
		wantID  int
	}{
		{
			name:    "short sent message",
			updates: &tg.UpdateShortSentMessage{ID: 42, Date: int(now.Unix())},
			wantID:  42,
		},
		{
			name: "updates with message id",
			updates: &tg.Updates{
				Updates: []tg.UpdateClass{&tg.UpdateMessageID{ID: 42}},
				Date:    int(now.Unix()),
			},
			wantID: 42,
		},
		{
			name: "updates with new message",
			updates: &tg.Updates{
				Updates: []tg.UpdateClass{&tg.UpdateNewMessage{Message: sent}},
			},
			wantID: 42,
		},
		{
			name: "updates with new channel message",
			updates: &tg.UpdatesCombined{
				Updates: []tg.UpdateClass{&tg.UpdateNewChannelMessage{Message: sent}},
			},
			wantID: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sentMessageFrom(tt.updates)
			if got.MessageID != tt.wantID {
				t.Fatalf("message_id = %d, want %d", got.MessageID, tt.wantID)
			}
			if !got.Timestamp.Equal(now) {
				t.Fatalf("timestamp = %v, want %v", got.Timestamp, now)
			}
		})
	}
}

func TestSentMessageFromUnknownContainer(t *testing.T) {
	got := sentMessageFrom(&tg.UpdatesTooLong{})
	if got.MessageID != 0 || !got.Timestamp.IsZero() {
		t.Fatalf("expected a zero result, got %+v", got)
	}
}

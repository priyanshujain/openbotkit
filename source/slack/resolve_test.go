package slack

import (
	"context"
	"testing"
)

type mockAPI struct {
	channels []Channel
	users    []User
}

func (m *mockAPI) SearchMessages(context.Context, string, SearchOptions) (*SearchResult, error) {
	return nil, nil
}
func (m *mockAPI) SearchFiles(context.Context, string, SearchOptions) (*FileSearchResult, error) {
	return nil, nil
}
func (m *mockAPI) ConversationsHistory(context.Context, string, HistoryOptions) ([]Message, error) {
	return nil, nil
}
func (m *mockAPI) ConversationsReplies(context.Context, string, string, HistoryOptions) ([]Message, error) {
	return nil, nil
}
func (m *mockAPI) ConversationsList(context.Context) ([]Channel, error) {
	return m.channels, nil
}
func (m *mockAPI) UsersList(context.Context) ([]User, error) { return m.users, nil }
func (m *mockAPI) UsersInfo(context.Context, string) (*User, error) {
	return nil, nil
}
func (m *mockAPI) PostMessage(context.Context, string, string, string) (string, error) {
	return "", nil
}
func (m *mockAPI) UpdateMessage(context.Context, string, string, string) error { return nil }
func (m *mockAPI) DeleteMessage(context.Context, string, string) error         { return nil }
func (m *mockAPI) AddReaction(context.Context, string, string, string) error   { return nil }
func (m *mockAPI) RemoveReaction(context.Context, string, string, string) error {
	return nil
}
func (m *mockAPI) ResolveChannel(context.Context, string) (string, error) { return "", nil }
func (m *mockAPI) ResolveUser(context.Context, string) (string, error)    { return "", nil }

func TestResolveChannel_ID(t *testing.T) {
	r := NewResolver(&mockAPI{})
	id, err := r.ResolveChannel(context.Background(), "C0123ABC")
	if err != nil {
		t.Fatal(err)
	}
	if id != "C0123ABC" {
		t.Errorf("got %q", id)
	}
}

func TestResolveChannel_Name(t *testing.T) {
	r := NewResolver(&mockAPI{
		channels: []Channel{{ID: "C111", Name: "general"}, {ID: "C222", Name: "random"}},
	})
	id, err := r.ResolveChannel(context.Background(), "#general")
	if err != nil {
		t.Fatal(err)
	}
	if id != "C111" {
		t.Errorf("got %q", id)
	}
}

func TestResolveChannel_URL(t *testing.T) {
	r := NewResolver(&mockAPI{})
	id, err := r.ResolveChannel(context.Background(), "https://team.slack.com/archives/C0123ABC")
	if err != nil {
		t.Fatal(err)
	}
	if id != "C0123ABC" {
		t.Errorf("got %q", id)
	}
}

func TestResolveChannel_DMID(t *testing.T) {
	r := NewResolver(&mockAPI{})
	id, err := r.ResolveChannel(context.Background(), "D0123ABC")
	if err != nil {
		t.Fatal(err)
	}
	if id != "D0123ABC" {
		t.Errorf("got %q", id)
	}
}

func TestResolveChannel_GroupID(t *testing.T) {
	r := NewResolver(&mockAPI{})
	id, err := r.ResolveChannel(context.Background(), "G0123ABC")
	if err != nil {
		t.Fatal(err)
	}
	if id != "G0123ABC" {
		t.Errorf("got %q", id)
	}
}

func TestResolveChannel_NotFound(t *testing.T) {
	r := NewResolver(&mockAPI{channels: []Channel{}})
	_, err := r.ResolveChannel(context.Background(), "#nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveUser_ID(t *testing.T) {
	r := NewResolver(&mockAPI{})
	id, err := r.ResolveUser(context.Background(), "U12345")
	if err != nil {
		t.Fatal(err)
	}
	if id != "U12345" {
		t.Errorf("got %q", id)
	}
}

func TestResolveUser_Handle(t *testing.T) {
	r := NewResolver(&mockAPI{
		users: []User{{ID: "U1", Name: "alice"}, {ID: "U2", Name: "bob"}},
	})
	id, err := r.ResolveUser(context.Background(), "@alice")
	if err != nil {
		t.Fatal(err)
	}
	if id != "U1" {
		t.Errorf("got %q", id)
	}
}

func TestResolveUser_Email(t *testing.T) {
	r := NewResolver(&mockAPI{
		users: []User{{ID: "U1", Name: "alice", Profile: &UserProfile{Email: "alice@example.com"}}},
	})
	id, err := r.ResolveUser(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != "U1" {
		t.Errorf("got %q", id)
	}
}

func TestResolveChannel_Caching(t *testing.T) {
	calls := 0
	api := &mockAPI{channels: []Channel{{ID: "C1", Name: "test"}}}
	origList := api.ConversationsList
	_ = origList
	// Wrap in counting resolver
	r := NewResolver(api)

	// We can't easily count calls through the interface, but we can verify
	// the resolver returns cached data by modifying the source after first call.
	_, _ = r.ResolveChannel(context.Background(), "#test")

	// Modify source data — should not affect cached result.
	api.channels = nil
	id, err := r.ResolveChannel(context.Background(), "#test")
	if err != nil {
		t.Fatal(err)
	}
	if id != "C1" {
		t.Errorf("cache miss: got %q", id)
	}
	_ = calls
}

func TestIsChannelID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"C0123ABC", true},
		{"G0123ABC", true},
		{"C1", true},
		{"U0123", false},
		{"general", false},
		{"", false},
		{"C", false},
	}
	for _, tt := range tests {
		if got := isChannelID(tt.input); got != tt.want {
			t.Errorf("isChannelID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsUserID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"U12345", true},
		{"W12345", true},
		{"C12345", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isUserID(tt.input); got != tt.want {
			t.Errorf("isUserID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

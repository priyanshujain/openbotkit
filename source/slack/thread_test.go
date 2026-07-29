package slack

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type threadAPI struct {
	userCacheAPI
	byTS      map[string][]Message
	requested []string
	err       error
}

func (a *threadAPI) ConversationsRepliesAll(_ context.Context, _ string, ts string, _ HistoryOptions) ([]Message, error) {
	a.requested = append(a.requested, ts)
	if a.err != nil {
		return nil, a.err
	}
	return a.byTS[ts], nil
}

func TestFetchThread_Basic(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &threadAPI{
		byTS: map[string][]Message{
			"1785334150.236249": {
				{TS: "1785334150.236249", User: "U1", Text: "customer cannot settle"},
				{TS: "1785334200.111111", User: "U2", Text: "looking", ThreadTS: "1785334150.236249"},
			},
		},
	}
	api.users = map[string]*User{
		"U1": {ID: "U1", Profile: &UserProfile{DisplayName: "rahul.support"}},
		"U2": {ID: "U2", Profile: &UserProfile{DisplayName: "priya.eng"}},
	}

	th, err := FetchThread(context.Background(), api, NewUserCache("okcredit", api), "C01", "p1785334150236249")
	if err != nil {
		t.Fatal(err)
	}
	if th.ChannelID != "C01" {
		t.Errorf("channel = %q", th.ChannelID)
	}
	if th.ThreadID != "p1785334150236249" {
		t.Errorf("thread_id = %q", th.ThreadID)
	}
	if th.ThreadTS != "1785334150.236249" {
		t.Errorf("thread_ts = %q", th.ThreadTS)
	}
	if len(th.Messages) != 2 {
		t.Fatalf("messages = %d", len(th.Messages))
	}
	if th.Messages[0].MessageID != "p1785334150236249" {
		t.Errorf("message_id = %q", th.Messages[0].MessageID)
	}
	if th.Messages[0].UserName != "rahul.support" || th.Messages[1].UserName != "priya.eng" {
		t.Errorf("user names = %q / %q", th.Messages[0].UserName, th.Messages[1].UserName)
	}
	if th.Users["U2"] != "priya.eng" {
		t.Errorf("users map = %+v", th.Users)
	}
}

// A permalink to a reply resolves to the whole thread, not just that message.
func TestFetchThread_ResolvesRootFromReply(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	root := "1785319311.419309"
	reply := "1785319451.036269"
	api := &threadAPI{
		byTS: map[string][]Message{
			reply: {
				{TS: reply, User: "U2", Text: "a reply", ThreadTS: root},
			},
			root: {
				{TS: root, User: "U1", Text: "the root"},
				{TS: reply, User: "U2", Text: "a reply", ThreadTS: root},
			},
		},
	}

	th, err := FetchThread(context.Background(), api, nil, "C094FL95GDV", "p1785319451036269")
	if err != nil {
		t.Fatal(err)
	}
	if th.ThreadTS != root {
		t.Errorf("thread_ts = %q, want root %q", th.ThreadTS, root)
	}
	if th.ThreadID != "p1785319311419309" {
		t.Errorf("thread_id = %q", th.ThreadID)
	}
	if len(th.Messages) != 2 {
		t.Fatalf("expected the full thread, got %d messages", len(th.Messages))
	}
	if len(api.requested) != 2 || api.requested[1] != root {
		t.Errorf("requested = %v", api.requested)
	}
}

func TestFetchThread_RootNeedsNoSecondCall(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	root := "1785319311.419309"
	api := &threadAPI{
		byTS: map[string][]Message{
			root: {
				{TS: root, User: "U1", Text: "the root", ThreadTS: root},
				{TS: "1785319451.036269", User: "U2", Text: "a reply", ThreadTS: root},
			},
		},
	}

	th, err := FetchThread(context.Background(), api, nil, "C01", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(th.Messages) != 2 {
		t.Fatalf("messages = %d", len(th.Messages))
	}
	if len(api.requested) != 1 {
		t.Errorf("expected 1 fetch, got %v", api.requested)
	}
}

func TestFetchThread_UsersCoversMentionsReactionsAndBots(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &threadAPI{
		byTS: map[string][]Message{
			"111.222222": {
				{
					TS:   "111.222222",
					User: "U1",
					Text: "<@U2> can you check <#C02XY|ledger>? cc <@U3|priya>",
					Reactions: []Reaction{
						{Name: "eyes", Users: []string{"U4"}, Count: 1},
					},
					Edited: &Edited{User: "U5", TS: "111.333333"},
				},
				{
					TS:          "111.444444",
					BotID:       "B1",
					Username:    "Sentry",
					ThreadTS:    "111.222222",
					Attachments: json.RawMessage(`[{"text":"paging <@U6>"}]`),
				},
			},
		},
	}
	api.users = map[string]*User{
		"U1": {ID: "U1", Name: "rahul"},
		"U2": {ID: "U2", Name: "amit"},
		"U3": {ID: "U3", Name: "priya"},
		"U4": {ID: "U4", Name: "sneha"},
		"U5": {ID: "U5", Name: "vikram"},
		"U6": {ID: "U6", Name: "oncall"},
	}

	th, err := FetchThread(context.Background(), api, NewUserCache("okcredit", api), "C01", "111.222222")
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"U1", "U2", "U3", "U4", "U5", "U6"} {
		if th.Users[id] == "" {
			t.Errorf("users map missing %s: %+v", id, th.Users)
		}
	}
	// Channel refs carry their name inline and are not looked up.
	if _, ok := th.Users["C02XY"]; ok {
		t.Error("channel refs should not be in the users map")
	}
	// The bot's inline username seeds the cache instead of costing a bots.info call.
	if th.Users["B1"] != "Sentry" {
		t.Errorf("B1 = %q", th.Users["B1"])
	}
	if api.botCalls != 0 {
		t.Errorf("bots.info calls = %d, want 0", api.botCalls)
	}
	if th.Messages[1].UserName != "Sentry" {
		t.Errorf("bot message user_name = %q", th.Messages[1].UserName)
	}
	// Text is left raw so the caller sees exactly what Slack stored.
	if th.Messages[0].Text != "<@U2> can you check <#C02XY|ledger>? cc <@U3|priya>" {
		t.Errorf("text was rewritten: %q", th.Messages[0].Text)
	}
}

func TestFetchThread_NilCacheSkipsNames(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &threadAPI{
		byTS: map[string][]Message{
			"111.222222": {{TS: "111.222222", User: "U1", Text: "hi"}},
		},
	}

	th, err := FetchThread(context.Background(), api, nil, "C01", "111.222222")
	if err != nil {
		t.Fatal(err)
	}
	if th.Users != nil {
		t.Errorf("users = %+v, want nil", th.Users)
	}
	if th.Messages[0].UserName != "" {
		t.Errorf("user_name = %q, want empty", th.Messages[0].UserName)
	}
	if th.Messages[0].MessageID != "p111222222" {
		t.Errorf("message_id = %q", th.Messages[0].MessageID)
	}
}

func TestFetchThread_InvalidTS(t *testing.T) {
	if _, err := FetchThread(context.Background(), &threadAPI{}, nil, "C01", "not-a-ts"); err == nil {
		t.Fatal("expected error for malformed timestamp")
	}
}

func TestFetchThread_APIError(t *testing.T) {
	api := &threadAPI{err: errors.New("channel_not_found")}
	if _, err := FetchThread(context.Background(), api, nil, "C01", "111.222222"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchThread_UnresolvedNamesAreNotFatal(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &threadAPI{
		byTS: map[string][]Message{
			"111.222222": {{TS: "111.222222", User: "U404", Text: "hi"}},
		},
	}

	th, err := FetchThread(context.Background(), api, NewUserCache("okcredit", api), "C01", "111.222222")
	if err != nil {
		t.Fatalf("name lookup failure must not fail the fetch: %v", err)
	}
	if th.Messages[0].UserName != "" {
		t.Errorf("user_name = %q, want empty", th.Messages[0].UserName)
	}
	if th.Messages[0].Text != "hi" {
		t.Errorf("text = %q", th.Messages[0].Text)
	}
}

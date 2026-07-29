package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("xoxp-test-token", "")
	c.baseURL = srv.URL
	return c
}

func testBrowserServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("xoxc-browser-token", "xoxd-cookie")
	c.baseURL = srv.URL
	return c
}

func TestClient_StandardAuth(t *testing.T) {
	var gotAuth string
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": "123"})
	})

	_, err := c.PostMessage(context.Background(), "C123", "hello", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer xoxp-test-token" {
		t.Errorf("auth header = %q", gotAuth)
	}
}

func TestClient_BrowserAuth(t *testing.T) {
	var gotCookie, gotAuth string
	var gotToken string
	c := testBrowserServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		r.ParseForm()
		gotToken = r.FormValue("token")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": "123"})
	})

	_, err := c.PostMessage(context.Background(), "C123", "hello", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Errorf("browser auth should not have Authorization header, got %q", gotAuth)
	}
	if gotCookie != "d=xoxd-cookie" {
		t.Errorf("cookie = %q", gotCookie)
	}
	if gotToken != "xoxc-browser-token" {
		t.Errorf("form token = %q", gotToken)
	}
}

func TestClient_ErrorParsing(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "channel_not_found"})
	})

	_, err := c.ConversationsHistory(context.Background(), "C999", HistoryOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "slack API conversations.history: channel_not_found" {
		t.Errorf("error = %q", got)
	}
}

func TestClient_RateLimitRetry(t *testing.T) {
	var calls atomic.Int32
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": []any{}})
	})

	msgs, err := c.ConversationsHistory(context.Background(), "C123", HistoryOptions{Limit: 5})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty messages, got %d", len(msgs))
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestClient_RateLimitExhausted(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(429)
	})

	_, err := c.ConversationsHistory(context.Background(), "C123", HistoryOptions{})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
}

func TestClient_SearchMessages(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": map[string]any{
				"total": 1,
				"page":  1,
				"pages": 1,
				"matches": []map[string]any{
					{"ts": "1234", "text": "found it", "user": "U123"},
				},
			},
		})
	})

	result, err := c.SearchMessages(context.Background(), "test query", SearchOptions{Count: 5})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d", result.Total)
	}
	if len(result.Messages) != 1 || result.Messages[0].Text != "found it" {
		t.Errorf("messages = %+v", result.Messages)
	}
}

func TestClient_AuthTest(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"team_id": "T123",
			"team":    "TestTeam",
			"user_id": "U456",
		})
	})

	teamID, teamName, userID, err := c.AuthTest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if teamID != "T123" || teamName != "TestTeam" || userID != "U456" {
		t.Errorf("got team=%q name=%q user=%q", teamID, teamName, userID)
	}
}

func TestClient_PostMessage(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("channel") != "C123" || r.FormValue("text") != "hello" {
			t.Errorf("wrong params: channel=%q text=%q", r.FormValue("channel"), r.FormValue("text"))
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": "999.999"})
	})

	ts, err := c.PostMessage(context.Background(), "C123", "hello", "")
	if err != nil {
		t.Fatal(err)
	}
	if ts != "999.999" {
		t.Errorf("ts = %q", ts)
	}
}

func TestClient_AddReaction(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("name") != "thumbsup" {
			t.Errorf("emoji = %q", r.FormValue("name"))
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	err := c.AddReaction(context.Background(), "C123", "1234.5678", "thumbsup")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_SearchFiles(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"files": map[string]any{
				"total": 1,
				"page":  1,
				"pages": 1,
				"matches": []map[string]any{
					{"id": "F1", "name": "report.pdf"},
				},
			},
		})
	})

	result, err := c.SearchFiles(context.Background(), "report", SearchOptions{Count: 5, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d", result.Total)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "report.pdf" {
		t.Errorf("files = %+v", result.Files)
	}
}

func TestClient_ConversationsReplies(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("channel") != "C123" {
			t.Errorf("channel = %q", r.FormValue("channel"))
		}
		if r.FormValue("ts") != "111.222" {
			t.Errorf("ts = %q", r.FormValue("ts"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"ts": "111.222", "text": "parent"},
				{"ts": "333.444", "text": "reply", "thread_ts": "111.222"},
			},
		})
	})

	msgs, err := c.ConversationsReplies(context.Background(), "C123", "111.222", HistoryOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].Text != "reply" {
		t.Errorf("reply text = %q", msgs[1].Text)
	}
}

func TestClient_ConversationsRepliesAll_Paginates(t *testing.T) {
	var cursors []string
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cursor := r.FormValue("cursor")
		cursors = append(cursors, cursor)

		switch cursor {
		case "":
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"ts": "111.222", "text": "parent"},
					{"ts": "333.444", "text": "reply 1", "thread_ts": "111.222"},
				},
				"has_more":          true,
				"response_metadata": map[string]any{"next_cursor": "page2"},
			})
		case "page2":
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{"ts": "111.222", "text": "parent"},
					{"ts": "555.666", "text": "reply 2", "thread_ts": "111.222"},
				},
				"response_metadata": map[string]any{"next_cursor": ""},
			})
		default:
			t.Errorf("unexpected cursor %q", cursor)
		}
	})

	msgs, err := c.ConversationsRepliesAll(context.Background(), "C123", "111.222", HistoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Text != "parent" || msgs[1].Text != "reply 1" || msgs[2].Text != "reply 2" {
		t.Errorf("messages = %+v", msgs)
	}
	if len(cursors) != 2 || cursors[1] != "page2" {
		t.Errorf("cursors = %v", cursors)
	}
}

func TestClient_ConversationsRepliesAll_SinglePage(t *testing.T) {
	var calls atomic.Int32
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"ts": "111.222", "text": "parent"},
			},
		})
	})

	msgs, err := c.ConversationsRepliesAll(context.Background(), "C123", "111.222", HistoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", calls.Load())
	}
}

func TestClient_ConversationsRepliesAll_PreservesAttachments(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"messages":[
			{"ts":"111.222","text":"","bot_id":"B01","username":"Sentry",
			 "attachments":[{"title":"PaymentFailed","text":"500 from upstream"}]}
		]}`))
	})

	msgs, err := c.ConversationsRepliesAll(context.Background(), "C123", "111.222", HistoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(string(msgs[0].Attachments), "PaymentFailed") {
		t.Errorf("attachments = %s", msgs[0].Attachments)
	}
}

func TestClient_ConversationsList(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "C1", "name": "general"},
				{"id": "C2", "name": "random"},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	})

	channels, err := c.ConversationsList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	if channels[0].Name != "general" {
		t.Errorf("first channel = %q", channels[0].Name)
	}
}

func TestClient_UsersList(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"members": []map[string]any{
				{"id": "U1", "name": "alice"},
				{"id": "U2", "name": "bob"},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	})

	users, err := c.UsersList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "alice" {
		t.Errorf("first user = %q", users[0].Name)
	}
}

func TestClient_UsersInfo(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("user") != "U123" {
			t.Errorf("user = %q", r.FormValue("user"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"user": map[string]any{"id": "U123", "name": "alice"},
		})
	})

	user, err := c.UsersInfo(context.Background(), "U123")
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "alice" {
		t.Errorf("name = %q", user.Name)
	}
}

func TestClient_UpdateMessage(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("channel") != "C123" {
			t.Errorf("channel = %q", r.FormValue("channel"))
		}
		if r.FormValue("ts") != "111.222" {
			t.Errorf("ts = %q", r.FormValue("ts"))
		}
		if r.FormValue("text") != "updated text" {
			t.Errorf("text = %q", r.FormValue("text"))
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	err := c.UpdateMessage(context.Background(), "C123", "111.222", "updated text")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_DeleteMessage(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("channel") != "C123" {
			t.Errorf("channel = %q", r.FormValue("channel"))
		}
		if r.FormValue("ts") != "111.222" {
			t.Errorf("ts = %q", r.FormValue("ts"))
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	err := c.DeleteMessage(context.Background(), "C123", "111.222")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_RemoveReaction(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("name") != "thumbsdown" {
			t.Errorf("emoji = %q", r.FormValue("name"))
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	err := c.RemoveReaction(context.Background(), "C123", "1234.5678", "thumbsdown")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_ResolveChannel(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "C123", "name": "general"},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	})

	id, err := c.ResolveChannel(context.Background(), "#general")
	if err != nil {
		t.Fatal(err)
	}
	if id != "C123" {
		t.Errorf("resolved = %q", id)
	}
}

func TestClient_ResolveUser(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"members": []map[string]any{
				{"id": "U123", "name": "alice"},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		})
	})

	id, err := c.ResolveUser(context.Background(), "@alice")
	if err != nil {
		t.Fatal(err)
	}
	if id != "U123" {
		t.Errorf("resolved = %q", id)
	}
}

func TestClient_HTTP500(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})

	_, err := c.ConversationsHistory(context.Background(), "C123", HistoryOptions{})
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

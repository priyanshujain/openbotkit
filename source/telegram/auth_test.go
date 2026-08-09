package telegram

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthPageMarkers(t *testing.T) {
	srv := httptest.NewServer(newAuthMux(newAuthState()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	page := string(raw)

	for _, marker := range []string{
		`id="qr"`,
		`id="linking"`,
		`id="password"`,
		`id="syncing"`,
		`id="done"`,
		`/api/qr`,
		`/api/password`,
		`qrcode.min.js`,
	} {
		if !strings.Contains(page, marker) {
			t.Fatalf("auth page is missing %q", marker)
		}
	}
}

func getState(t *testing.T, srv *httptest.Server) map[string]any {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/qr")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestAuthStateTransitions(t *testing.T) {
	st := newAuthState()
	srv := httptest.NewServer(newAuthMux(st))
	defer srv.Close()

	initial := getState(t, srv)
	for _, key := range []string{"qr", "linking", "password_needed", "syncing", "authenticated"} {
		if _, ok := initial[key]; !ok {
			t.Fatalf("state payload is missing %q", key)
		}
	}

	st.setQR("tg://login?token=abc")
	got := getState(t, srv)
	if got["qr"] != "tg://login?token=abc" {
		t.Fatalf("qr = %v", got["qr"])
	}

	// A refreshed token must replace the old one; the page re-renders on change.
	st.setQR("tg://login?token=def")
	if got := getState(t, srv); got["qr"] != "tg://login?token=def" {
		t.Fatalf("qr not refreshed: %v", got["qr"])
	}

	st.setLinking()
	got = getState(t, srv)
	if got["linking"] != true {
		t.Fatal("expected linking")
	}
	if got["qr"] != "" {
		t.Fatal("QR should be cleared once the token is accepted")
	}

	st.setPasswordNeeded("my hint")
	got = getState(t, srv)
	if got["password_needed"] != true {
		t.Fatal("expected password_needed")
	}
	if got["password_hint"] != "my hint" {
		t.Fatalf("password_hint = %v", got["password_hint"])
	}
	if got["linking"] != false {
		t.Fatal("linking should clear when the password is requested")
	}

	st.setError("Incorrect password, please try again.")
	if got := getState(t, srv); got["error"] != "Incorrect password, please try again." {
		t.Fatalf("error = %v", got["error"])
	}

	st.setSyncing()
	got = getState(t, srv)
	if got["syncing"] != true || got["password_needed"] != false {
		t.Fatalf("state = %v", got)
	}
	if got["error"] != "" {
		t.Fatal("error should clear once the password is accepted")
	}

	st.setAuthenticated()
	got = getState(t, srv)
	if got["authenticated"] != true || got["syncing"] != false {
		t.Fatalf("state = %v", got)
	}
}

func postPassword(t *testing.T, srv *httptest.Server, body string) int {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestAuthPasswordEndpoint(t *testing.T) {
	st := newAuthState()
	srv := httptest.NewServer(newAuthMux(st))
	defer srv.Close()

	if code := postPassword(t, srv, `{"password":"hunter2"}`); code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", code)
	}
	select {
	case pw := <-st.passwords:
		if pw != "hunter2" {
			t.Fatalf("password = %q", pw)
		}
	default:
		t.Fatal("password was not delivered to the auth goroutine")
	}
}

func TestAuthPasswordEndpointRejections(t *testing.T) {
	st := newAuthState()
	srv := httptest.NewServer(newAuthMux(st))
	defer srv.Close()

	if code := postPassword(t, srv, `{"password":""}`); code != http.StatusBadRequest {
		t.Fatalf("empty password status = %d, want 400", code)
	}
	if code := postPassword(t, srv, `not json`); code != http.StatusBadRequest {
		t.Fatalf("malformed body status = %d, want 400", code)
	}

	resp, err := http.Get(srv.URL + "/api/password")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", resp.StatusCode)
	}

	// While one password is queued, a second submission is rejected rather
	// than silently dropped.
	if code := postPassword(t, srv, `{"password":"first"}`); code != http.StatusAccepted {
		t.Fatalf("first status = %d", code)
	}
	if code := postPassword(t, srv, `{"password":"second"}`); code != http.StatusConflict {
		t.Fatalf("second status = %d, want 409", code)
	}
}

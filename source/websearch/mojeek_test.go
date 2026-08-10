package websearch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const mojeekFixtureHTML = `<!DOCTYPE html>
<html>
<body>
<ul class="results-standard">
  <li>
    <a class="ob" href="https://example.com/mojeek1">Mojeek Result One</a>
    <p class="s">Snippet for mojeek result one</p>
  </li>
  <li>
    <a class="ob" href="https://example.com/mojeek2">Mojeek Result Two</a>
    <p class="s">Snippet for mojeek result two</p>
  </li>
  <li>
    <a class="ob" href="https://example.com/mojeek3">Mojeek Result Three</a>
    <p class="s">Snippet for mojeek result three</p>
  </li>
</ul>
</body>
</html>`

func TestMojeekSearchNormal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mojeekFixtureHTML)
	}))
	defer srv.Close()

	m := &Mojeek{client: srv.Client(), baseURL: srv.URL}
	results, err := m.Search(context.Background(), "test query", SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Title != "Mojeek Result One" {
		t.Errorf("expected 'Mojeek Result One', got %q", results[0].Title)
	}
	if results[0].URL != "https://example.com/mojeek1" {
		t.Errorf("expected mojeek1 URL, got %q", results[0].URL)
	}
	if results[0].Snippet != "Snippet for mojeek result one" {
		t.Errorf("expected snippet, got %q", results[0].Snippet)
	}
	if results[0].Source != "mojeek" {
		t.Errorf("expected source 'mojeek', got %q", results[0].Source)
	}
}

func TestMojeekSearchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><ul class="results-standard"></ul></body></html>`)
	}))
	defer srv.Close()

	m := &Mojeek{client: srv.Client(), baseURL: srv.URL}
	results, err := m.Search(context.Background(), "asdfqwerzxcv", SearchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestMojeekSearchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	m := &Mojeek{client: srv.Client(), baseURL: srv.URL}
	_, err := m.Search(context.Background(), "test", SearchOptions{})
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestMojeekSearchTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, mojeekFixtureHTML)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	m := &Mojeek{client: srv.Client(), baseURL: srv.URL}
	_, err := m.Search(ctx, "test", SearchOptions{})
	if err == nil {
		t.Fatal("expected deadline exceeded error")
	}
}

func TestMojeekRegionCookies(t *testing.T) {
	var gotCookies []*http.Cookie
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookies = r.Cookies()
		fmt.Fprint(w, `<html><body><ul class="results-standard"></ul></body></html>`)
	}))
	defer srv.Close()

	m := &Mojeek{client: srv.Client(), baseURL: srv.URL}
	_, _ = m.Search(context.Background(), "test", SearchOptions{Region: "us-en"})

	arcFound, lbFound := false, false
	for _, c := range gotCookies {
		if c.Name == "arc" && c.Value == "us" {
			arcFound = true
		}
		if c.Name == "lb" && c.Value == "en" {
			lbFound = true
		}
	}
	if !arcFound {
		t.Error("expected arc=us cookie")
	}
	if !lbFound {
		t.Error("expected lb=en cookie")
	}
}

func TestRegionToCookies(t *testing.T) {
	tests := []struct {
		region  string
		wantArc string
		wantLb  string
	}{
		{"us-en", "us", "en"},
		{"de-de", "de", "de"},
		{"gb", "gb", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		arc, lb := regionToCookies(tt.region)
		if arc != tt.wantArc || lb != tt.wantLb {
			t.Errorf("regionToCookies(%q) = (%q, %q), want (%q, %q)",
				tt.region, arc, lb, tt.wantArc, tt.wantLb)
		}
	}
}

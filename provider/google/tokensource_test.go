package google

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// fakeTokenSource returns a new token on every call.
type fakeTokenSource struct {
	tok *oauth2.Token
}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	return f.tok, nil
}

func TestDBTokenSourceReturnsValidToken(t *testing.T) {
	ts := testTokenStore(t)

	initial := &oauth2.Token{
		AccessToken:  "valid-access",
		RefreshToken: "r1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	ts.SaveToken("user@gmail.com", initial, []string{"scope1"})

	src := newDBTokenSource("user@gmail.com", ts, nil, initial)

	got, err := src.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if got.AccessToken != "valid-access" {
		t.Errorf("expected valid-access, got %q", got.AccessToken)
	}
}

func TestDBTokenSourceRefreshesExpiredToken(t *testing.T) {
	ts := testTokenStore(t)

	expired := &oauth2.Token{
		AccessToken:  "expired",
		RefreshToken: "r1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour), // expired
	}
	ts.SaveToken("user@gmail.com", expired, []string{"scope1"})

	refreshed := &oauth2.Token{
		AccessToken:  "refreshed-access",
		RefreshToken: "r1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	fake := &fakeTokenSource{tok: refreshed}
	src := newDBTokenSource("user@gmail.com", ts, fake, expired)

	got, err := src.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if got.AccessToken != "refreshed-access" {
		t.Errorf("expected refreshed-access, got %q", got.AccessToken)
	}

	// Verify it was persisted.
	loaded, _, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.AccessToken != "refreshed-access" {
		t.Errorf("persisted access token: got %q, want %q", loaded.AccessToken, "refreshed-access")
	}
}

func TestDBTokenSourcePersistsRotatedRefreshToken(t *testing.T) {
	ts := testTokenStore(t)

	initial := &oauth2.Token{
		AccessToken:  "a1",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	ts.SaveToken("user@gmail.com", initial, []string{"scope1"})

	rotated := &oauth2.Token{
		AccessToken:  "a2",
		RefreshToken: "new-refresh", // different from initial
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	fake := &fakeTokenSource{tok: rotated}
	src := newDBTokenSource("user@gmail.com", ts, fake, initial)

	_, err := src.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	loaded, _, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.RefreshToken != "new-refresh" {
		t.Errorf("refresh token: got %q, want %q", loaded.RefreshToken, "new-refresh")
	}
}

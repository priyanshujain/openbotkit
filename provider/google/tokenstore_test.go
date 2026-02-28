package google

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func testTokenStore(t *testing.T) *TokenStore {
	t.Helper()
	ts, err := NewTokenStore(":memory:")
	if err != nil {
		t.Fatalf("open token store: %v", err)
	}
	t.Cleanup(func() { ts.Close() })
	return ts
}

func TestTokenStoreSaveAndLoad(t *testing.T) {
	ts := testTokenStore(t)

	tok := &oauth2.Token{
		AccessToken:  "access-123",
		TokenType:    "Bearer",
		RefreshToken: "refresh-456",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
	scopes := []string{"https://www.googleapis.com/auth/gmail.readonly", "openid"}

	if err := ts.SaveToken("user@gmail.com", tok, scopes); err != nil {
		t.Fatalf("save token: %v", err)
	}

	loaded, loadedScopes, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load token: %v", err)
	}

	if loaded.AccessToken != tok.AccessToken {
		t.Errorf("access token: got %q, want %q", loaded.AccessToken, tok.AccessToken)
	}
	if loaded.RefreshToken != tok.RefreshToken {
		t.Errorf("refresh token: got %q, want %q", loaded.RefreshToken, tok.RefreshToken)
	}
	if loaded.TokenType != tok.TokenType {
		t.Errorf("token type: got %q, want %q", loaded.TokenType, tok.TokenType)
	}
	if len(loadedScopes) != 2 {
		t.Fatalf("scopes: got %d, want 2", len(loadedScopes))
	}
	if loadedScopes[0] != scopes[0] || loadedScopes[1] != scopes[1] {
		t.Errorf("scopes: got %v, want %v", loadedScopes, scopes)
	}
}

func TestTokenStoreLoadMissing(t *testing.T) {
	ts := testTokenStore(t)

	_, _, err := ts.LoadToken("nobody@gmail.com")
	if err == nil {
		t.Fatal("expected error loading nonexistent token")
	}
}

func TestTokenStoreListAccounts(t *testing.T) {
	ts := testTokenStore(t)

	accounts, err := ts.ListAccounts()
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(accounts))
	}

	tok := &oauth2.Token{RefreshToken: "r1", TokenType: "Bearer"}
	ts.SaveToken("b@gmail.com", tok, []string{"scope1"})
	ts.SaveToken("a@gmail.com", tok, []string{"scope2"})

	accounts, err = ts.ListAccounts()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	// Ordered alphabetically.
	if accounts[0] != "a@gmail.com" || accounts[1] != "b@gmail.com" {
		t.Errorf("accounts: got %v", accounts)
	}
}

func TestTokenStoreDeleteToken(t *testing.T) {
	ts := testTokenStore(t)

	tok := &oauth2.Token{RefreshToken: "r1", TokenType: "Bearer"}
	ts.SaveToken("user@gmail.com", tok, []string{"scope1"})

	if err := ts.DeleteToken("user@gmail.com"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	accounts, _ := ts.ListAccounts()
	if len(accounts) != 0 {
		t.Fatalf("expected 0 accounts after delete, got %d", len(accounts))
	}
}

func TestTokenStoreHasScopes(t *testing.T) {
	ts := testTokenStore(t)

	tok := &oauth2.Token{RefreshToken: "r1", TokenType: "Bearer"}
	ts.SaveToken("user@gmail.com", tok, []string{
		"https://www.googleapis.com/auth/gmail.readonly",
		"https://www.googleapis.com/auth/calendar.readonly",
	})

	tests := []struct {
		name     string
		required []string
		want     bool
	}{
		{"has single", []string{"https://www.googleapis.com/auth/gmail.readonly"}, true},
		{"has both", []string{"https://www.googleapis.com/auth/gmail.readonly", "https://www.googleapis.com/auth/calendar.readonly"}, true},
		{"missing one", []string{"https://www.googleapis.com/auth/drive.readonly"}, false},
		{"empty required", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ts.HasScopes("user@gmail.com", tt.required)
			if err != nil {
				t.Fatalf("has scopes: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenStoreHasScopesUnknownAccount(t *testing.T) {
	ts := testTokenStore(t)

	has, err := ts.HasScopes("nobody@gmail.com", []string{"scope"})
	if err != nil {
		t.Fatalf("has scopes: %v", err)
	}
	if has {
		t.Error("expected false for unknown account")
	}
}

func TestTokenStoreUpdateScopes(t *testing.T) {
	ts := testTokenStore(t)

	tok := &oauth2.Token{RefreshToken: "r1", TokenType: "Bearer"}
	ts.SaveToken("user@gmail.com", tok, []string{"scope1"})

	newScopes := []string{"scope1", "scope2", "scope3"}
	if err := ts.UpdateScopes("user@gmail.com", newScopes); err != nil {
		t.Fatalf("update scopes: %v", err)
	}

	_, loaded, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 scopes, got %d: %v", len(loaded), loaded)
	}
}

func TestTokenStoreSaveAccessToken(t *testing.T) {
	ts := testTokenStore(t)

	tok := &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "r1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
	ts.SaveToken("user@gmail.com", tok, []string{"scope1"})

	newTok := &oauth2.Token{
		AccessToken: "new-access",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second),
	}
	if err := ts.SaveAccessToken("user@gmail.com", newTok); err != nil {
		t.Fatalf("save access token: %v", err)
	}

	loaded, _, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.AccessToken != "new-access" {
		t.Errorf("access token: got %q, want %q", loaded.AccessToken, "new-access")
	}
	// Refresh token should be unchanged.
	if loaded.RefreshToken != "r1" {
		t.Errorf("refresh token should be unchanged, got %q", loaded.RefreshToken)
	}
}

func TestTokenStoreSaveRefreshToken(t *testing.T) {
	ts := testTokenStore(t)

	tok := &oauth2.Token{RefreshToken: "old-refresh", TokenType: "Bearer"}
	ts.SaveToken("user@gmail.com", tok, []string{"scope1"})

	if err := ts.SaveRefreshToken("user@gmail.com", "new-refresh"); err != nil {
		t.Fatalf("save refresh token: %v", err)
	}

	loaded, _, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.RefreshToken != "new-refresh" {
		t.Errorf("refresh token: got %q, want %q", loaded.RefreshToken, "new-refresh")
	}
}

func TestTokenStoreTokenExpiry(t *testing.T) {
	ts := testTokenStore(t)

	// No token — should return nil.
	exp, err := ts.TokenExpiry("nobody@gmail.com")
	if err != nil {
		t.Fatalf("expiry missing: %v", err)
	}
	if exp != nil {
		t.Error("expected nil expiry for missing account")
	}

	// With token.
	expTime := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	tok := &oauth2.Token{
		RefreshToken: "r1",
		AccessToken:  "a1",
		TokenType:    "Bearer",
		Expiry:       expTime,
	}
	ts.SaveToken("user@gmail.com", tok, []string{"scope1"})

	exp, err = ts.TokenExpiry("user@gmail.com")
	if err != nil {
		t.Fatalf("expiry: %v", err)
	}
	if exp == nil {
		t.Fatal("expected non-nil expiry")
	}
}

func TestTokenStoreUpsert(t *testing.T) {
	ts := testTokenStore(t)

	tok1 := &oauth2.Token{RefreshToken: "r1", AccessToken: "a1", TokenType: "Bearer"}
	ts.SaveToken("user@gmail.com", tok1, []string{"scope1"})

	tok2 := &oauth2.Token{RefreshToken: "r2", AccessToken: "a2", TokenType: "Bearer"}
	ts.SaveToken("user@gmail.com", tok2, []string{"scope1", "scope2"})

	loaded, scopes, err := ts.LoadToken("user@gmail.com")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.RefreshToken != "r2" {
		t.Errorf("expected upserted refresh token r2, got %q", loaded.RefreshToken)
	}
	if len(scopes) != 2 {
		t.Errorf("expected 2 scopes after upsert, got %d", len(scopes))
	}

	accounts, _ := ts.ListAccounts()
	if len(accounts) != 1 {
		t.Errorf("expected 1 account (no duplicates), got %d", len(accounts))
	}
}

package slack

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type userCacheAPI struct {
	mockAPI
	users     map[string]*User
	bots      map[string]*Bot
	userCalls int
	botCalls  int
}

func (a *userCacheAPI) UsersInfo(_ context.Context, id string) (*User, error) {
	a.userCalls++
	if u, ok := a.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("user_not_found")
}

func (a *userCacheAPI) BotsInfo(_ context.Context, id string) (*Bot, error) {
	a.botCalls++
	if b, ok := a.bots[id]; ok {
		return b, nil
	}
	return nil, errors.New("bot_not_found")
}

func TestUserCache_ColdMissFetches(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{users: map[string]*User{
		"U1": {ID: "U1", Name: "rahul", Profile: &UserProfile{DisplayName: "rahul.support"}},
		"U2": {ID: "U2", Name: "priya"},
	}}

	c := NewUserCache("okcredit", api)
	names, err := c.Names(context.Background(), []string{"U1", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if names["U1"] != "rahul.support" {
		t.Errorf("U1 = %q", names["U1"])
	}
	if names["U2"] != "priya" {
		t.Errorf("U2 = %q", names["U2"])
	}
	if api.userCalls != 2 {
		t.Errorf("expected 2 users.info calls, got %d", api.userCalls)
	}
}

func TestUserCache_WarmHitCostsNothing(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{users: map[string]*User{"U1": {ID: "U1", Name: "rahul"}}}

	c := NewUserCache("okcredit", api)
	if _, err := c.Names(context.Background(), []string{"U1"}); err != nil {
		t.Fatal(err)
	}
	if api.userCalls != 1 {
		t.Fatalf("first pass calls = %d", api.userCalls)
	}

	names, err := c.Names(context.Background(), []string{"U1"})
	if err != nil {
		t.Fatal(err)
	}
	if names["U1"] != "rahul" {
		t.Errorf("U1 = %q", names["U1"])
	}
	if api.userCalls != 1 {
		t.Errorf("second pass should make no calls, total = %d", api.userCalls)
	}
}

func TestUserCache_PersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)
	api := &userCacheAPI{users: map[string]*User{"U1": {ID: "U1", Name: "rahul"}}}

	first := NewUserCache("okcredit", api)
	if _, err := first.Names(context.Background(), []string{"U1"}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "slack", "users-okcredit.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("cache file mode = %v, want 0600", perm)
	}

	second := NewUserCache("okcredit", api)
	names, err := second.Names(context.Background(), []string{"U1"})
	if err != nil {
		t.Fatal(err)
	}
	if names["U1"] != "rahul" {
		t.Errorf("U1 = %q", names["U1"])
	}
	if api.userCalls != 1 {
		t.Errorf("disk cache should avoid a second call, total = %d", api.userCalls)
	}
}

func TestUserCache_Refresh(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{users: map[string]*User{"U1": {ID: "U1", Name: "rahul"}}}

	c := NewUserCache("okcredit", api)
	if _, err := c.Names(context.Background(), []string{"U1"}); err != nil {
		t.Fatal(err)
	}

	c.Refresh()
	if _, err := c.Names(context.Background(), []string{"U1"}); err != nil {
		t.Fatal(err)
	}
	if api.userCalls != 2 {
		t.Errorf("refresh should force a re-fetch, calls = %d", api.userCalls)
	}
}

func TestUserCache_DisplayNameFallbackChain(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{users: map[string]*User{
		"U1": {ID: "U1", Name: "handle", RealName: "Real Name", Profile: &UserProfile{DisplayName: "display", RealName: "Profile Real"}},
		"U2": {ID: "U2", Name: "handle", RealName: "Real Name", Profile: &UserProfile{RealName: "Profile Real"}},
		"U3": {ID: "U3", Name: "handle", RealName: "Real Name"},
		"U4": {ID: "U4", Name: "handle"},
		"U5": {ID: "U5"},
	}}

	c := NewUserCache("okcredit", api)
	names, err := c.Names(context.Background(), []string{"U1", "U2", "U3", "U4", "U5"})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"U1": "display",
		"U2": "Profile Real",
		"U3": "Real Name",
		"U4": "handle",
		"U5": "U5",
	}
	for id, expect := range want {
		if names[id] != expect {
			t.Errorf("%s = %q, want %q", id, names[id], expect)
		}
	}
}

func TestUserCache_BotIDsUseBotsInfo(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{bots: map[string]*Bot{"B1": {ID: "B1", Name: "Sentry"}}}

	c := NewUserCache("okcredit", api)
	names, err := c.Names(context.Background(), []string{"B1"})
	if err != nil {
		t.Fatal(err)
	}
	if names["B1"] != "Sentry" {
		t.Errorf("B1 = %q", names["B1"])
	}
	if api.botCalls != 1 || api.userCalls != 0 {
		t.Errorf("bot calls = %d, user calls = %d", api.botCalls, api.userCalls)
	}
}

func TestUserCache_SeedSkipsLookup(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{}

	c := NewUserCache("okcredit", api)
	c.Seed("B1", "Alertmanager")

	names, err := c.Names(context.Background(), []string{"B1"})
	if err != nil {
		t.Fatal(err)
	}
	if names["B1"] != "Alertmanager" {
		t.Errorf("B1 = %q", names["B1"])
	}
	if api.botCalls != 0 {
		t.Errorf("seeded name should not trigger bots.info, calls = %d", api.botCalls)
	}
}

func TestUserCache_UnresolvableIDsAreAbsent(t *testing.T) {
	t.Setenv("OBK_CONFIG_DIR", t.TempDir())
	api := &userCacheAPI{users: map[string]*User{"U1": {ID: "U1", Name: "rahul"}}}

	c := NewUserCache("okcredit", api)
	names, err := c.Names(context.Background(), []string{"U1", "U404"})
	if err != nil {
		t.Fatalf("lookup failures must not be fatal: %v", err)
	}
	if names["U1"] != "rahul" {
		t.Errorf("U1 = %q", names["U1"])
	}
	if _, ok := names["U404"]; ok {
		t.Errorf("unresolvable ID should be absent, got %q", names["U404"])
	}
}

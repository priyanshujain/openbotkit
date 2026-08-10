package telegram

import (
	"context"
	"testing"

	"github.com/gotd/td/telegram/updates"
)

func TestStateStorageRoundTrip(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)
	ctx := context.Background()

	if _, found, err := s.GetState(ctx, 1); err != nil || found {
		t.Fatalf("expected no state, found=%v err=%v", found, err)
	}

	want := updates.State{Pts: 10, Qts: 20, Date: 30, Seq: 40}
	if err := s.SetState(ctx, 1, want); err != nil {
		t.Fatalf("set state: %v", err)
	}

	got, found, err := s.GetState(ctx, 1)
	if err != nil || !found {
		t.Fatalf("get state: found=%v err=%v", found, err)
	}
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
}

// pts alone is not a state. A half-written one must not read back as valid, or
// gap recovery resumes from zeroed qts/date/seq without saying anything.
func TestStateStoragePartialStateIsNotFound(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)
	ctx := context.Background()

	if err := SetKV(db, stateKey(1, "pts"), "10"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, found, err := s.GetState(ctx, 1); err != nil || found {
		t.Fatalf("a state missing qts/date/seq must report not found, found=%v err=%v", found, err)
	}
}

// One statement, so a crash cannot land between the fields.
func TestStateStorageSetStateIsOneStatement(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)

	if err := s.SetState(context.Background(), 1, updates.State{Pts: 1, Qts: 2, Date: 3, Seq: 4}); err != nil {
		t.Fatalf("set state: %v", err)
	}

	stored, err := ListKVPrefix(db, "state:1:")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(stored) != 4 {
		t.Fatalf("stored %d keys, want 4: %v", len(stored), stored)
	}
}

func TestStateStorageFieldSetters(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)
	ctx := context.Background()

	if err := s.SetState(ctx, 1, updates.State{Pts: 1, Qts: 2, Date: 3, Seq: 4}); err != nil {
		t.Fatalf("set state: %v", err)
	}

	if err := s.SetPts(ctx, 1, 11); err != nil {
		t.Fatalf("set pts: %v", err)
	}
	if err := s.SetQts(ctx, 1, 22); err != nil {
		t.Fatalf("set qts: %v", err)
	}
	if err := s.SetDateSeq(ctx, 1, 33, 44); err != nil {
		t.Fatalf("set date/seq: %v", err)
	}

	got, _, err := s.GetState(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	want := updates.State{Pts: 11, Qts: 22, Date: 33, Seq: 44}
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
}

// The interface requires the per-field setters to fail when no state exists.
func TestStateStorageSetFieldWithoutState(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)
	ctx := context.Background()

	if err := s.SetPts(ctx, 1, 5); err == nil {
		t.Fatal("expected SetPts to fail when the user has no state")
	}
	if err := s.SetSeq(ctx, 1, 5); err == nil {
		t.Fatal("expected SetSeq to fail when the user has no state")
	}
}

func TestStateStorageChannelPts(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)
	ctx := context.Background()

	if _, found, err := s.GetChannelPts(ctx, 1, 500); err != nil || found {
		t.Fatalf("expected miss, found=%v err=%v", found, err)
	}

	if err := s.SetChannelPts(ctx, 1, 500, 77); err != nil {
		t.Fatalf("set channel pts: %v", err)
	}
	if err := s.SetChannelPts(ctx, 1, 600, 88); err != nil {
		t.Fatalf("set channel pts: %v", err)
	}
	// Another account's channels must not leak into this one's iteration.
	if err := s.SetChannelPts(ctx, 2, 700, 99); err != nil {
		t.Fatalf("set channel pts: %v", err)
	}

	pts, found, err := s.GetChannelPts(ctx, 1, 500)
	if err != nil || !found {
		t.Fatalf("get channel pts: found=%v err=%v", found, err)
	}
	if pts != 77 {
		t.Fatalf("pts = %d, want 77", pts)
	}

	seen := map[int64]int{}
	err = s.ForEachChannels(ctx, 1, func(ctx context.Context, channelID int64, pts int) error {
		seen[channelID] = pts
		return nil
	})
	if err != nil {
		t.Fatalf("for each: %v", err)
	}
	if len(seen) != 2 || seen[500] != 77 || seen[600] != 88 {
		t.Fatalf("channels = %v", seen)
	}
}

func TestStateStorageAccessHashers(t *testing.T) {
	db := testDB(t)
	s := NewStateStorage(db)
	ctx := context.Background()

	if _, found, err := s.GetChannelAccessHash(ctx, 1, 500); err != nil || found {
		t.Fatalf("expected miss, found=%v err=%v", found, err)
	}
	if err := s.SetChannelAccessHash(ctx, 1, 500, 4242); err != nil {
		t.Fatalf("set channel hash: %v", err)
	}
	hash, found, err := s.GetChannelAccessHash(ctx, 1, 500)
	if err != nil || !found {
		t.Fatalf("get channel hash: found=%v err=%v", found, err)
	}
	if hash != 4242 {
		t.Fatalf("hash = %d, want 4242", hash)
	}

	// Stored on the chat row, so the send path can reach it.
	chat, err := GetChat(db, ChatIDFromChannel(500))
	if err != nil || chat == nil {
		t.Fatalf("chat row missing: %v", err)
	}
	if chat.AccessHash != 4242 || !chat.IsChannel {
		t.Fatalf("chat = %+v", chat)
	}

	if err := s.SetUserAccessHash(ctx, 1, 900, 1234); err != nil {
		t.Fatalf("set user hash: %v", err)
	}
	hash, found, err = s.GetUserAccessHash(ctx, 1, 900)
	if err != nil || !found {
		t.Fatalf("get user hash: found=%v err=%v", found, err)
	}
	if hash != 1234 {
		t.Fatalf("hash = %d, want 1234", hash)
	}
}

func TestSelfRoundTrip(t *testing.T) {
	db := testDB(t)

	if id, _, err := LoadSelf(db); err != nil || id != 0 {
		t.Fatalf("expected no self, id=%d err=%v", id, err)
	}

	if err := SaveSelf(db, 12345, "ann"); err != nil {
		t.Fatalf("save self: %v", err)
	}
	id, username, err := LoadSelf(db)
	if err != nil {
		t.Fatalf("load self: %v", err)
	}
	if id != 12345 || username != "ann" {
		t.Fatalf("self = (%d, %q)", id, username)
	}
}

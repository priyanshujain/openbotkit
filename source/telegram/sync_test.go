package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
)

// fakeFetcher drives sync without a network.
type fakeFetcher struct {
	dialogs []dialogs.Elem
	// history is keyed by the normalised chat ID.
	history map[int64][]messages.Elem

	dialogsErr error
	historyErr error

	// offsets records the offsetID each chat was asked for.
	offsets map[int64]int
}

func (f *fakeFetcher) Dialogs(ctx context.Context, fn func(context.Context, dialogs.Elem) error) error {
	if f.dialogsErr != nil {
		return f.dialogsErr
	}
	for _, d := range f.dialogs {
		if err := fn(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeFetcher) History(ctx context.Context, p tg.InputPeerClass, offsetID int, fn func(context.Context, messages.Elem) error) error {
	if f.historyErr != nil {
		return f.historyErr
	}
	chatID, ok := chatIDFromInputPeer(p)
	if !ok {
		return nil
	}
	if f.offsets == nil {
		f.offsets = map[int64]int{}
	}
	f.offsets[chatID] = offsetID

	for _, m := range f.history[chatID] {
		msg := m.Msg.(*tg.Message)
		// Real getHistory walks backwards from offsetID.
		if offsetID > 0 && msg.ID >= offsetID {
			continue
		}
		if err := fn(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func entities(users []*tg.User, chats []*tg.Chat, channels []*tg.Channel) peer.Entities {
	um := map[int64]*tg.User{}
	for _, u := range users {
		um[u.ID] = u
	}
	cm := map[int64]*tg.Chat{}
	for _, c := range chats {
		cm[c.ID] = c
	}
	chm := map[int64]*tg.Channel{}
	for _, c := range channels {
		chm[c.ID] = c
	}
	return peer.NewEntities(um, cm, chm)
}

func tgMessage(id int, at time.Time, text string, opts ...func(*tg.Message)) *tg.Message {
	m := &tg.Message{ID: id, Date: int(at.Unix()), Message: text}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// userDialog builds a one-to-one dialog with the given messages, newest first.
func userDialog(u *tg.User, msgs ...*tg.Message) (dialogs.Elem, []messages.Elem) {
	ents := entities([]*tg.User{u}, nil, nil)
	peerIn := &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}

	d := dialogs.Elem{
		Dialog:   &tg.Dialog{Peer: &tg.PeerUser{UserID: u.ID}},
		Peer:     peerIn,
		Entities: ents,
	}
	if len(msgs) > 0 {
		d.Last = msgs[0]
	}

	var hist []messages.Elem
	for _, m := range msgs {
		hist = append(hist, messages.Elem{Msg: m, Peer: peerIn, Entities: ents})
	}
	return d, hist
}

func TestBackfillStoresChatsUsersAndMessages(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, AccessHash: 1111, FirstName: "Ann", LastName: "Lee", Username: "ann"}
	d, hist := userDialog(ann,
		tgMessage(20, now, "newest"),
		tgMessage(19, now.Add(-time.Hour), "older"),
	)

	f := &fakeFetcher{
		dialogs: []dialogs.Elem{d},
		history: map[int64][]messages.Elem{ChatIDFromUser(11): hist},
	}

	res, err := Backfill(ctx, f, db, BackfillOptions{})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if res.Chats != 1 || res.Messages != 2 || res.Errors != 0 {
		t.Fatalf("result = %+v", res)
	}

	chat, err := GetChat(db, ChatIDFromUser(11))
	if err != nil || chat == nil {
		t.Fatalf("chat missing: %v", err)
	}
	if chat.Title != "Ann Lee" || chat.AccessHash != 1111 || chat.Type != PeerUser {
		t.Fatalf("chat = %+v", chat)
	}
	if chat.LastMessageAt == nil {
		t.Fatal("last_message_at not set from the dialog's last message")
	}

	user, err := GetUser(db, 11)
	if err != nil || user == nil {
		t.Fatalf("user missing: %v", err)
	}
	if user.AccessHash != 1111 {
		t.Fatalf("access hash = %d", user.AccessHash)
	}

	msgs, err := ListMessages(db, ListOptions{ChatID: ChatIDFromUser(11)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	// Incoming one-to-one messages carry no from_id; the peer is the sender.
	if msgs[0].SenderID != 11 {
		t.Fatalf("sender_id = %d, want 11", msgs[0].SenderID)
	}
	if msgs[0].SenderName != "Ann Lee" {
		t.Fatalf("sender_name = %q", msgs[0].SenderName)
	}
}

func TestBackfillStopsAtSince(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, AccessHash: 1111, FirstName: "Ann"}
	d, hist := userDialog(ann,
		tgMessage(30, now, "today"),
		tgMessage(29, now.AddDate(0, 0, -5), "recent"),
		tgMessage(28, now.AddDate(0, 0, -40), "old"),
		tgMessage(27, now.AddDate(0, 0, -60), "ancient"),
	)

	f := &fakeFetcher{
		dialogs: []dialogs.Elem{d},
		history: map[int64][]messages.Elem{ChatIDFromUser(11): hist},
	}

	res, err := Backfill(ctx, f, db, BackfillOptions{Since: now.AddDate(0, 0, -30)})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if res.Messages != 2 {
		t.Fatalf("expected to stop after 2 messages, got %d", res.Messages)
	}
	if res.Errors != 0 {
		t.Fatalf("stopping early must not count as an error, got %d", res.Errors)
	}

	count, _ := CountMessages(db, ChatIDFromUser(11))
	if count != 2 {
		t.Fatalf("stored %d messages, want 2", count)
	}
}

func TestBackfillPerChatLimit(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, FirstName: "Ann"}
	var msgs []*tg.Message
	for i := 10; i > 0; i-- {
		msgs = append(msgs, tgMessage(i, now.Add(-time.Duration(10-i)*time.Minute), "m"))
	}
	d, hist := userDialog(ann, msgs...)

	f := &fakeFetcher{
		dialogs: []dialogs.Elem{d},
		history: map[int64][]messages.Elem{ChatIDFromUser(11): hist},
	}

	res, err := Backfill(ctx, f, db, BackfillOptions{PerChatLimit: 3})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if res.Messages != 3 {
		t.Fatalf("expected the limit to cap at 3, got %d", res.Messages)
	}
}

// An interrupted backfill must resume from the oldest message it stored,
// not start over from the newest.
func TestBackfillResumesFromStoredProgress(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, FirstName: "Ann"}
	var msgs []*tg.Message
	for i := 10; i > 0; i-- {
		msgs = append(msgs, tgMessage(i, now.Add(-time.Duration(10-i)*time.Minute), "m"))
	}
	d, hist := userDialog(ann, msgs...)
	chatID := ChatIDFromUser(11)

	f := &fakeFetcher{
		dialogs: []dialogs.Elem{d},
		history: map[int64][]messages.Elem{chatID: hist},
	}

	if _, err := Backfill(ctx, f, db, BackfillOptions{PerChatLimit: 4}); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if f.offsets[chatID] != 0 {
		t.Fatalf("first pass should start from the newest message, got offset %d", f.offsets[chatID])
	}

	state, err := GetSyncState(db, chatID)
	if err != nil || state == nil {
		t.Fatalf("sync state missing: %v", err)
	}
	if state.MaxID != 10 || state.MinID != 7 {
		t.Fatalf("state = %+v, want min 7 max 10", state)
	}

	res, err := Backfill(ctx, f, db, BackfillOptions{})
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if f.offsets[chatID] != 7 {
		t.Fatalf("second pass should resume at offset 7, got %d", f.offsets[chatID])
	}
	if res.Messages != 6 {
		t.Fatalf("expected the remaining 6 messages, got %d", res.Messages)
	}

	total, _ := CountMessages(db, chatID)
	if total != 10 {
		t.Fatalf("stored %d messages after resume, want 10", total)
	}
}

func TestBackfillFullIgnoresProgress(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, FirstName: "Ann"}
	d, hist := userDialog(ann,
		tgMessage(3, now, "c"),
		tgMessage(2, now.Add(-time.Minute), "b"),
		tgMessage(1, now.Add(-2*time.Minute), "a"),
	)
	chatID := ChatIDFromUser(11)
	f := &fakeFetcher{
		dialogs: []dialogs.Elem{d},
		history: map[int64][]messages.Elem{chatID: hist},
	}

	if _, err := Backfill(ctx, f, db, BackfillOptions{PerChatLimit: 1}); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if _, err := Backfill(ctx, f, db, BackfillOptions{Full: true}); err != nil {
		t.Fatalf("full pass: %v", err)
	}
	if f.offsets[chatID] != 0 {
		t.Fatalf("full backfill should ignore stored progress, got offset %d", f.offsets[chatID])
	}
}

// One failing chat must not abort the whole run.
func TestBackfillCountsChatErrors(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, FirstName: "Ann"}
	d, _ := userDialog(ann, tgMessage(1, now, "hi"))

	f := &fakeFetcher{
		dialogs:    []dialogs.Elem{d},
		historyErr: errors.New("boom"),
	}

	res, err := Backfill(ctx, f, db, BackfillOptions{})
	if err != nil {
		t.Fatalf("backfill should survive a per-chat failure: %v", err)
	}
	if res.Chats != 1 {
		t.Fatalf("chats = %d, want 1", res.Chats)
	}
	if res.Errors != 1 {
		t.Fatalf("errors = %d, want 1", res.Errors)
	}
}

func TestBackfillDialogsError(t *testing.T) {
	db := testDB(t)
	f := &fakeFetcher{dialogsErr: errors.New("no dialogs")}

	if _, err := Backfill(context.Background(), f, db, BackfillOptions{}); err == nil {
		t.Fatal("expected the dialog error to propagate")
	}
}

func TestBackfillGroupsAndChannels(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	bob := &tg.User{ID: 21, AccessHash: 2121, FirstName: "Bob"}
	group := &tg.Chat{ID: 31, Title: "Family"}
	channel := &tg.Channel{ID: 41, AccessHash: 4141, Title: "Eng", Username: "eng", Megagroup: true}

	groupEnts := entities([]*tg.User{bob}, []*tg.Chat{group}, nil)
	groupPeer := &tg.InputPeerChat{ChatID: group.ID}
	groupMsg := tgMessage(5, now, "hey all", func(m *tg.Message) {
		m.SetFromID(&tg.PeerUser{UserID: bob.ID})
	})

	channelEnts := entities(nil, nil, []*tg.Channel{channel})
	channelPeer := &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash}
	channelMsg := tgMessage(9, now, "shipped")

	f := &fakeFetcher{
		dialogs: []dialogs.Elem{
			{Dialog: &tg.Dialog{Peer: &tg.PeerChat{ChatID: group.ID}}, Peer: groupPeer, Entities: groupEnts, Last: groupMsg},
			{Dialog: &tg.Dialog{Peer: &tg.PeerChannel{ChannelID: channel.ID}}, Peer: channelPeer, Entities: channelEnts, Last: channelMsg},
		},
		history: map[int64][]messages.Elem{
			ChatIDFromGroup(group.ID):     {{Msg: groupMsg, Peer: groupPeer, Entities: groupEnts}},
			ChatIDFromChannel(channel.ID): {{Msg: channelMsg, Peer: channelPeer, Entities: channelEnts}},
		},
	}

	res, err := Backfill(ctx, f, db, BackfillOptions{})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if res.Chats != 2 || res.Messages != 2 {
		t.Fatalf("result = %+v", res)
	}

	g, err := GetChat(db, ChatIDFromGroup(group.ID))
	if err != nil || g == nil {
		t.Fatalf("group chat missing: %v", err)
	}
	if g.Title != "Family" || !g.IsGroup || g.IsChannel {
		t.Fatalf("group chat = %+v", g)
	}

	c, err := GetChat(db, ChatIDFromChannel(channel.ID))
	if err != nil || c == nil {
		t.Fatalf("channel chat missing: %v", err)
	}
	if c.AccessHash != 4141 || !c.IsChannel {
		t.Fatalf("channel chat = %+v", c)
	}
	// A megagroup is a channel that is also a group.
	if !c.IsGroup {
		t.Fatalf("megagroup should be flagged as a group: %+v", c)
	}

	msgs, err := ListMessages(db, ListOptions{ChatID: ChatIDFromGroup(group.ID)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 || msgs[0].SenderID != bob.ID {
		t.Fatalf("group message = %+v", msgs)
	}
	if msgs[0].SenderName != "Bob" {
		t.Fatalf("sender_name = %q", msgs[0].SenderName)
	}
}

func TestSyncDialogsSkipsHistory(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	ann := &tg.User{ID: 11, AccessHash: 1111, FirstName: "Ann"}
	d, hist := userDialog(ann, tgMessage(1, now, "hi"))

	f := &fakeFetcher{
		dialogs: []dialogs.Elem{d},
		history: map[int64][]messages.Elem{ChatIDFromUser(11): hist},
	}

	count, err := SyncDialogs(ctx, f, db)
	if err != nil {
		t.Fatalf("sync dialogs: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	if chat, _ := GetChat(db, ChatIDFromUser(11)); chat == nil {
		t.Fatal("chat not stored")
	}
	if n, _ := CountMessages(db, 0); n != 0 {
		t.Fatalf("SyncDialogs must not fetch history, stored %d messages", n)
	}
}

func TestMessageFromTGSkipsServiceMessages(t *testing.T) {
	if _, ok := messageFromTG(&tg.MessageService{ID: 1}, 100); ok {
		t.Fatal("service messages should not be stored")
	}
}

func TestMessageFromTGFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	edited := now.Add(time.Hour)

	m := &tg.Message{ID: 5, Date: int(now.Unix()), Message: "hi", Out: true}
	m.SetFromID(&tg.PeerUser{UserID: 77})
	m.SetReplyTo(&tg.MessageReplyHeader{ReplyToMsgID: 4})
	m.SetEditDate(int(edited.Unix()))
	m.SetMedia(&tg.MessageMediaPhoto{})

	got, ok := messageFromTG(m, ChatIDFromUser(11))
	if !ok {
		t.Fatal("expected a storable message")
	}
	if got.MessageID != 5 || got.SenderID != 77 || got.ReplyToID != 4 {
		t.Fatalf("message = %+v", got)
	}
	if !got.IsOutgoing {
		t.Fatal("out flag lost")
	}
	if got.MediaType != "photo" {
		t.Fatalf("media_type = %q", got.MediaType)
	}
	if got.EditDate == nil || !got.EditDate.Equal(edited) {
		t.Fatalf("edit_date = %v, want %v", got.EditDate, edited)
	}
}

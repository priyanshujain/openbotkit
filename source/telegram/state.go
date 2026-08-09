package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/td/telegram/updates"

	"github.com/73ai/openbotkit/store"
)

// StateStorage backs gotd's updates.Manager with our own SQL store, so pts/qts
// survive restarts without pulling in bbolt or pebble for contrib's ready-made
// implementations. It satisfies updates.StateStorage, updates.ChannelAccessHasher
// and updates.UserAccessHasher.
//
// Access hashes live in the telegram_chats/telegram_users access_hash columns
// rather than a private table, because the send path needs them too.
type StateStorage struct {
	db *store.DB
}

func NewStateStorage(db *store.DB) *StateStorage {
	return &StateStorage{db: db}
}

var (
	_ updates.StateStorage        = (*StateStorage)(nil)
	_ updates.ChannelAccessHasher = (*StateStorage)(nil)
	_ updates.UserAccessHasher    = (*StateStorage)(nil)
)

func stateKey(userID int64, field string) string {
	return fmt.Sprintf("state:%d:%s", userID, field)
}

func channelPtsPrefix(userID int64) string {
	return fmt.Sprintf("channel_pts:%d:", userID)
}

func channelPtsKey(userID, channelID int64) string {
	return channelPtsPrefix(userID) + strconv.FormatInt(channelID, 10)
}

func (s *StateStorage) getInt(key string) (int, bool, error) {
	raw, found, err := GetKV(s.db, key)
	if err != nil || !found {
		return 0, found, err
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("parse %q: %w", key, err)
	}
	return n, true, nil
}

func (s *StateStorage) setInt(key string, value int) error {
	return SetKV(s.db, key, strconv.Itoa(value))
}

func (s *StateStorage) GetState(ctx context.Context, userID int64) (updates.State, bool, error) {
	pts, found, err := s.getInt(stateKey(userID, "pts"))
	if err != nil || !found {
		return updates.State{}, found, err
	}
	qts, _, err := s.getInt(stateKey(userID, "qts"))
	if err != nil {
		return updates.State{}, false, err
	}
	date, _, err := s.getInt(stateKey(userID, "date"))
	if err != nil {
		return updates.State{}, false, err
	}
	seq, _, err := s.getInt(stateKey(userID, "seq"))
	if err != nil {
		return updates.State{}, false, err
	}
	return updates.State{Pts: pts, Qts: qts, Date: date, Seq: seq}, true, nil
}

func (s *StateStorage) SetState(ctx context.Context, userID int64, state updates.State) error {
	for field, value := range map[string]int{
		"pts": state.Pts, "qts": state.Qts, "date": state.Date, "seq": state.Seq,
	} {
		if err := s.setInt(stateKey(userID, field), value); err != nil {
			return err
		}
	}
	return nil
}

// setField refuses to write when no state exists yet, as the interface requires.
func (s *StateStorage) setField(userID int64, field string, value int) error {
	if _, found, err := s.getInt(stateKey(userID, "pts")); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("state for user %d does not exist", userID)
	}
	return s.setInt(stateKey(userID, field), value)
}

func (s *StateStorage) SetPts(ctx context.Context, userID int64, pts int) error {
	return s.setField(userID, "pts", pts)
}

func (s *StateStorage) SetQts(ctx context.Context, userID int64, qts int) error {
	return s.setField(userID, "qts", qts)
}

func (s *StateStorage) SetDate(ctx context.Context, userID int64, date int) error {
	return s.setField(userID, "date", date)
}

func (s *StateStorage) SetSeq(ctx context.Context, userID int64, seq int) error {
	return s.setField(userID, "seq", seq)
}

func (s *StateStorage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	if err := s.setField(userID, "date", date); err != nil {
		return err
	}
	return s.setField(userID, "seq", seq)
}

func (s *StateStorage) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	return s.getInt(channelPtsKey(userID, channelID))
}

func (s *StateStorage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	return s.setInt(channelPtsKey(userID, channelID), pts)
}

func (s *StateStorage) ForEachChannels(ctx context.Context, userID int64, f func(ctx context.Context, channelID int64, pts int) error) error {
	prefix := channelPtsPrefix(userID)
	rows, err := ListKVPrefix(s.db, prefix)
	if err != nil {
		return err
	}
	for key, raw := range rows {
		channelID, err := strconv.ParseInt(strings.TrimPrefix(key, prefix), 10, 64)
		if err != nil {
			continue
		}
		pts, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		if err := f(ctx, channelID, pts); err != nil {
			return err
		}
	}
	return nil
}

// SetChannelAccessHash stores the hash on the chat row so the send path can
// rebuild an InputPeerChannel without a second lookup. userID is ignored: the
// store holds exactly one account.
func (s *StateStorage) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	return UpsertChat(s.db, &Chat{
		ChatID:     ChatIDFromChannel(channelID),
		Type:       PeerChannel,
		AccessHash: accessHash,
		IsChannel:  true,
	})
}

func (s *StateStorage) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (int64, bool, error) {
	chat, err := GetChat(s.db, ChatIDFromChannel(channelID))
	if err != nil {
		return 0, false, err
	}
	if chat == nil || chat.AccessHash == 0 {
		return 0, false, nil
	}
	return chat.AccessHash, true, nil
}

func (s *StateStorage) SetUserAccessHash(ctx context.Context, userID, targetUserID, accessHash int64) error {
	return UpsertUser(s.db, &User{UserID: targetUserID, AccessHash: accessHash})
}

func (s *StateStorage) GetUserAccessHash(ctx context.Context, userID, targetUserID int64) (int64, bool, error) {
	user, err := GetUser(s.db, targetUserID)
	if err != nil {
		return 0, false, err
	}
	if user == nil || user.AccessHash == 0 {
		return 0, false, nil
	}
	return user.AccessHash, true, nil
}

// SaveSelf records the signed-in account so status can be reported without a
// network round trip.
func SaveSelf(db *store.DB, userID int64, username string) error {
	if err := SetKV(db, "self:id", strconv.FormatInt(userID, 10)); err != nil {
		return err
	}
	return SetKV(db, "self:username", username)
}

// LoadSelf returns the stored account ID and username, if any.
func LoadSelf(db *store.DB) (int64, string, error) {
	raw, found, err := GetKV(db, "self:id")
	if err != nil || !found {
		return 0, "", err
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse self:id: %w", err)
	}
	username, _, err := GetKV(db, "self:username")
	if err != nil {
		return 0, "", err
	}
	return id, username, nil
}

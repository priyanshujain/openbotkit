package telegram

import (
	"time"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"

	"github.com/73ai/openbotkit/store"
)

func userFromTG(u *tg.User) *User {
	return &User{
		UserID:     u.ID,
		Username:   u.Username,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Phone:      u.Phone,
		AccessHash: u.AccessHash,
		IsBot:      u.Bot,
	}
}

func chatFromTGUser(u *tg.User) *Chat {
	return &Chat{
		ChatID:     ChatIDFromUser(u.ID),
		Type:       PeerUser,
		Title:      userFromTG(u).DisplayName(),
		Username:   u.Username,
		AccessHash: u.AccessHash,
	}
}

func chatFromTGChat(c *tg.Chat) *Chat {
	return &Chat{
		ChatID:  ChatIDFromGroup(c.ID),
		Type:    PeerGroup,
		Title:   c.Title,
		IsGroup: true,
	}
}

func chatFromTGChannel(c *tg.Channel) *Chat {
	// Supergroups are channels that carry the megagroup flag.
	return &Chat{
		ChatID:     ChatIDFromChannel(c.ID),
		Type:       PeerChannel,
		Title:      c.Title,
		Username:   c.Username,
		AccessHash: c.AccessHash,
		IsGroup:    c.Megagroup,
		IsChannel:  true,
	}
}

// saveEntities records every user, chat and channel referenced by a response.
// Storing access hashes here is what makes sending possible later without a
// second lookup.
func saveEntities(db *store.DB, ents peer.Entities) error {
	for _, u := range ents.Users() {
		if u == nil {
			continue
		}
		if err := UpsertUser(db, userFromTG(u)); err != nil {
			return err
		}
		if err := UpsertChat(db, chatFromTGUser(u)); err != nil {
			return err
		}
	}
	for _, c := range ents.Chats() {
		if c == nil {
			continue
		}
		if err := UpsertChat(db, chatFromTGChat(c)); err != nil {
			return err
		}
	}
	for _, c := range ents.Channels() {
		if c == nil {
			continue
		}
		if err := UpsertChat(db, chatFromTGChannel(c)); err != nil {
			return err
		}
	}
	return nil
}

// chatFromDialog derives the chat row for a dialog. It reports false for
// dialog kinds that carry no peer.
func chatFromDialog(elem dialogs.Elem) (*Chat, bool) {
	chatID, ok := chatIDFromInputPeer(elem.Peer)
	if !ok {
		return nil, false
	}
	chat := chatFromEntities(chatID, elem.Entities)
	if elem.Last != nil {
		ts := time.Unix(int64(elem.Last.GetDate()), 0).UTC()
		chat.LastMessageAt = &ts
	}
	return chat, true
}

// chatFromEntities builds the richest chat row the entity set allows, falling
// back to a bare row when the peer is not present.
func chatFromEntities(chatID int64, ents peer.Entities) *Chat {
	kind, rawID := SplitChatID(chatID)
	switch kind {
	case PeerUser:
		if u, ok := ents.User(rawID); ok && u != nil {
			return chatFromTGUser(u)
		}
	case PeerGroup:
		if c, ok := ents.Chat(rawID); ok && c != nil {
			return chatFromTGChat(c)
		}
	case PeerChannel:
		if c, ok := ents.Channel(rawID); ok && c != nil {
			return chatFromTGChannel(c)
		}
	}
	return &Chat{
		ChatID:    chatID,
		Type:      kind,
		IsGroup:   kind == PeerGroup,
		IsChannel: kind == PeerChannel,
	}
}

func chatIDFromInputPeer(p tg.InputPeerClass) (int64, bool) {
	switch v := p.(type) {
	case *tg.InputPeerUser:
		return ChatIDFromUser(v.UserID), true
	case *tg.InputPeerChat:
		return ChatIDFromGroup(v.ChatID), true
	case *tg.InputPeerChannel:
		return ChatIDFromChannel(v.ChannelID), true
	default:
		return 0, false
	}
}

// ChatIDFromPeer normalises a tg.PeerClass, as carried by updates.
func ChatIDFromPeer(p tg.PeerClass) (int64, bool) {
	switch v := p.(type) {
	case *tg.PeerUser:
		return ChatIDFromUser(v.UserID), true
	case *tg.PeerChat:
		return ChatIDFromGroup(v.ChatID), true
	case *tg.PeerChannel:
		return ChatIDFromChannel(v.ChannelID), true
	default:
		return 0, false
	}
}

// messageFromTG converts a message into a storable row. Service messages and
// empty messages report false.
func messageFromTG(m tg.NotEmptyMessage, chatID int64) (*Message, bool) {
	msg, ok := m.(*tg.Message)
	if !ok {
		return nil, false
	}

	out := &Message{
		MessageID:  msg.ID,
		ChatID:     chatID,
		Text:       msg.Message,
		Timestamp:  time.Unix(int64(msg.Date), 0).UTC(),
		IsOutgoing: msg.Out,
		MediaType:  mediaTypeOf(msg),
	}

	if from, ok := msg.GetFromID(); ok {
		if u, ok := from.(*tg.PeerUser); ok {
			out.SenderID = u.UserID
		}
	} else if !msg.Out {
		// One-to-one incoming messages omit from_id; the peer is the sender.
		if kind, rawID := SplitChatID(chatID); kind == PeerUser {
			out.SenderID = rawID
		}
	}

	if reply, ok := msg.GetReplyTo(); ok {
		if header, ok := reply.(*tg.MessageReplyHeader); ok {
			out.ReplyToID = header.ReplyToMsgID
		}
	}

	if editDate, ok := msg.GetEditDate(); ok && editDate > 0 {
		ts := time.Unix(int64(editDate), 0).UTC()
		out.EditDate = &ts
	}

	return out, true
}

func mediaTypeOf(msg *tg.Message) string {
	media, ok := msg.GetMedia()
	if !ok {
		return ""
	}
	switch media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		return "document"
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
		return "location"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaPoll:
		return "poll"
	case *tg.MessageMediaWebPage:
		return "webpage"
	case *tg.MessageMediaVenue:
		return "venue"
	case *tg.MessageMediaDice:
		return "dice"
	case *tg.MessageMediaGame:
		return "game"
	case *tg.MessageMediaInvoice:
		return "invoice"
	case *tg.MessageMediaEmpty:
		return ""
	default:
		return "other"
	}
}

// resolveSenderName looks up a stored display name for a sender.
func resolveSenderName(db *store.DB, senderID int64) string {
	if senderID == 0 {
		return ""
	}
	u, err := GetUser(db, senderID)
	if err != nil || u == nil {
		return ""
	}
	return u.DisplayName()
}

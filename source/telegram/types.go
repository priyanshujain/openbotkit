package telegram

import "time"

// Peer kinds stored in telegram_chats.type.
const (
	PeerUser    = "user"
	PeerGroup   = "group"
	PeerChannel = "channel"
)

// channelIDOffset mirrors the Bot API "-100" prefix for channels and
// supergroups. Telegram numbers users, legacy chats and channels in separate
// spaces, so a raw ID is not unique on its own; normalising to the Bot API form
// makes chat_id safe as a primary key and matches the IDs users see elsewhere.
const channelIDOffset = 1000000000000

func ChatIDFromUser(id int64) int64    { return id }
func ChatIDFromGroup(id int64) int64   { return -id }
func ChatIDFromChannel(id int64) int64 { return -(channelIDOffset + id) }

// SplitChatID reverses the normalisation, returning the peer kind and the raw
// Telegram ID for that kind.
func SplitChatID(chatID int64) (kind string, rawID int64) {
	switch {
	case chatID > 0:
		return PeerUser, chatID
	case chatID <= -channelIDOffset:
		return PeerChannel, -chatID - channelIDOffset
	default:
		return PeerGroup, -chatID
	}
}

// stubChat is everything a chat ID alone can say about a peer. A supergroup
// looks like a plain channel here, which is why it must never overwrite a row
// built from real entities.
func stubChat(chatID int64) *Chat {
	kind, _ := SplitChatID(chatID)
	return &Chat{
		ChatID:    chatID,
		Type:      kind,
		IsGroup:   kind == PeerGroup,
		IsChannel: kind == PeerChannel,
	}
}

type Message struct {
	MessageID  int
	ChatID     int64
	SenderID   int64
	SenderName string
	Text       string
	Timestamp  time.Time
	MediaType  string
	ReplyToID  int
	IsOutgoing bool
	EditDate   *time.Time
}

type Chat struct {
	ChatID        int64
	Type          string
	Title         string
	Username      string
	AccessHash    int64
	IsGroup       bool
	IsChannel     bool
	LastMessageAt *time.Time
}

type User struct {
	UserID     int64
	Username   string
	FirstName  string
	LastName   string
	Phone      string
	AccessHash int64
	IsBot      bool
}

// DisplayName renders a user as "First Last", falling back to @username.
func (u User) DisplayName() string {
	name := u.FirstName
	if u.LastName != "" {
		if name != "" {
			name += " "
		}
		name += u.LastName
	}
	if name == "" && u.Username != "" {
		name = "@" + u.Username
	}
	return name
}

type SyncState struct {
	ChatID          int64
	MinID           int
	MaxID           int
	BackfilledUntil *time.Time
}

type Config struct {
	SessionPath string
}

type BackfillOptions struct {
	// Since bounds how far back history is walked. Zero means no bound.
	Since time.Time
	// PerChatLimit caps messages stored per chat. Zero means unlimited.
	PerChatLimit int
	// Full ignores stored per-chat progress and restarts from the newest message.
	Full bool
}

type BackfillResult struct {
	Chats    int
	Messages int
	Errors   int
}

type SendInput struct {
	ChatID int64
	Text   string
}

type SendResult struct {
	MessageID int
	Timestamp time.Time
}

type ListOptions struct {
	ChatID int64
	After  string // YYYY-MM-DD
	Before string // YYYY-MM-DD
	Limit  int
	Offset int
}

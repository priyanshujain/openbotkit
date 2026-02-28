package whatsapp

import "time"

type Message struct {
	MessageID  string
	ChatJID    string
	SenderJID  string
	SenderName string
	Text       string
	Timestamp  time.Time
	MediaType  string
	MediaPath  string
	IsGroup    bool
	IsFromMe   bool
	ReplyToID  string
}

type Chat struct {
	JID           string
	Name          string
	IsGroup       bool
	LastMessageAt *time.Time
}

type Config struct {
	SessionDBPath string
}

type SyncOptions struct {
	Follow bool
}

type SyncResult struct {
	Received        int
	HistoryMessages int
	Errors          int
}

type ListOptions struct {
	ChatJID string
	After   string // YYYY-MM-DD
	Before  string // YYYY-MM-DD
	Limit   int
	Offset  int
}

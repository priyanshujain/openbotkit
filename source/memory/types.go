package memory

import "time"

type Config struct {
	DataDBPath string
}

type Conversation struct {
	SessionID string
	StartedAt time.Time
	CWD       string
	MsgCount  int
}

type Message struct {
	ConversationID int64
	Role           string // "user" or "assistant"
	Content        string
	Timestamp      time.Time
}

type CaptureInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
}

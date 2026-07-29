package slack

import "encoding/json"

type Message struct {
	TS          string          `json:"ts"`
	User        string          `json:"user,omitempty"`
	Text        string          `json:"text"`
	ThreadTS    string          `json:"thread_ts,omitempty"`
	ReplyCount  int             `json:"reply_count,omitempty"`
	Reactions   []Reaction      `json:"reactions,omitempty"`
	Files       []File          `json:"files,omitempty"`
	Username    string          `json:"username,omitempty"`
	BotID       string          `json:"bot_id,omitempty"`
	SubType     string          `json:"subtype,omitempty"`
	Blocks      json.RawMessage `json:"blocks,omitempty"`
	Attachments json.RawMessage `json:"attachments,omitempty"`
	Edited      *Edited         `json:"edited,omitempty"`

	// Derived on output, never sent by Slack.
	MessageID string `json:"message_id,omitempty"`
	UserName  string `json:"user_name,omitempty"`
}

type Edited struct {
	User string `json:"user,omitempty"`
	TS   string `json:"ts,omitempty"`
}

type Bot struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	AppID  string `json:"app_id,omitempty"`
	UserID string `json:"user_id,omitempty"`
}

type Reaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users,omitempty"`
	Count int      `json:"count"`
}

type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsPrivate  bool   `json:"is_private,omitempty"`
	NumMembers int    `json:"num_members,omitempty"`
	Topic      *Topic `json:"topic,omitempty"`
	Purpose    *Topic `json:"purpose,omitempty"`
	IsMember   bool   `json:"is_member,omitempty"`
	IsArchived bool   `json:"is_archived,omitempty"`
}

type Topic struct {
	Value string `json:"value"`
}

type User struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	RealName    string       `json:"real_name,omitempty"`
	DisplayName string       `json:"display_name,omitempty"`
	Profile     *UserProfile `json:"profile,omitempty"`
	Deleted     bool         `json:"deleted,omitempty"`
	IsBot       bool         `json:"is_bot,omitempty"`
}

type UserProfile struct {
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
}

type File struct {
	ID                 string `json:"id"`
	Name               string `json:"name,omitempty"`
	FileType           string `json:"filetype,omitempty"`
	Mimetype           string `json:"mimetype,omitempty"`
	URLPrivate         string `json:"url_private,omitempty"`
	URLPrivateDownload string `json:"url_private_download,omitempty"`
	Permalink          string `json:"permalink,omitempty"`
	Preview            string `json:"preview,omitempty"`
	PlainText          string `json:"plain_text,omitempty"`
	Size               int    `json:"size,omitempty"`
}

type SearchOptions struct {
	Page  int
	Count int
}

type HistoryOptions struct {
	Limit  int
	Cursor string
	Oldest string
	Latest string
}

type SearchResult struct {
	Messages []Message
	Total    int
	Page     int
	Pages    int
}

type FileSearchResult struct {
	Files []File
	Total int
	Page  int
	Pages int
}

type apiResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Metadata struct {
		NextCursor string `json:"next_cursor,omitempty"`
	} `json:"response_metadata,omitempty"`
}

package slack

import (
	"context"
	"io"
)

type API interface {
	SearchMessages(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error)
	SearchFiles(ctx context.Context, query string, opts SearchOptions) (*FileSearchResult, error)
	ConversationsHistory(ctx context.Context, channel string, opts HistoryOptions) ([]Message, error)
	ConversationsReplies(ctx context.Context, channel, threadTS string, opts HistoryOptions) ([]Message, error)
	ConversationsRepliesAll(ctx context.Context, channel, threadTS string, opts HistoryOptions) ([]Message, error)
	ConversationsList(ctx context.Context) ([]Channel, error)
	UsersList(ctx context.Context) ([]User, error)
	UsersInfo(ctx context.Context, userID string) (*User, error)
	BotsInfo(ctx context.Context, botID string) (*Bot, error)
	FilesInfo(ctx context.Context, fileID string) (*File, error)
	DownloadFile(ctx context.Context, url string, w io.Writer) error
	PostMessage(ctx context.Context, channel, text, threadTS string) (string, error)
	UpdateMessage(ctx context.Context, channel, ts, text string) error
	DeleteMessage(ctx context.Context, channel, ts string) error
	AddReaction(ctx context.Context, channel, ts, emoji string) error
	RemoveReaction(ctx context.Context, channel, ts, emoji string) error
	ResolveChannel(ctx context.Context, ref string) (string, error)
	ResolveUser(ctx context.Context, ref string) (string, error)
}

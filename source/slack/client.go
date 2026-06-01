package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const slackBaseURL = "https://slack.com/api"

type Client struct {
	token    string
	cookie   string
	baseURL  string
	http     *http.Client
	resolver *Resolver
}

func NewClient(token, cookie string) *Client {
	c := &Client{
		token:   token,
		cookie:  cookie,
		baseURL: slackBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	c.resolver = NewResolver(c)
	return c
}

func (c *Client) isBrowserToken() bool {
	return strings.HasPrefix(c.token, "xoxc-")
}

func (c *Client) call(ctx context.Context, method string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}
	if c.isBrowserToken() {
		params.Set("token", c.token)
	}

	const maxRetries = 3
	for attempt := range maxRetries {
		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/"+method, strings.NewReader(params.Encode()))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		if !c.isBrowserToken() {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		if c.cookie != "" {
			req.Header.Set("Cookie", "d="+c.cookie)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		if resp.StatusCode == 429 {
			retryAfter := 1
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if v, err := strconv.Atoi(ra); err == nil {
					retryAfter = v
				}
			}
			if attempt < maxRetries-1 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(retryAfter) * time.Second):
					continue
				}
			}
			return nil, fmt.Errorf("rate limited after %d retries", maxRetries)
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("slack API %s: HTTP %d", method, resp.StatusCode)
		}

		var ar apiResponse
		if err := json.Unmarshal(body, &ar); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if !ar.OK {
			return nil, fmt.Errorf("slack API %s: %s", method, ar.Error)
		}

		return body, nil
	}
	return nil, fmt.Errorf("unreachable")
}

// Read methods

func (c *Client) SearchMessages(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	params := url.Values{"query": {query}}
	if opts.Count > 0 {
		params.Set("count", strconv.Itoa(opts.Count))
	}
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}

	body, err := c.call(ctx, "search.messages", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Messages struct {
			Total   int `json:"total"`
			Page    int `json:"page"`
			Pages   int `json:"pages"`
			Matches []Message `json:"matches"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}

	return &SearchResult{
		Messages: resp.Messages.Matches,
		Total:    resp.Messages.Total,
		Page:     resp.Messages.Page,
		Pages:    resp.Messages.Pages,
	}, nil
}

func (c *Client) SearchFiles(ctx context.Context, query string, opts SearchOptions) (*FileSearchResult, error) {
	params := url.Values{"query": {query}}
	if opts.Count > 0 {
		params.Set("count", strconv.Itoa(opts.Count))
	}
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}

	body, err := c.call(ctx, "search.files", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Files struct {
			Total   int    `json:"total"`
			Page    int    `json:"page"`
			Pages   int    `json:"pages"`
			Matches []File `json:"matches"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse file search results: %w", err)
	}

	return &FileSearchResult{
		Files: resp.Files.Matches,
		Total: resp.Files.Total,
		Page:  resp.Files.Page,
		Pages: resp.Files.Pages,
	}, nil
}

func (c *Client) ConversationsHistory(ctx context.Context, channel string, opts HistoryOptions) ([]Message, error) {
	params := url.Values{"channel": {channel}}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		params.Set("cursor", opts.Cursor)
	}
	if opts.Oldest != "" {
		params.Set("oldest", opts.Oldest)
	}
	if opts.Latest != "" {
		params.Set("latest", opts.Latest)
	}

	body, err := c.call(ctx, "conversations.history", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse history: %w", err)
	}
	return resp.Messages, nil
}

func (c *Client) ConversationsReplies(ctx context.Context, channel, threadTS string, opts HistoryOptions) ([]Message, error) {
	params := url.Values{
		"channel": {channel},
		"ts":      {threadTS},
	}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		params.Set("cursor", opts.Cursor)
	}

	body, err := c.call(ctx, "conversations.replies", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse replies: %w", err)
	}
	return resp.Messages, nil
}

func (c *Client) ConversationsList(ctx context.Context) ([]Channel, error) {
	var all []Channel
	cursor := ""
	for {
		params := url.Values{
			"types":            {"public_channel,private_channel"},
			"exclude_archived": {"true"},
			"limit":            {"200"},
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, err := c.call(ctx, "conversations.list", params)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Channels []Channel `json:"channels"`
			apiResponse
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse channels: %w", err)
		}
		all = append(all, resp.Channels...)

		cursor = resp.Metadata.NextCursor
		if cursor == "" {
			break
		}
	}
	return all, nil
}

func (c *Client) UsersList(ctx context.Context) ([]User, error) {
	var all []User
	cursor := ""
	for {
		params := url.Values{"limit": {"200"}}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, err := c.call(ctx, "users.list", params)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Members []User `json:"members"`
			apiResponse
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse users: %w", err)
		}
		all = append(all, resp.Members...)

		cursor = resp.Metadata.NextCursor
		if cursor == "" {
			break
		}
	}
	return all, nil
}

func (c *Client) UsersInfo(ctx context.Context, userID string) (*User, error) {
	body, err := c.call(ctx, "users.info", url.Values{"user": {userID}})
	if err != nil {
		return nil, err
	}

	var resp struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse user info: %w", err)
	}
	return &resp.User, nil
}

// Write methods

func (c *Client) PostMessage(ctx context.Context, channel, text, threadTS string) (string, error) {
	params := url.Values{
		"channel": {channel},
		"text":    {text},
	}
	if threadTS != "" {
		params.Set("thread_ts", threadTS)
	}

	body, err := c.call(ctx, "chat.postMessage", params)
	if err != nil {
		return "", err
	}

	var resp struct {
		TS string `json:"ts"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse post response: %w", err)
	}
	return resp.TS, nil
}

func (c *Client) UpdateMessage(ctx context.Context, channel, ts, text string) error {
	_, err := c.call(ctx, "chat.update", url.Values{
		"channel": {channel},
		"ts":      {ts},
		"text":    {text},
	})
	return err
}

func (c *Client) DeleteMessage(ctx context.Context, channel, ts string) error {
	_, err := c.call(ctx, "chat.delete", url.Values{
		"channel": {channel},
		"ts":      {ts},
	})
	return err
}

func (c *Client) AddReaction(ctx context.Context, channel, ts, emoji string) error {
	_, err := c.call(ctx, "reactions.add", url.Values{
		"channel":   {channel},
		"timestamp": {ts},
		"name":      {emoji},
	})
	return err
}

func (c *Client) RemoveReaction(ctx context.Context, channel, ts, emoji string) error {
	_, err := c.call(ctx, "reactions.remove", url.Values{
		"channel":   {channel},
		"timestamp": {ts},
		"name":      {emoji},
	})
	return err
}

// ResolveChannel resolves a channel reference to an ID using the client's Resolver.
func (c *Client) ResolveChannel(ctx context.Context, ref string) (string, error) {
	return c.resolver.ResolveChannel(ctx, ref)
}

// ResolveUser resolves a user reference to an ID using the client's Resolver.
func (c *Client) ResolveUser(ctx context.Context, ref string) (string, error) {
	return c.resolver.ResolveUser(ctx, ref)
}

// AuthTest validates the current credentials and returns team/user info.
func (c *Client) AuthTest(ctx context.Context) (teamID, teamName, userID string, err error) {
	body, err := c.call(ctx, "auth.test", nil)
	if err != nil {
		return "", "", "", err
	}

	var resp struct {
		TeamID string `json:"team_id"`
		Team   string `json:"team"`
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", "", fmt.Errorf("parse auth.test: %w", err)
	}
	return resp.TeamID, resp.Team, resp.UserID, nil
}

package client

import (
	"context"
	"encoding/json"
)

func (c *Client) HomeTimeline(ctx context.Context, count int, cursor string) (json.RawMessage, error) {
	vars := map[string]any{
		"count":                  count,
		"includePromotedContent": true,
		"latestControlAvailable": true,
		"requestContext":         "launch",
		"withCommunity":          true,
		"seenTweetIds":           []string{},
	}
	if cursor != "" {
		vars["cursor"] = cursor
	}
	return c.graphqlGET(ctx, "HomeTimeline", vars, DefaultFeatures())
}

func (c *Client) HomeLatestTimeline(ctx context.Context, count int, cursor string) (json.RawMessage, error) {
	vars := map[string]any{
		"count":                  count,
		"includePromotedContent": true,
		"latestControlAvailable": true,
		"requestContext":         "launch",
		"withCommunity":          true,
		"seenTweetIds":           []string{},
	}
	if cursor != "" {
		vars["cursor"] = cursor
	}
	return c.graphqlGET(ctx, "HomeLatestTimeline", vars, DefaultFeatures())
}

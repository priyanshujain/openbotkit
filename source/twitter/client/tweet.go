package client

import (
	"context"
	"encoding/json"
)

func (c *Client) GetTweet(ctx context.Context, tweetID string) (json.RawMessage, error) {
	vars := map[string]any{
		"focalTweetId":                           tweetID,
		"with_rux_injections":                    false,
		"includePromotedContent":                 true,
		"withCommunity":                          true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withBirdwatchNotes":                     true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	return c.graphqlGET(ctx, "TweetDetail", vars, DefaultFeatures())
}

func (c *Client) CreateTweet(ctx context.Context, text string, replyToID string) (json.RawMessage, error) {
	vars := map[string]any{
		"tweet_text":              text,
		"dark_request":            false,
		"media":                   map[string]any{"media_entities": []any{}, "possibly_sensitive": false},
		"semantic_annotation_ids": []any{},
	}
	if replyToID != "" {
		vars["reply"] = map[string]any{
			"in_reply_to_tweet_id":   replyToID,
			"exclude_reply_user_ids": []any{},
		}
	}
	return c.graphqlPOST(ctx, "CreateTweet", vars, DefaultFeatures())
}

func (c *Client) CreateRetweet(ctx context.Context, tweetID string) (json.RawMessage, error) {
	vars := map[string]any{
		"tweet_id":     tweetID,
		"dark_request": false,
	}
	return c.graphqlPOST(ctx, "CreateRetweet", vars, nil)
}

func (c *Client) FavoriteTweet(ctx context.Context, tweetID string) (json.RawMessage, error) {
	vars := map[string]any{
		"tweet_id": tweetID,
	}
	return c.graphqlPOST(ctx, "FavoriteTweet", vars, nil)
}

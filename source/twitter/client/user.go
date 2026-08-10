package client

import (
	"context"
	"encoding/json"
)

func (c *Client) UserByScreenName(ctx context.Context, screenName string) (json.RawMessage, error) {
	vars := map[string]any{
		"screen_name":              screenName,
		"withSafetyModeUserFields": true,
	}
	features := map[string]any{
		"hidden_profile_subscriptions_enabled":                              true,
		"rweb_tipjar_consumption_enabled":                                   true,
		"responsive_web_graphql_exclude_directive_enabled":                  true,
		"verified_phone_label_enabled":                                      false,
		"highlights_tweets_tab_ui_enabled":                                  true,
		"responsive_web_twitter_article_notes_tab_enabled":                  true,
		"subscriptions_feature_can_gift_premium":                            true,
		"creator_subscriptions_tweet_preview_api_enabled":                   true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"responsive_web_graphql_timeline_navigation_enabled":                true,
	}
	return c.graphqlGET(ctx, "UserByScreenName", vars, features)
}

func (c *Client) UserTweets(ctx context.Context, userID string, count int, cursor string) (json.RawMessage, error) {
	vars := map[string]any{
		"userId":                                 userID,
		"count":                                  count,
		"includePromotedContent":                 true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	if cursor != "" {
		vars["cursor"] = cursor
	}
	return c.graphqlGET(ctx, "UserTweets", vars, DefaultFeatures())
}

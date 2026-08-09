package twitter

import (
	"encoding/json"
	"fmt"
	"time"
)

const xTimeFmt = "Mon Jan 02 15:04:05 -0700 2006"

// ParseTimelineResponse parses a URT (Unified Rich Timeline) response from
// HomeTimeline or HomeLatestTimeline endpoints.
func ParseTimelineResponse(raw json.RawMessage) ([]Tweet, string, error) {
	var resp struct {
		Data struct {
			Home struct {
				HomeTimelineURT struct {
					Instructions []instruction `json:"instructions"`
				} `json:"home_timeline_urt"`
			} `json:"home"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("parse timeline response: %w", err)
	}

	var tweets []Tweet
	var cursor string
	for _, inst := range resp.Data.Home.HomeTimelineURT.Instructions {
		t, c := extractFromInstruction(inst)
		tweets = append(tweets, t...)
		if c != "" {
			cursor = c
		}
	}
	return tweets, cursor, nil
}

// ParseTweetDetail parses a TweetDetail response, returning the focal tweet
// and any thread replies.
func ParseTweetDetail(raw json.RawMessage) (*Tweet, []Tweet, error) {
	var resp struct {
		Data struct {
			TweetResult struct {
				Result tweetResult `json:"result"`
			} `json:"tweetResult"`
			ThreadedConversation struct {
				Instructions []instruction `json:"instructions"`
			} `json:"threaded_conversation_with_injections_v2"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("parse tweet detail: %w", err)
	}

	focal := extractTweet(resp.Data.TweetResult.Result)

	var replies []Tweet
	for _, inst := range resp.Data.ThreadedConversation.Instructions {
		t, _ := extractFromInstruction(inst)
		replies = append(replies, t...)
	}

	return focal, replies, nil
}

// ParseSearchResponse parses a SearchTimeline response.
func ParseSearchResponse(raw json.RawMessage) ([]Tweet, string, error) {
	var resp struct {
		Data struct {
			SearchByRawQuery struct {
				SearchTimeline struct {
					Timeline struct {
						Instructions []instruction `json:"instructions"`
					} `json:"timeline"`
				} `json:"search_timeline"`
			} `json:"search_by_raw_query"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("parse search response: %w", err)
	}

	var tweets []Tweet
	var cursor string
	for _, inst := range resp.Data.SearchByRawQuery.SearchTimeline.Timeline.Instructions {
		t, c := extractFromInstruction(inst)
		tweets = append(tweets, t...)
		if c != "" {
			cursor = c
		}
	}
	return tweets, cursor, nil
}

// ParseNotifications parses a Notifications response.
func ParseNotifications(raw json.RawMessage) ([]Tweet, string, error) {
	var resp struct {
		Data struct {
			Viewer struct {
				Timeline struct {
					Timeline struct {
						Instructions []instruction `json:"instructions"`
					} `json:"timeline"`
				} `json:"notifications_timeline"`
			} `json:"viewer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("parse notifications: %w", err)
	}

	var tweets []Tweet
	var cursor string
	for _, inst := range resp.Data.Viewer.Timeline.Timeline.Instructions {
		t, c := extractFromInstruction(inst)
		tweets = append(tweets, t...)
		if c != "" {
			cursor = c
		}
	}
	return tweets, cursor, nil
}

type instruction struct {
	Type    string  `json:"type"`
	Entries []entry `json:"entries"`
}

type entry struct {
	EntryID   string       `json:"entryId"`
	SortIndex string       `json:"sortIndex"`
	Content   entryContent `json:"content"`
}

type entryContent struct {
	EntryType   string       `json:"entryType"`
	ItemContent *itemContent `json:"itemContent,omitempty"`
	Value       string       `json:"value,omitempty"`
	CursorType  string       `json:"cursorType,omitempty"`
	Items       []subEntry   `json:"items,omitempty"`
}

type subEntry struct {
	EntryID string `json:"entryId"`
	Item    struct {
		ItemContent *itemContent `json:"itemContent,omitempty"`
	} `json:"item"`
}

type itemContent struct {
	ItemType     string `json:"itemType"`
	TweetResults *struct {
		Result tweetResult `json:"result"`
	} `json:"tweet_results,omitempty"`
}

type tweetResult struct {
	TypeName     string       `json:"__typename"`
	RestID       string       `json:"rest_id"`
	Core         *tweetCore   `json:"core,omitempty"`
	Legacy       *tweetLegacy `json:"legacy,omitempty"`
	Tweet        *tweetResult `json:"tweet,omitempty"` // for TweetWithVisibilityResults
	QuotedStatus *struct {
		Result tweetResult `json:"result"`
	} `json:"quoted_status_result,omitempty"`
}

type tweetCore struct {
	UserResults struct {
		Result struct {
			RestID string     `json:"rest_id"`
			Legacy userLegacy `json:"legacy"`
		} `json:"result"`
	} `json:"user_results"`
}

type userLegacy struct {
	ScreenName string `json:"screen_name"`
	Name       string `json:"name"`
}

type tweetLegacy struct {
	FullText              string `json:"full_text"`
	CreatedAt             string `json:"created_at"`
	ConversationID        string `json:"conversation_id_str"`
	InReplyToStatusID     string `json:"in_reply_to_status_id_str"`
	RetweetCount          int    `json:"retweet_count"`
	FavoriteCount         int    `json:"favorite_count"`
	ReplyCount            int    `json:"reply_count"`
	RetweetedStatusResult *struct {
		Result tweetResult `json:"result"`
	} `json:"retweeted_status_result,omitempty"`
}

func extractFromInstruction(inst instruction) ([]Tweet, string) {
	var tweets []Tweet
	var cursor string

	for _, e := range inst.Entries {
		if e.Content.CursorType == "Bottom" && e.Content.Value != "" {
			cursor = e.Content.Value
			continue
		}

		if e.Content.ItemContent != nil {
			if tw := extractFromItemContent(e.Content.ItemContent); tw != nil {
				tweets = append(tweets, *tw)
			}
		}

		for _, sub := range e.Content.Items {
			if sub.Item.ItemContent != nil {
				if tw := extractFromItemContent(sub.Item.ItemContent); tw != nil {
					tweets = append(tweets, *tw)
				}
			}
		}
	}
	return tweets, cursor
}

func extractFromItemContent(ic *itemContent) *Tweet {
	if ic.TweetResults == nil {
		return nil
	}
	return extractTweet(ic.TweetResults.Result)
}

func extractTweet(r tweetResult) *Tweet {
	// Handle TweetWithVisibilityResults wrapper
	if r.TypeName == "TweetWithVisibilityResults" && r.Tweet != nil {
		return extractTweet(*r.Tweet)
	}

	if r.Legacy == nil {
		return nil
	}

	tw := &Tweet{
		TweetID:        r.RestID,
		Text:           r.Legacy.FullText,
		ConversationID: r.Legacy.ConversationID,
		InReplyTo:      r.Legacy.InReplyToStatusID,
		RetweetCount:   r.Legacy.RetweetCount,
		LikeCount:      r.Legacy.FavoriteCount,
		ReplyCount:     r.Legacy.ReplyCount,
	}

	if t, err := time.Parse(xTimeFmt, r.Legacy.CreatedAt); err == nil {
		tw.CreatedAt = t
	}

	if r.Core != nil {
		tw.UserID = r.Core.UserResults.Result.RestID
		tw.UserName = r.Core.UserResults.Result.Legacy.ScreenName
		tw.UserFullName = r.Core.UserResults.Result.Legacy.Name
	}

	if r.Legacy.RetweetedStatusResult != nil {
		tw.IsRetweet = true
		rt := r.Legacy.RetweetedStatusResult.Result
		if rt.Core != nil {
			tw.RetweetedFrom = rt.Core.UserResults.Result.Legacy.ScreenName
		}
	}

	if r.QuotedStatus != nil {
		tw.QuotedTweetID = r.QuotedStatus.Result.RestID
	}

	return tw
}

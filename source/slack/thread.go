package slack

import (
	"context"
	"fmt"
	"regexp"
	"sort"
)

// Thread is a complete Slack thread with every referenced ID resolved to a name.
type Thread struct {
	ChannelID string            `json:"channel_id"`
	ThreadID  string            `json:"thread_id"`
	ThreadTS  string            `json:"thread_ts"`
	Users     map[string]string `json:"users,omitempty"`
	Messages  []Message         `json:"messages"`
}

// mentionRe matches <@U123>, <@U123|name>, and the bot form <@B123>.
var mentionRe = regexp.MustCompile(`<@([UWB][A-Z0-9]+)[|>]`)

// FetchThread returns the whole thread containing ts, regardless of whether ts
// names the root or a reply. Passing cache as nil skips name resolution.
func FetchThread(ctx context.Context, api API, cache *UserCache, channelID, ts string) (*Thread, error) {
	rootTS, err := MessageIDToTS(ts)
	if err != nil {
		return nil, err
	}

	msgs, err := api.ConversationsRepliesAll(ctx, channelID, rootTS, HistoryOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetch thread: %w", err)
	}

	// Asking for a reply returns that reply pointing at the real root; follow it.
	if len(msgs) > 0 && msgs[0].ThreadTS != "" && msgs[0].ThreadTS != rootTS {
		rootTS = msgs[0].ThreadTS
		msgs, err = api.ConversationsRepliesAll(ctx, channelID, rootTS, HistoryOptions{})
		if err != nil {
			return nil, fmt.Errorf("fetch thread root: %w", err)
		}
	}

	thread := &Thread{
		ChannelID: channelID,
		ThreadID:  TSToMessageID(rootTS),
		ThreadTS:  rootTS,
		Messages:  msgs,
	}

	names := resolveNames(ctx, cache, msgs)
	for i := range thread.Messages {
		m := &thread.Messages[i]
		m.MessageID = TSToMessageID(m.TS)
		if name, ok := names[m.User]; ok && m.User != "" {
			m.UserName = name
		} else if name, ok := names[m.BotID]; ok && m.BotID != "" {
			m.UserName = name
		}
	}
	if len(names) > 0 {
		thread.Users = names
	}
	return thread, nil
}

// resolveNames looks up every ID the thread references: authors, in-text
// mentions, reaction voters, editors, and bots. The users map is the only way
// to resolve an ID, since message text is left raw, so a partial map would
// leave IDs dangling.
func resolveNames(ctx context.Context, cache *UserCache, msgs []Message) map[string]string {
	if cache == nil {
		return nil
	}

	seen := make(map[string]bool)
	var ids []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}

	for _, m := range msgs {
		// A bot's own message carries its display name; seeding avoids bots.info.
		if m.BotID != "" && m.Username != "" {
			cache.Seed(m.BotID, m.Username)
		}
		add(m.User)
		add(m.BotID)
		if m.Edited != nil {
			add(m.Edited.User)
		}
		for _, r := range m.Reactions {
			for _, u := range r.Users {
				add(u)
			}
		}
		for _, match := range mentionRe.FindAllStringSubmatch(m.Text, -1) {
			add(match[1])
		}
		for _, match := range mentionRe.FindAllStringSubmatch(string(m.Attachments), -1) {
			add(match[1])
		}
		for _, match := range mentionRe.FindAllStringSubmatch(string(m.Blocks), -1) {
			add(match[1])
		}
	}

	sort.Strings(ids)
	names, err := cache.Names(ctx, ids)
	if err != nil {
		return nil
	}
	return names
}

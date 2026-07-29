package slack

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var slackURLChannelRe = regexp.MustCompile(`/archives/([A-Z0-9]+)`)

type Resolver struct {
	api API

	mu       sync.Mutex
	channels []Channel
	users    []User
}

func NewResolver(api API) *Resolver {
	return &Resolver{api: api}
}

func (r *Resolver) ResolveChannel(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)

	// Already a channel ID (C/G/D prefix + alphanumeric).
	if isChannelID(ref) {
		return ref, nil
	}

	// Slack URL: https://team.slack.com/archives/C0123ABC
	if m := slackURLChannelRe.FindStringSubmatch(ref); len(m) == 2 {
		return m[1], nil
	}

	// Channel name with or without #.
	name := strings.TrimPrefix(ref, "#")
	channels, err := r.loadChannels(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve channel %q: %w", ref, err)
	}
	for _, ch := range channels {
		if ch.Name == name {
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("channel %q not found", ref)
}

func (r *Resolver) ResolveUser(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)

	// Already a user ID.
	if isUserID(ref) {
		return ref, nil
	}

	users, err := r.loadUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve user %q: %w", ref, err)
	}

	handle := strings.TrimPrefix(ref, "@")

	for _, u := range users {
		if u.Name == handle {
			return u.ID, nil
		}
		if u.Profile != nil && u.Profile.DisplayName == handle {
			return u.ID, nil
		}
		// Email match.
		if u.Profile != nil && u.Profile.Email != "" && u.Profile.Email == ref {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("user %q not found", ref)
}

func (r *Resolver) loadChannels(ctx context.Context) ([]Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.channels != nil {
		return r.channels, nil
	}
	channels, err := r.api.ConversationsList(ctx)
	if err != nil {
		return nil, err
	}
	r.channels = channels
	return channels, nil
}

func (r *Resolver) loadUsers(ctx context.Context) ([]User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.users != nil {
		return r.users, nil
	}
	users, err := r.api.UsersList(ctx)
	if err != nil {
		return nil, err
	}
	r.users = users
	return users, nil
}

func isChannelID(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] != 'C' && s[0] != 'G' && s[0] != 'D' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func isUserID(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] != 'U' && s[0] != 'W' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

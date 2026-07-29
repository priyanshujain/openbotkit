package tools

import (
	"github.com/73ai/openbotkit/source/slack"
)

type SlackToolDeps struct {
	Client        slack.API
	Resolver      *slack.Resolver
	Interactor    Interactor
	ApprovalRules *ApprovalRuleSet
	Workspace     string // enables ID -> name resolution
	ScratchDir    string // where downloaded media is written
	UserCache     *slack.UserCache
}

// SlackResolver returns the shared Resolver, creating one if needed.
func (d SlackToolDeps) SlackResolver() *slack.Resolver {
	if d.Resolver != nil {
		return d.Resolver
	}
	return slack.NewResolver(d.Client)
}

// SlackUserCache returns the name cache, or nil when no workspace is
// configured — callers treat nil as "skip name resolution".
func (d SlackToolDeps) SlackUserCache() *slack.UserCache {
	if d.UserCache != nil {
		return d.UserCache
	}
	if d.Workspace == "" {
		return nil
	}
	return slack.NewUserCache(d.Workspace, d.Client)
}

// truncateUTF8 truncates s to at most maxRunes runes, appending "..." if truncated.
func truncateUTF8(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

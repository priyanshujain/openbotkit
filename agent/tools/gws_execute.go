package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	neturl "net/url"
	"sort"
	"strings"
	"time"

	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/oauth/google"
)

const defaultAuthTimeout = 5 * time.Minute

var errAuthPending = errors.New("authorization pending")

const authPendingMessage = "I've sent an authorization link. Please tap it to grant access, then try your request again."

// GWSToolConfig configures a GWSExecuteTool.
type GWSToolConfig struct {
	Interactor      Interactor
	ScopeChecker    ScopeChecker
	Bridge          *TokenBridge
	ScopeWaiter     *google.ScopeWaiter
	Google          *google.Google
	Account         string
	Manifest        *skills.Manifest
	Runner          CommandRunner
	AuthTimeout     time.Duration
	ApprovalRules   *ApprovalRuleSet
	AuthRedirectURL string
}

// GWSExecuteTool routes all gws commands through a single tool
// with scope checking, progressive consent, and write approval.
type GWSExecuteTool struct {
	interactor      Interactor
	scopeChecker    ScopeChecker
	bridge          *TokenBridge
	scopeWaiter     *google.ScopeWaiter
	google          *google.Google
	account         string
	manifest        *skills.Manifest
	runner          CommandRunner
	authTimeout     time.Duration
	approvalRules   *ApprovalRuleSet
	authRedirectURL string
}

func NewGWSExecuteTool(cfg GWSToolConfig) *GWSExecuteTool {
	timeout := cfg.AuthTimeout
	if timeout == 0 {
		timeout = defaultAuthTimeout
	}
	return &GWSExecuteTool{
		interactor:      cfg.Interactor,
		scopeChecker:    cfg.ScopeChecker,
		bridge:          cfg.Bridge,
		scopeWaiter:     cfg.ScopeWaiter,
		google:          cfg.Google,
		account:         cfg.Account,
		manifest:        cfg.Manifest,
		runner:          cfg.Runner,
		authTimeout:     timeout,
		approvalRules:   cfg.ApprovalRules,
		authRedirectURL: cfg.AuthRedirectURL,
	}
}

func (g *GWSExecuteTool) Name() string        { return "gws_execute" }
func (g *GWSExecuteTool) Description() string { return "Execute a Google Workspace CLI (gws) command" }
func (g *GWSExecuteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The gws command without --params or --json flags (e.g. 'drive files list', 'calendar events list')"
			},
			"params": {
				"type": "object",
				"description": "URL/query parameters as a JSON object (becomes --params flag)"
			},
			"body": {
				"type": "object",
				"description": "Data for the command. For API methods, sent as --json. For helper commands (+write, +insert, etc.), each field is automatically expanded to individual --flags."
			},
			"flags": {
				"type": "object",
				"description": "Named CLI flags as key-value pairs. Each key becomes a --flag with its value. Use for helper commands (+write, +insert, etc.) that need specific flags like --document, --text, --summary."
			}
		},
		"required": ["command"]
	}`)
}

type gwsInput struct {
	Command string          `json:"command"`
	Params  json.RawMessage `json:"params,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
	Flags   json.RawMessage `json:"flags,omitempty"`
}

func (g *GWSExecuteTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	// Discover account if not yet set (e.g. after first-time OAuth completed).
	if g.account == "" {
		if accounts, err := g.google.Accounts(ctx); err == nil && len(accounts) > 0 {
			g.account = accounts[0]
			g.bridge.SetAccount(accounts[0])
		}
	}

	var in gwsInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if in.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	slog.Info("gws_execute called", "command", in.Command)
	args := splitCommand(in.Command)
	// Strip leading "gws" if present.
	if len(args) > 0 && args[0] == "gws" {
		args = args[1:]
	}
	// Append structured params.
	if len(in.Params) > 0 && string(in.Params) != "null" {
		args = append(args, "--params", string(in.Params))
	}
	// Smart body expansion: helpers get individual flags, raw API gets --json.
	if len(in.Body) > 0 && string(in.Body) != "null" {
		if isHelperCommand(args) {
			expanded, err := expandBodyAsFlags(in.Body)
			if err != nil {
				return "", fmt.Errorf("expand body flags: %w", err)
			}
			args = append(args, expanded...)
		} else {
			args = append(args, "--json", string(in.Body))
		}
	}
	// Explicit flags field — always expanded as --key value.
	if len(in.Flags) > 0 && string(in.Flags) != "null" {
		expanded, err := expandBodyAsFlags(in.Flags)
		if err != nil {
			return "", fmt.Errorf("expand flags: %w", err)
		}
		args = append(args, expanded...)
	}
	service := gwsServiceFromCommand(args)
	isWrite := g.isWriteCommand(args)

	// Scope check + progressive consent.
	if service != "" {
		requiredScopes := g.scopesForService(service)
		if len(requiredScopes) > 0 {
			has, err := g.scopeChecker.HasScopes(g.account, requiredScopes)
			if err != nil {
				return "", fmt.Errorf("check scopes: %w", err)
			}
			if !has {
				if err := g.requestConsent(ctx, requiredScopes); err != nil {
					if errors.Is(err, errAuthPending) {
						return authPendingMessage, nil
					}
					return "", err
				}
			}
		}
	}

	// Write approval.
	if isWrite {
		var opts []GuardOption
		if g.approvalRules != nil {
			opts = append(opts, WithApprovalRules(g.approvalRules, "gws_execute", input))
		}
		return GuardedAction(ctx, g.interactor, RiskHigh, fmt.Sprintf("Run gws command: %s", in.Command), func() (string, error) {
			return g.run(ctx, args)
		}, opts...)
	}

	return g.run(ctx, args)
}

func (g *GWSExecuteTool) run(ctx context.Context, args []string) (string, error) {
	env, err := g.bridge.Env(ctx)
	if err != nil {
		slog.Warn("gws_execute: token error, attempting re-auth", "error", err)
		service := gwsServiceFromCommand(args)
		scopes := g.scopesForService(service)
		if len(scopes) == 0 {
			return "", fmt.Errorf("get token: %w", err)
		}
		if cerr := g.requestConsent(ctx, scopes); cerr != nil {
			if errors.Is(cerr, errAuthPending) {
				return authPendingMessage, nil
			}
			return "", fmt.Errorf("get token: %w (re-auth also failed: %v)", err, cerr)
		}
		env, err = g.bridge.Env(ctx)
		if err != nil {
			return "", fmt.Errorf("get token after re-auth: %w", err)
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, runErr := g.runner.Run(runCtx, args, env)

	// If Google returns insufficient scopes, the local DB is stale.
	// Trigger progressive consent and retry once.
	if runErr != nil && strings.Contains(out, "ACCESS_TOKEN_SCOPE_INSUFFICIENT") {
		slog.Warn("gws_execute: scope insufficient, triggering re-auth")
		service := gwsServiceFromCommand(args)
		scopes := g.scopesForService(service)
		if len(scopes) > 0 {
			if cerr := g.requestConsent(ctx, scopes); cerr != nil {
				if errors.Is(cerr, errAuthPending) {
					return authPendingMessage, nil
				}
			} else {
				env, err = g.bridge.Env(ctx)
				if err == nil {
					retryCtx, retryCancel := context.WithTimeout(ctx, 30*time.Second)
					defer retryCancel()
					out, runErr = g.runner.Run(retryCtx, args, env)
				}
			}
		}
	}

	// If the API is disabled, flag it so the agent can extract the
	// enablement URL from the error and share it with the user.
	if runErr != nil && (strings.Contains(out, "SERVICE_DISABLED") || strings.Contains(out, "API has not been used")) {
		slog.Warn("gws_execute: API disabled for service", "service", gwsServiceFromCommand(args))
		out = out + "\n\nNote: The Google API for this service is not enabled. The error above contains an activation URL — share it with the user so they can enable the API."
	}

	out = TruncateHeadTail(out, MaxLinesHeadTail, MaxLinesHeadTail)
	out = TruncateBytes(out, MaxOutputBytes)
	return out, runErr
}

func (g *GWSExecuteTool) requestConsent(_ context.Context, scopes []string) error {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Errorf("generate state: %w", err)
	}
	state := "gws-" + hex.EncodeToString(b[:])
	url, err := g.google.AuthURL(g.account, scopes, state)
	if err != nil {
		return fmt.Errorf("generate auth URL: %w", err)
	}
	if g.authRedirectURL != "" {
		url = g.authRedirectURL + "?url=" + neturl.QueryEscape(url)
	}
	g.scopeWaiter.Register(state, scopes, g.account)

	// Clean up the waiter entry in the background (drains channel or times out).
	go g.scopeWaiter.Await(state, g.authTimeout)

	if err := g.interactor.Notify("I need additional Google access to complete this request."); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	if err := g.interactor.NotifyLink("Tap to grant access", url); err != nil {
		return fmt.Errorf("notify link: %w", err)
	}

	return errAuthPending
}

func (g *GWSExecuteTool) isWriteCommand(args []string) bool {
	if g.manifest == nil {
		return false
	}
	// Build the expected skill name from the command args.
	// e.g. ["calendar", "+agenda"] → "gws-calendar-agenda"
	service := gwsServiceFromCommand(args)
	for _, arg := range args {
		if sub, ok := strings.CutPrefix(arg, "+"); ok {
			skillName := "gws-" + service + "-" + sub
			if entry, ok := g.manifest.Skills[skillName]; ok {
				return entry.Write
			}
			return true // unknown + command, assume write to be safe
		}
	}
	return false
}

// serviceToScope maps gws short service names to full Google API scope URLs.
var serviceToScope = map[string]string{
	"calendar": "https://www.googleapis.com/auth/calendar",
	"drive":    "https://www.googleapis.com/auth/drive",
	"docs":     "https://www.googleapis.com/auth/documents",
	"sheets":   "https://www.googleapis.com/auth/spreadsheets",
	"tasks":    "https://www.googleapis.com/auth/tasks",
	"people":   "https://www.googleapis.com/auth/contacts",
	"gmail":    "https://www.googleapis.com/auth/gmail.modify",
}

func (g *GWSExecuteTool) scopesForService(service string) []string {
	// Check manifest for GWS skills first.
	if g.manifest != nil {
		for _, entry := range g.manifest.Skills {
			if entry.Source == "gws" && len(entry.Scopes) > 0 && entry.Scopes[0] == service {
				if scope, ok := serviceToScope[service]; ok {
					return []string{scope}
				}
			}
		}
	}
	// Fall back to the scope map directly so progressive consent works
	// even for services without a GWS-generated skill (e.g. gmail).
	if scope, ok := serviceToScope[service]; ok {
		return []string{scope}
	}
	return nil
}

// splitCommand splits a command string on whitespace, respecting single and
// double quotes so that quoted values stay as a single argument.
func splitCommand(s string) []string {
	if !strings.ContainsAny(s, `"'`) {
		return strings.Fields(s)
	}
	var args []string
	var current strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
			current.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
		case c == ' ' || c == '\t':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// isHelperCommand returns true if any arg starts with "+".
func isHelperCommand(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") {
			return true
		}
	}
	return false
}

// expandBodyAsFlags converts a JSON object into --key value flag pairs.
// Arrays become repeated flags, booleans become bare flags (true) or are
// omitted (false), and everything else is stringified.
func expandBodyAsFlags(body json.RawMessage) ([]string, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("body must be a JSON object: %w", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var flags []string
	for _, k := range keys {
		flag := "--" + k
		switch val := m[k].(type) {
		case []any:
			for _, item := range val {
				flags = append(flags, flag, fmt.Sprint(item))
			}
		case bool:
			if val {
				flags = append(flags, flag)
			}
		default:
			flags = append(flags, flag, fmt.Sprint(m[k]))
		}
	}
	return flags, nil
}

// gwsServiceFromCommand extracts the service name from gws command args.
func gwsServiceFromCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

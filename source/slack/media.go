package slack

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// filePermalinkRe matches the file ID in /files/U02ABC/F0BLMEDEJCS/name.png.
var filePermalinkRe = regexp.MustCompile(`/files/[^/]+/(F[A-Z0-9]+)`)

// ResolveFileURL turns a Slack file reference into a URL that serves the file
// bytes. A files.slack.com url_private is already one; a /files/… permalink is
// an HTML page, so its file ID is looked up via files.info.
func ResolveFileURL(ctx context.Context, api API, raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("not a Slack file URL: %q (want a url_private or a /files/… permalink)", raw)
	}

	host := strings.ToLower(u.Hostname())
	if host == "files.slack.com" {
		return raw, nil
	}
	if !strings.HasSuffix(host, ".slack.com") {
		return "", fmt.Errorf("not a Slack file URL: %q (want a url_private or a /files/… permalink)", raw)
	}

	m := filePermalinkRe.FindStringSubmatch(u.Path)
	if m == nil {
		return "", fmt.Errorf("no file ID in %q (want /files/<user>/<file-id>/<name>)", raw)
	}

	f, err := api.FilesInfo(ctx, m[1])
	if err != nil {
		return "", fmt.Errorf("file info %s: %w", m[1], err)
	}
	if f.URLPrivate == "" {
		return "", fmt.Errorf("file %s has no downloadable URL", m[1])
	}
	return f.URLPrivate, nil
}

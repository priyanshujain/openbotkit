package websearch

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const bingURL = "https://www.bing.com/search"

type Bing struct {
	client  HTTPDoer
	baseURL string
}

func NewBing(client HTTPDoer) *Bing {
	return &Bing{client: client, baseURL: bingURL}
}

func (b *Bing) Name() string  { return "bing" }
func (b *Bing) Priority() int { return 0 }

func (b *Bing) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	page := opts.Page
	if page <= 1 {
		page = 1
	}
	offset := (page - 1) * 10

	u, err := url.Parse(b.baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	if offset > 0 {
		q.Set("first", fmt.Sprintf("%d", offset+1))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Engine: "bing", Code: resp.StatusCode}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bing: parse html: %w", err)
	}

	var results []Result
	doc.Find("li.b_algo").Each(func(i int, s *goquery.Selection) {
		if opts.MaxResults > 0 && len(results) >= opts.MaxResults {
			return
		}

		link := s.Find("h2 a")
		title := strings.TrimSpace(link.Text())
		href, _ := link.Attr("href")
		snippet := strings.TrimSpace(s.Find("p").Text())

		if title == "" || href == "" {
			return
		}

		href = unwrapBingURL(href)
		if isBingAdURL(href) {
			return
		}

		results = append(results, Result{
			Title:   title,
			URL:     href,
			Snippet: snippet,
			Source:  "bing",
		})
	})

	return results, nil
}

func unwrapBingURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if !strings.Contains(u.Path, "/ck/a") {
		return raw
	}
	encoded := u.Query().Get("u")
	if encoded == "" {
		return raw
	}
	if len(encoded) < 3 {
		return raw
	}
	// Strip first 2 chars ("a1" prefix), then base64url decode.
	decoded, err := base64.RawURLEncoding.DecodeString(encoded[2:])
	if err != nil {
		return raw
	}
	return string(decoded)
}

func isBingAdURL(href string) bool {
	return strings.Contains(href, "/aclick?")
}

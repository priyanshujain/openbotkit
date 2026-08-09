package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"github.com/73ai/openbotkit/internal/browser"
)

const (
	defaultCookieURL = "https://fc.yahoo.com"
	defaultCrumbURL  = "https://query2.finance.yahoo.com/v1/test/getcrumb"
	defaultQuoteURL  = "https://query1.finance.yahoo.com/v7/finance/quote"
	userAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

type Client struct {
	mu        sync.Mutex
	http      *http.Client
	crumb     string
	inited    bool
	cookieURL string
	crumbURL  string
	quoteURL  string
}

func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		http: &http.Client{
			Transport: browser.NewChromeTransport(),
			Jar:       jar,
		},
		cookieURL: defaultCookieURL,
		crumbURL:  defaultCrumbURL,
		quoteURL:  defaultQuoteURL,
	}
}

func (c *Client) Quote(ctx context.Context, symbols ...string) ([]Quote, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.inited {
		if err := c.initSession(ctx); err != nil {
			return nil, fmt.Errorf("init session: %w", err)
		}
	}

	quotes, err := c.fetchQuotes(ctx, symbols)
	if errors.Is(err, errUnauthorized) {
		if err := c.initSession(ctx); err != nil {
			return nil, fmt.Errorf("refresh session: %w", err)
		}
		quotes, err = c.fetchQuotes(ctx, symbols)
	}
	return quotes, err
}

var errUnauthorized = errors.New("unauthorized")

func (c *Client) initSession(ctx context.Context) error {
	// Step 1: Get cookies from fc.yahoo.com (returns 404 body but sets A3 cookie).
	req, err := http.NewRequestWithContext(ctx, "GET", c.cookieURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch cookies: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// Step 2: Get crumb.
	req, err = http.NewRequestWithContext(ctx, "GET", c.crumbURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err = c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch crumb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("crumb endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read crumb: %w", err)
	}

	c.crumb = strings.TrimSpace(string(body))
	if c.crumb == "" {
		return fmt.Errorf("empty crumb received")
	}
	c.inited = true
	return nil
}

func (c *Client) fetchQuotes(ctx context.Context, symbols []string) ([]Quote, error) {
	u, err := url.Parse(c.quoteURL)
	if err != nil {
		return nil, fmt.Errorf("parse quote URL: %w", err)
	}
	q := u.Query()
	q.Set("symbols", strings.Join(symbols, ","))
	q.Set("crumb", c.crumb)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quote endpoint returned %d", resp.StatusCode)
	}

	var result struct {
		QuoteResponse struct {
			Result []Quote `json:"result"`
			Error  *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"quoteResponse"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if e := result.QuoteResponse.Error; e != nil {
		return nil, fmt.Errorf("yahoo api error: %s: %s", e.Code, e.Description)
	}

	return result.QuoteResponse.Result, nil
}

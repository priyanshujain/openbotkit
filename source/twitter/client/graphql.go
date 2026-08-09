package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	graphqlBase = "https://x.com/i/api/graphql"
	// Public bearer token used by Twitter's web client. Not a secret —
	// every reverse-engineered Twitter library uses the same value.
	bearerToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"
)

func (c *Client) buildHeaders() http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+bearerToken)
	h.Set("X-Csrf-Token", c.session.CSRFToken)
	h.Set("X-Twitter-Active-User", "yes")
	h.Set("X-Twitter-Auth-Type", "OAuth2Session")
	h.Set("X-Twitter-Client-Language", "en")
	h.Set("Cookie", fmt.Sprintf("auth_token=%s; ct0=%s", c.session.AuthToken, c.session.CSRFToken))
	h.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "*/*")
	h.Set("Referer", "https://x.com/")
	h.Set("Origin", "https://x.com")
	return h
}

func (c *Client) graphqlGET(ctx context.Context, opName string, variables, features map[string]any) (json.RawMessage, error) {
	ep, ok := c.endpoints[opName]
	if !ok {
		return nil, fmt.Errorf("unknown operation: %s", opName)
	}

	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}
	featJSON, err := json.Marshal(features)
	if err != nil {
		return nil, fmt.Errorf("marshal features: %w", err)
	}

	u, err := url.Parse(fmt.Sprintf("%s/%s/%s", c.graphqlBaseURL(), ep.QueryID, opName))
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("variables", string(varsJSON))
	q.Set("features", string(featJSON))
	u.RawQuery = q.Encode()

	return c.doRequest(ctx, http.MethodGet, u.String(), nil)
}

func (c *Client) graphqlPOST(ctx context.Context, opName string, variables, features map[string]any) (json.RawMessage, error) {
	ep, ok := c.endpoints[opName]
	if !ok {
		return nil, fmt.Errorf("unknown operation: %s", opName)
	}

	body := map[string]any{
		"variables": variables,
		"features":  features,
		"queryId":   ep.QueryID,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	u := fmt.Sprintf("%s/%s/%s", c.graphqlBaseURL(), ep.QueryID, opName)
	return c.doRequest(ctx, http.MethodPost, u, bytes.NewReader(bodyJSON))
}

func (c *Client) doRequest(ctx context.Context, method, url string, body io.Reader) (json.RawMessage, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	headers := c.buildHeaders()
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("X-Client-Transaction-Id", GenerateTransactionID(method, url))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter API error (status %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	return json.RawMessage(respBody), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

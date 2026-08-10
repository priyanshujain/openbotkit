package provider

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ErrorKind int

const (
	ErrorPermanent     ErrorKind = iota // everything else (zero value = safe default)
	ErrorRetryable                      // 429, 5xx
	ErrorAuth                           // 401, 403
	ErrorContextWindow                  // 400 + "context" in message
)

type APIError struct {
	StatusCode int
	Kind       ErrorKind
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Message)
}

var httpStatusPattern = regexp.MustCompile(`HTTP (\d{3})`)

func ClassifyError(err error) *APIError {
	if err == nil {
		return nil
	}

	msg := err.Error()
	apiErr := &APIError{Message: msg}

	if m := httpStatusPattern.FindStringSubmatch(msg); len(m) == 2 {
		apiErr.StatusCode, _ = strconv.Atoi(m[1])
	}

	switch {
	case apiErr.StatusCode == 429 || apiErr.StatusCode >= 500:
		apiErr.Kind = ErrorRetryable
	case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
		apiErr.Kind = ErrorAuth
	case apiErr.StatusCode == 400 && strings.Contains(strings.ToLower(msg), "context"):
		apiErr.Kind = ErrorContextWindow
	default:
		apiErr.Kind = ErrorPermanent
	}

	return apiErr
}

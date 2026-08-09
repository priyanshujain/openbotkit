package websearch

import (
	"errors"
	"fmt"
)

// FailureKind classifies search engine errors for the health tracker.
type FailureKind int

const (
	FailureTransient    FailureKind = iota // timeout, 5xx, connection error
	FailureRateLimit                       // 429, DDG 202
	FailureAccessDenied                    // 403
)

// StatusError is returned by engines when the HTTP response status is not OK.
type StatusError struct {
	Engine string
	Code   int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s returned status %d", e.Engine, e.Code)
}

func classifyError(err error) FailureKind {
	var se *StatusError
	if errors.As(err, &se) {
		switch {
		case se.Code == 429:
			return FailureRateLimit
		case se.Code == 202 && se.Engine == "duckduckgo":
			return FailureRateLimit
		case se.Code == 403:
			return FailureAccessDenied
		}
	}
	return FailureTransient
}

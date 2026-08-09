package websearch

import (
	"fmt"
	"testing"
)

func TestClassifyError_StatusCodes(t *testing.T) {
	tests := []struct {
		err  error
		want FailureKind
	}{
		{&StatusError{Engine: "brave", Code: 429}, FailureRateLimit},
		{&StatusError{Engine: "duckduckgo", Code: 202}, FailureRateLimit},
		{&StatusError{Engine: "brave", Code: 202}, FailureTransient}, // 202 is DDG-only
		{&StatusError{Engine: "mojeek", Code: 403}, FailureAccessDenied},
		{&StatusError{Engine: "google", Code: 500}, FailureTransient},
		{&StatusError{Engine: "bing", Code: 503}, FailureTransient},
		{fmt.Errorf("connection refused"), FailureTransient},
	}
	for _, tt := range tests {
		got := classifyError(tt.err)
		if got != tt.want {
			t.Errorf("classifyError(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
}

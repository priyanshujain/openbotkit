package auth

import (
	"testing"
)

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: nil},
		{name: "single", input: "gmail.readonly", want: []string{"gmail.readonly"}},
		{name: "multiple", input: "gmail.readonly,calendar.readonly", want: []string{"gmail.readonly", "calendar.readonly"}},
		{name: "with spaces", input: " gmail.readonly , calendar.readonly ", want: []string{"gmail.readonly", "calendar.readonly"}},
		{name: "trailing comma", input: "gmail.readonly,", want: []string{"gmail.readonly"}},
		{name: "full url", input: "https://www.googleapis.com/auth/gmail.readonly", want: []string{"https://www.googleapis.com/auth/gmail.readonly"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseScopes(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExpandScopes(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "aliases expanded",
			input: []string{"gmail.readonly", "calendar.readonly"},
			want: []string{
				"https://www.googleapis.com/auth/gmail.readonly",
				"https://www.googleapis.com/auth/calendar.readonly",
			},
		},
		{
			name:  "full url passed through",
			input: []string{"https://www.googleapis.com/auth/gmail.readonly"},
			want:  []string{"https://www.googleapis.com/auth/gmail.readonly"},
		},
		{
			name:  "unknown alias passed through",
			input: []string{"drive.readonly"},
			want:  []string{"drive.readonly"},
		},
		{
			name:  "empty",
			input: []string{},
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandScopes(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestScopeLabel(t *testing.T) {
	tests := []struct {
		scope string
		want  string
	}{
		{"https://www.googleapis.com/auth/gmail.readonly", "Gmail (read)"},
		{"https://www.googleapis.com/auth/calendar.readonly", "Calendar (read)"},
		{"https://www.googleapis.com/auth/unknown", "https://www.googleapis.com/auth/unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			got := scopeLabel(tt.scope)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

package gmail

import "testing"

func TestBuildQuery(t *testing.T) {
	tests := []struct {
		name  string
		query FetchQuery
		want  string
	}{
		{
			name:  "raw query takes precedence",
			query: FetchQuery{Query: "is:unread", From: "ignored", After: "ignored"},
			want:  "is:unread",
		},
		{
			name:  "from only",
			query: FetchQuery{From: "alice@example.com"},
			want:  "from:alice@example.com",
		},
		{
			name:  "after only",
			query: FetchQuery{After: "2025/01/01"},
			want:  "after:2025/01/01",
		},
		{
			name:  "before only",
			query: FetchQuery{Before: "2025/06/01"},
			want:  "before:2025/06/01",
		},
		{
			name:  "from and after",
			query: FetchQuery{From: "bob@test.com", After: "2025/03/01"},
			want:  "from:bob@test.com after:2025/03/01",
		},
		{
			name:  "after and before",
			query: FetchQuery{After: "2025/01/01", Before: "2025/06/01"},
			want:  "after:2025/01/01 before:2025/06/01",
		},
		{
			name:  "all fields",
			query: FetchQuery{From: "alice@test.com", After: "2025/01/01", Before: "2025/12/31"},
			want:  "from:alice@test.com after:2025/01/01 before:2025/12/31",
		},
		{
			name:  "empty query",
			query: FetchQuery{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildQuery(tt.query)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

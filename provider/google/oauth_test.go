package google

import "testing"

func TestMergeScopes(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []string
	}{
		{
			name: "no overlap",
			a:    []string{"a", "b"},
			b:    []string{"c", "d"},
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "with overlap",
			a:    []string{"a", "b", "c"},
			b:    []string{"b", "c", "d"},
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "empty a",
			a:    nil,
			b:    []string{"x"},
			want: []string{"x"},
		},
		{
			name: "empty b",
			a:    []string{"x"},
			b:    nil,
			want: []string{"x"},
		},
		{
			name: "both empty",
			a:    nil,
			b:    nil,
			want: nil,
		},
		{
			name: "duplicates within a",
			a:    []string{"a", "a", "b"},
			b:    []string{"b"},
			want: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeScopes(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d (%v vs %v)", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestImplicitScopesIncluded(t *testing.T) {
	// loadConfig should always include openid + email.
	// We can't call loadConfig without a real file, but we can verify the constant.
	if len(implicitScopes) != 2 {
		t.Fatalf("expected 2 implicit scopes, got %d", len(implicitScopes))
	}
	found := map[string]bool{}
	for _, s := range implicitScopes {
		found[s] = true
	}
	if !found["openid"] || !found["email"] {
		t.Errorf("implicit scopes should contain openid and email, got %v", implicitScopes)
	}
}

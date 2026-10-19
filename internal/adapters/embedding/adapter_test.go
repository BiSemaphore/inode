package embedding

import "testing"

func TestDimension(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  int
		ok    bool
	}{
		{"nomic-embed-text", 768, true},
		{"nomic-embed-text:latest", 768, true},
		{"voyage-3", 1024, true},
		{"nomic-embed-text:v1.5", 0, false},
		{"mxbai-embed-large", 0, false},
	} {
		got, ok := Dimension(tc.model)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Dimension(%q) = %d, %v; want %d, %v", tc.model, got, ok, tc.want, tc.ok)
		}
	}
}

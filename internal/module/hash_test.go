package module_test

import (
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

func TestHash(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"empty", "", "e3b0c4"},
		{"hello", "hello\n", "5891b5"},
		{"heading", "## Working rules\n", "d43188"},
		{"trailing blanks are significant", "a\n\n\n", "ca8b17"},
		{"a byte matters", "body\n", "9e2ec9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := module.Hash(tt.body); got != tt.want {
				t.Errorf("Hash(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestHashLength(t *testing.T) {
	if got := module.Hash("anything"); len(got) != 6 {
		t.Errorf("Hash returned %q (len %d), want 6 hex characters", got, len(got))
	}
}

// TestEmbeddedModuleHashes pins the example modules the binary ships. They are
// what a fresh install renders and what the format documents itself with, so
// editing one changes every region marker it has already written: this test
// fails on purpose, and re-pinning it is how that edit is made deliberate.
func TestEmbeddedModuleHashes(t *testing.T) {
	set, err := module.Load(embeddedModules(t))
	if err != nil {
		t.Fatalf("Load(embedded): %v", err)
	}
	want := map[string]string{
		"agents":           "556395",
		"module-authoring": "503f5d",
	}
	for name, wantHash := range want {
		m, ok := set.Get(name)
		if !ok {
			t.Errorf("module %q missing from the embedded set", name)
			continue
		}
		if m.Hash != wantHash {
			t.Errorf("module %q hash = %q, want %q", name, m.Hash, wantHash)
		}
	}
}

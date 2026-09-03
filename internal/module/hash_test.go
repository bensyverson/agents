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

// TestEmbeddedModuleHashes is the dogfood contract with the rendered AGENTS.md
// in this repo: these are the hashes in its region markers. Editing a module
// changes its hash and fails this test on purpose — that is the signal to
// re-render AGENTS.md (`agents sync` after `go install`) and update the table.
func TestEmbeddedModuleHashes(t *testing.T) {
	set, err := module.Load(agentsModules(t))
	if err != nil {
		t.Fatalf("Load(embedded): %v", err)
	}
	want := map[string]string{
		"core":        "3a7a5e",
		"principles":  "7a5b19",
		"stage-build": "3d5d83",
		"go":          "91ab6a",
		"delegation":  "722742",
		"evidence":    "2393e6",
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

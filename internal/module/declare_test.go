package module_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

// Ignore lines and pre-commit commands are carried verbatim: the tool applies
// them without knowing what they mean.
func TestParseIgnoreAndPreCommit(t *testing.T) {
	raw := "---\nignore: [\".scratch/\", \"!keep.log\"]\npre-commit:\n  - fmt --check\n  - test -z \"$(lint)\"\n---\n## Rules\n"
	m, err := module.Parse("stack", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := []string{".scratch/", "!keep.log"}; !reflect.DeepEqual(m.Ignore, want) {
		t.Errorf("Ignore = %v, want %v", m.Ignore, want)
	}
	if want := []string{"fmt --check", `test -z "$(lint)"`}; !reflect.DeepEqual(m.PreCommit, want) {
		t.Errorf("PreCommit = %v, want %v", m.PreCommit, want)
	}
	if want := module.Hash("## Rules\n"); m.Hash != want {
		t.Errorf("Hash = %q, want %q — declarations must not change the body hash", m.Hash, want)
	}
}

// Surrounding whitespace is YAML noise, not part of the line.
func TestParseTrimsDeclarations(t *testing.T) {
	m, err := module.Parse("x", "---\nkind: file\nignore: [\"  out/  \"]\npre-commit: [\"  make lint \"]\n---\nBody.\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := []string{"out/"}; !reflect.DeepEqual(m.Ignore, want) {
		t.Errorf("Ignore = %v, want %v", m.Ignore, want)
	}
	if want := []string{"make lint"}; !reflect.DeepEqual(m.PreCommit, want) {
		t.Errorf("PreCommit = %v, want %v", m.PreCommit, want)
	}
}

func TestParseDeclarationErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{"empty ignore line", "---\nignore: [\"\"]\n---\nBody.\n", module.ErrIgnoreInvalid},
		{"blank ignore line", "---\nignore: [\"   \"]\n---\nBody.\n", module.ErrIgnoreInvalid},
		{"multi-line ignore entry", "---\nignore: [\"a\\nb\"]\n---\nBody.\n", module.ErrIgnoreInvalid},
		{"comment as an ignore line", "---\nignore: [\"# note\"]\n---\nBody.\n", module.ErrIgnoreInvalid},
		{"empty pre-commit command", "---\npre-commit: [\"\"]\n---\nBody.\n", module.ErrPreCommitInvalid},
		{"comment as a pre-commit command", "---\npre-commit: [\"# agents:end pre-commit\"]\n---\nBody.\n", module.ErrPreCommitInvalid},
		{"multi-line pre-commit command", "---\npre-commit: [\"a\\nb\"]\n---\nBody.\n", module.ErrPreCommitInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := module.Parse("x", tt.raw)
			if !errors.Is(err, tt.want) {
				t.Errorf("Parse(%q) error = %v, want %v", tt.raw, err, tt.want)
			}
		})
	}
}

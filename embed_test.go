package agents

import (
	"io/fs"
	"testing"

	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/source"
)

// The binary carries one source, and a source is a directory with modules/ and
// templates/ at its root: Embedded must open through the ordinary loader, not
// through a shape only the binary has.
func TestEmbeddedIsASourceRoot(t *testing.T) {
	src, err := source.FromFS("embedded", Embedded())
	if err != nil {
		t.Fatalf("source.FromFS(Embedded()): %v", err)
	}
	set, err := module.Load(src.Modules)
	if err != nil {
		t.Fatalf("module.Load: %v", err)
	}
	if len(set.Names()) == 0 {
		t.Error("the embedded source holds no modules")
	}
	for _, name := range set.Names() {
		m, _ := set.Get(name)
		for _, seed := range m.Seeds {
			if _, err := fs.ReadFile(src.Templates, seed); err != nil {
				t.Errorf("module %s seeds %s, which the embedded templates lack: %v", name, seed, err)
			}
		}
	}
}

// Modules() and Templates() are the same tree seen from inside; a caller that
// wants one subtree must not get a different set of files from the source.
func TestEmbeddedSubtreesMatchTheSource(t *testing.T) {
	src, err := source.FromFS("embedded", Embedded())
	if err != nil {
		t.Fatalf("source.FromFS(Embedded()): %v", err)
	}
	for _, pair := range []struct {
		what string
		a, b fs.FS
	}{
		{"modules", Modules(), src.Modules},
		{"templates", Templates(), src.Templates},
	} {
		want := names(t, pair.a)
		got := names(t, pair.b)
		if len(want) == 0 {
			t.Errorf("%s is empty", pair.what)
		}
		if len(got) != len(want) {
			t.Errorf("%s: source holds %v, %s() holds %v", pair.what, got, pair.what, want)
		}
	}
}

func names(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	var out []string
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out = append(out, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walking: %v", err)
	}
	return out
}

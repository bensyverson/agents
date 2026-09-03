package repo

import (
	"path/filepath"
	"testing"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/render"
)

// TestDogfoodThisRepo runs Init and Sync over a copy of this repo's own inputs
// with the embedded modules: the tool must reproduce the hand-rendered
// AGENTS.md byte-for-byte. If the module review has moved a body since that
// hand render the rewrite is legitimate, and what must hold instead is that a
// second sync is a no-op.
func TestDogfoodThisRepo(t *testing.T) {
	const root = "../.."
	dir := t.TempDir()
	original := readFile(t, filepath.Join(root, AgentsFile))
	writeFixture(t, dir, AgentsFile, original)
	writeFixture(t, dir, manifest.FileName, readFile(t, filepath.Join(root, manifest.FileName)))

	set, err := module.Load(agents.Modules())
	if err != nil {
		t.Fatalf("module.Load(embedded): %v", err)
	}
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		t.Fatalf("manifest.Read: %v", err)
	}
	// A kind:file module renders to its own file; the fixture needs those
	// too, or the render would legitimately write them.
	for _, ref := range m.Modules {
		if mod, ok := set.Get(ref.Name); ok && mod.Kind == module.KindFile {
			writeFixture(t, dir, mod.Path, readFile(t, filepath.Join(root, mod.Path)))
		}
	}

	res, err := Init(dir, InitOptions{
		Modules:      m.Entries(),
		Open:         Options{Embedded: agents.Embedded()},
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	rendered := readFixture(t, dir, AgentsFile)
	if rendered == original {
		if len(res.Sync.Written) != 0 {
			t.Errorf("Sync.Written = %v, want nothing to do on an already-rendered file", res.Sync.Written)
		}
		return
	}

	t.Logf("the embedded modules no longer match the hand-rendered AGENTS.md; checking idempotence instead:\n%s",
		render.Diff(original, rendered))
	r, err := Open(dir, Options{Embedded: agents.Embedded()})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	again, err := r.Sync(false)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if len(again.Written) != 0 {
		t.Errorf("second Sync wrote %v, want a no-op", again.Written)
	}
	if got := readFixture(t, dir, AgentsFile); got != rendered {
		t.Errorf("second Sync changed the file:\n%s", render.Diff(rendered, got))
	}
}

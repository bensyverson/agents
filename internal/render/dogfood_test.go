package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// TestDogfoodAgentsMD renders this repo's own hand-written AGENTS.md from the
// modules on disk: the tool's output must be byte-identical to what a human
// produced, and every region must inspect as fresh.
func TestDogfoodAgentsMD(t *testing.T) {
	const root = "../.."
	doc := readRepoFile(t, filepath.Join(root, "AGENTS.md"))

	// The inline modules this repo enables, from its own manifest — so enabling
	// a module here never needs a test edit.
	m, err := manifest.Read(filepath.Join(root, manifest.FileName))
	if err != nil {
		t.Fatalf("manifest.Read: %v", err)
	}
	set, err := module.LoadDir(filepath.Join(root, "modules"))
	if err != nil {
		t.Fatalf("module.LoadDir: %v", err)
	}
	var mods []module.Module
	for _, name := range m.Modules {
		mod, ok := set.Get(name)
		if !ok {
			t.Fatalf("manifest names %q, which modules/ lacks", name)
		}
		if mod.Kind == module.KindInline {
			mods = append(mods, mod)
		}
	}

	got := mustRender(t, doc, mods)
	if got != doc {
		t.Errorf("Render(AGENTS.md) is not byte-identical to AGENTS.md:\n%s", Diff(doc, got))
	}

	rep := mustInspect(t, doc, mods)
	if len(rep.Regions) != len(mods) {
		t.Fatalf("want %d regions, got %d", len(mods), len(rep.Regions))
	}
	for i, r := range rep.Regions {
		if r.Name != mods[i].Name {
			t.Errorf("region %d: got %q want %q", i, r.Name, mods[i].Name)
		}
		if !r.Fresh() {
			t.Errorf("region %q is not fresh: marker=%s body=%s module=%s (stale=%v edited=%v)",
				r.Name, r.MarkerHash, r.BodyHash, r.ModuleHash, r.Stale, r.Edited)
		}
	}
	if len(rep.Missing) != 0 {
		t.Errorf("Missing: %v", rep.Missing)
	}
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

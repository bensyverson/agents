package repo

import (
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

func TestListSortsAndMarksEnabledModules(t *testing.T) {
	got := List(testSet(t), []string{"principles", "core"})

	var names []string
	byName := map[string]ModuleInfo{}
	for _, info := range got {
		names = append(names, info.Name)
		byName[info.Name] = info
	}
	want := []string{"core", "delegation", "docs", "principles", "stage-build"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("names = %v, want %v (sorted)", names, want)
		}
	}
	for _, name := range []string{"core", "principles"} {
		if !byName[name].Enabled {
			t.Errorf("%s is not marked enabled", name)
		}
	}
	for _, name := range []string{"docs", "delegation", "stage-build"} {
		if byName[name].Enabled {
			t.Errorf("%s is marked enabled", name)
		}
	}
	if got := byName["delegation"].Path; got != delegationPath {
		t.Errorf("delegation path = %q, want %q", got, delegationPath)
	}
	if got := byName["core"].Path; got != "" {
		t.Errorf("an inline module has path %q, want empty", got)
	}
}

// Outside a managed repo there is nothing to mark, and that is not an error:
// `agents list` has to answer "what could I enable?" anywhere.
func TestListMarksNothingWithoutAManifest(t *testing.T) {
	for _, info := range List(testSet(t), nil) {
		if info.Enabled {
			t.Errorf("%s is marked enabled with no manifest", info.Name)
		}
	}
}

func TestEnabledModulesReadsTheManifest(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: manifestInline})

	got, err := EnabledModules(dir)
	if err != nil {
		t.Fatalf("EnabledModules: %v", err)
	}
	if len(got) != 2 || got[0] != "core" || got[1] != "principles" {
		t.Errorf("EnabledModules = %v, want [core principles]", got)
	}
}

func TestEnabledModulesOutsideAManagedRepo(t *testing.T) {
	got, err := EnabledModules(t.TempDir())
	if err != nil {
		t.Fatalf("EnabledModules on an unmanaged directory: %v", err)
	}
	if got != nil {
		t.Errorf("EnabledModules = %v, want nil", got)
	}
}

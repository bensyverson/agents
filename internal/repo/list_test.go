package repo

import (
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/manifest"
)

// listSources is the test source under the default name, plus a second source
// holding one module of its own.
func listSources(t *testing.T) Sources {
	t.Helper()
	sources, err := loadFS(map[string]fstest.MapFS{
		"house": testSourceFS(),
		"other": {"modules/extra.md": &fstest.MapFile{Data: []byte("extra rules\n")}},
	})
	if err != nil {
		t.Fatalf("loading sources: %v", err)
	}
	return sources
}

// loadFS builds Sources from in-memory source trees, keyed by source name.
func loadFS(trees map[string]fstest.MapFS) (Sources, error) {
	out := make(Sources, len(trees))
	for name, tree := range trees {
		s, err := LoadSource(name, tree)
		if err != nil {
			return nil, err
		}
		out[name] = s
	}
	return out, nil
}

func byName(infos []ModuleInfo) map[string]ModuleInfo {
	out := make(map[string]ModuleInfo, len(infos))
	for _, info := range infos {
		out[info.Name] = info
	}
	return out
}

func listNames(infos []ModuleInfo) []string {
	out := make([]string, len(infos))
	for i, info := range infos {
		out[i] = info.Name
	}
	return out
}

// A module of the default source lists under its bare name; every other module
// lists under the qualified name that would go in the manifest.
func TestListQualifiesEveryNonDefaultModule(t *testing.T) {
	enabled := []manifest.ModuleRef{{Source: "house", Name: "core"}, {Source: "other", Name: "extra"}}

	got := List(listSources(t), "house", enabled)

	want := []string{"alpha", "beta", "core", "delegation", "docs", "other/extra", "principles", "stage-build"}
	names := listNames(got)
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v (sorted)", names, want)
		}
	}

	infos := byName(got)
	for _, name := range []string{"core", "other/extra"} {
		if !infos[name].Enabled {
			t.Errorf("%s is not marked enabled", name)
		}
	}
	for _, name := range []string{"docs", "delegation", "stage-build", "alpha", "beta", "principles"} {
		if infos[name].Enabled {
			t.Errorf("%s is marked enabled", name)
		}
	}
	if got := infos["delegation"].Path; got != delegationPath {
		t.Errorf("delegation path = %q, want %q", got, delegationPath)
	}
	if got := infos["core"].Path; got != "" {
		t.Errorf("an inline module has path %q, want empty", got)
	}
}

// Outside a managed repo there is nothing to mark, and that is not an error:
// `agents list` has to answer "what could I enable?" anywhere.
func TestListMarksNothingWithoutAManifest(t *testing.T) {
	for _, info := range List(listSources(t), "house", nil) {
		if info.Enabled {
			t.Errorf("%s is marked enabled with no manifest", info.Name)
		}
	}
}

package repo

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/source"
)

// embeddedWithDefaults is the test source declaring its defaults in
// source.yaml, the way the binary's embedded source does.
func embeddedWithDefaults(defaults string) fstest.MapFS {
	fsys := testSourceFS()
	fsys[source.SourceFile] = &fstest.MapFile{Data: []byte("defaults: " + defaults + "\n")}
	return fsys
}

// A source and no module list is a refusal, not a guess: only a source that
// declares defaults in source.yaml can say which of its modules to enable.
func TestInitWithASourceThatDeclaresNoDefaultsRequiresAModuleList(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(dir, InitOptions{
		Source:       manifest.Source{Name: "local", Path: newPathSource(t)},
		Open:         testOptionsWith(t.TempDir(), FetchMissing),
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	})

	if !errors.Is(err, ErrModulesRequired) {
		t.Fatalf("Init error = %v, want ErrModulesRequired", err)
	}
	for _, want := range []string{"local", "--with", "source.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if fileExists(t, dir, manifest.FileName) {
		t.Errorf("Init wrote %s despite refusing", manifest.FileName)
	}
}

// With no module list, init enables the default source's declared defaults, in
// the order it declares them — here the embedded source's, since the tool names
// no module of its own.
func TestInitWithNoModulesUsesTheEmbeddedSourcesDefaults(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(dir, InitOptions{
		Open:         Options{Embedded: embeddedWithDefaults("[principles, core]")},
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got, want := readFixture(t, dir, manifest.FileName), "modules:\n  - principles\n  - core\n"; got != want {
		t.Errorf("%s:\n got %q\nwant %q", manifest.FileName, got, want)
	}
}

// An embedded source that declares nothing leaves init nothing to guess.
func TestInitWithNoModulesAndNoDefaultsIsRefused(t *testing.T) {
	_, err := Init(t.TempDir(), InitOptions{
		Open:         testOptions(),
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	})
	if !errors.Is(err, ErrModulesRequired) {
		t.Fatalf("Init error = %v, want ErrModulesRequired", err)
	}
}

// A git source's defaults come from the commit init pinned.
func TestInitWithAGitSourceUsesItsDefaultsAndPinsASha(t *testing.T) {
	hermeticGit(t)
	files := gitSourceFiles()
	files[source.SourceFile] = "defaults: [one]\n"
	fx := newGitSource(t, files)
	dir := t.TempDir()

	res, err := Init(dir, InitOptions{
		Source:       manifest.Source{Name: "house", Git: fx.url},
		Open:         testOptionsWith(t.TempDir(), FetchMissing),
		RegistryPath: filepath.Join(t.TempDir(), fixtureRegistry),
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if res.Source.Ref != fx.sha {
		t.Errorf("pinned %q, want %q", res.Source.Ref, fx.sha)
	}
	if want := []string{"one"}; !reflect.DeepEqual(res.Modules, want) {
		t.Errorf("Modules = %v, want %v", res.Modules, want)
	}
	if got, want := readFixture(t, dir, AgentsFile), fresh("one", gitModuleBody); !strings.Contains(got, want) {
		t.Errorf("%s does not hold the default module:\n%s", AgentsFile, got)
	}
}

// A path source's defaults are read from its directory.
func TestInitWithAPathSourceUsesItsDefaults(t *testing.T) {
	pathDir := newPathSource(t)
	writeFixture(t, pathDir, source.SourceFile, "defaults: [two]\n")
	dir := t.TempDir()

	if _, err := Init(dir, InitOptions{
		Source:       manifest.Source{Name: "local", Path: pathDir},
		Open:         testOptionsWith(t.TempDir(), FetchMissing),
		RegistryPath: filepath.Join(t.TempDir(), fixtureRegistry),
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got, want := readFixture(t, dir, manifest.FileName), "modules:\n  - two\n"; !strings.HasSuffix(got, want) {
		t.Errorf("%s = %q, want it to end %q", manifest.FileName, got, want)
	}
}

// A default the source does not hold is the source's mistake, named as such.
func TestInitWithADefaultTheSourceLacksNamesIt(t *testing.T) {
	_, err := Init(t.TempDir(), InitOptions{
		Open:         Options{Embedded: embeddedWithDefaults("[core, nope]")},
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("Init error = %v, want it to name nope", err)
	}
}

// Re-running a bare init on a repo whose manifest a human curated is a no-op,
// not a conflict with defaults the human never asked for.
func TestInitWithNoModulesKeepsAnExistingManifest(t *testing.T) {
	existing := "modules:\n  - core\n"
	dir := newRepo(t, map[string]string{manifest.FileName: existing})

	if _, err := Init(dir, InitOptions{
		Open:         Options{Embedded: embeddedWithDefaults("[principles, core]")},
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := readFixture(t, dir, manifest.FileName); got != existing {
		t.Errorf("manifest = %q, want it untouched", got)
	}
}

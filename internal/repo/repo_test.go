package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

const (
	manifestAll    = "modules:\n  - core\n  - principles\n  - delegation\n"
	manifestInline = "modules:\n  - core\n  - principles\n"
	manifestFile   = "modules:\n  - delegation\n"
)

func TestOpenErrors(t *testing.T) {
	set := testSet(t)

	t.Run("missing directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nope")
		_, err := Open(dir, set)
		if !errors.Is(err, ErrMissingDir) {
			t.Fatalf("got %v, want ErrMissingDir", err)
		}
	})

	t.Run("no manifest", func(t *testing.T) {
		_, err := Open(t.TempDir(), set)
		if !errors.Is(err, ErrNotManaged) {
			t.Fatalf("got %v, want ErrNotManaged", err)
		}
	})

	t.Run("unknown module", func(t *testing.T) {
		dir := newRepo(t, map[string]string{manifest.FileName: "modules:\n  - core\n  - nope\n  - alsonope\n"})
		_, err := Open(dir, set)
		var unknown *manifest.UnknownModulesError
		if !errors.As(err, &unknown) {
			t.Fatalf("got %v, want *manifest.UnknownModulesError", err)
		}
		if len(unknown.Refs) != 2 {
			t.Errorf("Names = %v, want both unknown names", unknown.Refs)
		}
	})

	t.Run("malformed manifest", func(t *testing.T) {
		dir := newRepo(t, map[string]string{manifest.FileName: "modules: [\n"})
		if _, err := Open(dir, set); err == nil {
			t.Fatal("want error on malformed manifest, got nil")
		}
	})
}

func TestOpen(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: manifestAll})
	r := mustOpen(t, dir)
	if r.Dir != dir {
		t.Errorf("Dir = %q, want %q", r.Dir, dir)
	}
	want := []string{"core", "principles", "delegation"}
	if len(r.Manifest.Modules) != len(want) {
		t.Fatalf("Modules = %v, want %v", r.Manifest.Modules, want)
	}
	for i, name := range want {
		if r.Manifest.Modules[i].Name != name {
			t.Errorf("Modules[%d] = %q, want %q", i, r.Manifest.Modules[i], name)
		}
	}
}

func TestTargetsOnFreshRepo(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		"AGENTS.md":       "# Head\n\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		delegationPath:    fresh("delegation", delegationBody),
	})
	targets, err := mustOpen(t, dir).Targets()
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %v, want AGENTS.md and %s", targetPaths(targets), delegationPath)
	}
	if targets[0].Path != AgentsFile {
		t.Errorf("targets[0].Path = %q, want %q", targets[0].Path, AgentsFile)
	}
	for _, tg := range targets {
		if !tg.Exists {
			t.Errorf("%s: Exists = false", tg.Path)
		}
		if tg.NeedsWrite() {
			t.Errorf("%s: NeedsWrite = true on a fresh repo; rendered:\n%q\ncurrent:\n%q", tg.Path, tg.Rendered, tg.Current)
		}
		if tg.Report.AnyStale() || tg.Report.AnyEdited() || tg.Report.AnyOrphan() {
			t.Errorf("%s: report flags stale/edited/orphan on a fresh repo", tg.Path)
		}
	}
	inline := targetNamed(t, targets, AgentsFile)
	if len(inline.Modules) != 2 {
		t.Errorf("AGENTS.md modules = %d, want the 2 inline ones", len(inline.Modules))
	}
	file := targetNamed(t, targets, delegationPath)
	if len(file.Modules) != 1 || file.Modules[0].Name != "delegation" {
		t.Errorf("%s modules = %v, want just delegation", delegationPath, file.Modules)
	}
}

func TestTargetsOnEmptyRepo(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: manifestAll})
	targets, err := mustOpen(t, dir).Targets()
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	for _, tg := range targets {
		if tg.Exists {
			t.Errorf("%s: Exists = true in an empty repo", tg.Path)
		}
		if tg.Current != "" {
			t.Errorf("%s: Current = %q, want empty", tg.Path, tg.Current)
		}
		if !tg.NeedsWrite() {
			t.Errorf("%s: NeedsWrite = false, want a first render", tg.Path)
		}
		if len(tg.Report.Missing) != len(tg.Modules) {
			t.Errorf("%s: Missing = %v, want every module", tg.Path, tg.Report.Missing)
		}
	}
}

// A manifest of only file-kind modules still has AGENTS.md as a target, with no
// regions: rendering it must leave the project's own file alone.
func TestTargetsFileOnlyManifest(t *testing.T) {
	head := "# Just a head\n\nno regions here\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestFile,
		"AGENTS.md":       head,
	})
	targets, err := mustOpen(t, dir).Targets()
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	inline := targetNamed(t, targets, AgentsFile)
	if len(inline.Modules) != 0 {
		t.Errorf("AGENTS.md modules = %v, want none", inline.Modules)
	}
	if inline.NeedsWrite() {
		t.Errorf("AGENTS.md would be rewritten to %q, want it left at %q", inline.Rendered, head)
	}
}

func TestTargetsReadsThroughSymlink(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"CLAUDE.md":       "# Real file\n",
	})
	if err := os.Symlink("CLAUDE.md", filepath.Join(dir, AgentsFile)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	targets, err := mustOpen(t, dir).Targets()
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	inline := targetNamed(t, targets, AgentsFile)
	if inline.Current != "# Real file\n" {
		t.Errorf("Current = %q, want the symlink's contents", inline.Current)
	}
}

package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/module"
)

// Module bodies used across the repo tests. They are deliberately tiny: what is
// under test is the file plumbing, not the rules.
const (
	coreBody       = "core rules\n"
	principlesBody = "principles\n"
	stageBody      = "stage: build\n"
	docsBody       = "docs rules\n"
	delegationBody = "# Delegating\n\nrules\n"
	delegationPath = "project/agents/delegation.md"
	// backlogFile is a second seed path, so a test can tell one from another.
	backlogFile = "project/backlog.md"
	// templateBody and backlogTemplateBody stand in for the real templates;
	// the code must not depend on what either says.
	templateBody        = "# Gotchas\n\nseeded\n"
	backlogTemplateBody = "# Backlog\n\nseeded\n"
	// headTemplateBody stands in for templates/head.md; the code must not
	// depend on what the real template says.
	headTemplateBody = "# Project\n\nProject-owned rules go here.\n"
)

// testHash re-implements the module content hash independently of the packages
// under test, so a fixture cannot agree with the implementation by construction.
func testHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:6]
}

// region builds a region with an arbitrary marker hash, for stale fixtures.
func region(name, markerHash, body string) string {
	return "<!-- agents:begin " + name + "@" + markerHash + " -->\n" + body +
		"<!-- agents:end " + name + " -->\n"
}

// fresh builds the region a just-rendered module produces.
func fresh(name, body string) string { return region(name, testHash(body), body) }

// stale builds a region rendered from an older version of the module: its
// marker still matches its body, so it reads as stale but not hand-edited.
func stale(name, oldBody string) string { return region(name, testHash(oldBody), oldBody) }

// oldCoreBody is what core used to say, for stale fixtures.
const oldCoreBody = "old core rules\n"

func testSet(t *testing.T) module.Set {
	t.Helper()
	set, err := module.Load(fstest.MapFS{
		"core.md":        &fstest.MapFile{Data: []byte("---\nseeds: [" + GotchasFile + "]\n---\n" + coreBody)},
		"principles.md":  &fstest.MapFile{Data: []byte(principlesBody)},
		"stage-build.md": &fstest.MapFile{Data: []byte(stageBody)},
		"docs.md":        &fstest.MapFile{Data: []byte("---\nseeds: [" + backlogFile + "]\n---\n" + docsBody)},
		"delegation.md":  &fstest.MapFile{Data: []byte("---\nkind: file\npath: " + delegationPath + "\n---\n" + delegationBody)},
	})
	if err != nil {
		t.Fatalf("module.Load: %v", err)
	}
	return set
}

// testTemplates keys the seed bodies by the repo-relative path the modules
// declare, exactly as templates/<seed path> does.
func testTemplates() fstest.MapFS {
	return fstest.MapFS{
		GotchasFile: &fstest.MapFile{Data: []byte(templateBody)},
		backlogFile: &fstest.MapFile{Data: []byte(backlogTemplateBody)},
	}
}

// testTemplatesWithHead is a template FS that also carries a head template, so
// the seeded and unseeded paths are both exercised.
func testTemplatesWithHead() fstest.MapFS {
	fsys := testTemplates()
	fsys[HeadTemplate] = &fstest.MapFile{Data: []byte(headTemplateBody)}
	return fsys
}

// newRepo lays out a temp directory from repo-relative path -> content.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		writeFixture(t, dir, rel, content)
	}
	return dir
}

func writeFixture(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readFixture(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func fileExists(t *testing.T, dir, rel string) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(dir, rel))
	return err == nil
}

// snapshot records every regular file's content and every symlink's target,
// so a test can assert that a second run changed no bytes on disk.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[rel] = "-> " + target
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

func mustOpen(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Open(dir, testSet(t))
	if err != nil {
		t.Fatalf("Open(%s): %v", dir, err)
	}
	return r
}

// mustOpenSeeding opens a repo that will also create declared seeds, the way
// `agents sync` does; mustOpen deliberately leaves seeding off.
func mustOpenSeeding(t *testing.T, dir string) *Repo {
	t.Helper()
	r := mustOpen(t, dir)
	r.Templates = testTemplates()
	return r
}

// targetNamed returns the target rendering to rel.
func targetNamed(t *testing.T, targets []Target, rel string) Target {
	t.Helper()
	for _, tg := range targets {
		if tg.Path == rel {
			return tg
		}
	}
	t.Fatalf("no target for %s (have %v)", rel, targetPaths(targets))
	return Target{}
}

func targetPaths(targets []Target) []string {
	var out []string
	for _, tg := range targets {
		out = append(out, tg.Path)
	}
	return out
}

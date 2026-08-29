package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// past is an mtime far enough back that any rewrite is unmistakable.
var past = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func setMtime(t *testing.T, dir, rel string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("chtimes %s: %v", rel, err)
	}
}

func mtime(t *testing.T, dir, rel string) time.Time {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("stat %s: %v", rel, err)
	}
	return info.ModTime()
}

func mustSync(t *testing.T, r *Repo, force bool) SyncResult {
	t.Helper()
	res, err := r.Sync(force)
	if err != nil {
		t.Fatalf("Sync(force=%v): %v", force, err)
	}
	return res
}

func TestSyncNoOpOnFreshRepo(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		"AGENTS.md":       "# Head\n\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		delegationPath:    fresh("delegation", delegationBody),
	})
	setMtime(t, dir, "AGENTS.md")
	setMtime(t, dir, delegationPath)
	before := snapshot(t, dir)

	res := mustSync(t, mustOpen(t, dir), false)

	if len(res.Written) != 0 {
		t.Errorf("Written = %v, want nothing", res.Written)
	}
	if len(res.Fresh) != 2 {
		t.Errorf("Fresh = %v, want both targets", res.Fresh)
	}
	if res.Refused() {
		t.Error("Refused = true on a fresh repo")
	}
	for _, rel := range []string{"AGENTS.md", delegationPath} {
		if got := mtime(t, dir, rel); !got.Equal(past) {
			t.Errorf("%s was rewritten (mtime %v)", rel, got)
		}
	}
	if got := snapshot(t, dir); len(got) != len(before) {
		t.Errorf("file set changed: %v -> %v", before, got)
	}
}

func TestSyncRewritesStaleRegion(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       "# Head\n\n" + stale("core", oldCoreBody) + "\n" + fresh("principles", principlesBody),
	})

	res := mustSync(t, mustOpen(t, dir), false)

	want := "# Head\n\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody)
	if got := readFixture(t, dir, "AGENTS.md"); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	if len(res.Written) != 1 || res.Written[0] != AgentsFile {
		t.Errorf("Written = %v, want [%s]", res.Written, AgentsFile)
	}
}

// A stale marker hash with a body that no longer matches it is a hand edit: the
// tool must not silently discard whatever the agent wrote.
func TestSyncRefusesHandEdit(t *testing.T) {
	edited := "core rules\nAND MY OWN LINE\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		"AGENTS.md":       "# Head\n\n" + region("core", testHash(coreBody), edited) + "\n" + fresh("principles", principlesBody),
	})
	before := snapshot(t, dir)

	res, err := mustOpen(t, dir).Sync(false)
	if !errors.Is(err, ErrHandEdited) {
		t.Fatalf("Sync error = %v, want ErrHandEdited", err)
	}
	if !res.Refused() {
		t.Error("Refused = false")
	}
	if len(res.Edited) != 1 {
		t.Fatalf("Edited = %v, want one region", res.Edited)
	}
	got := res.Edited[0]
	if got.Target != AgentsFile || got.Region != "core" {
		t.Errorf("Edited[0] = %s: %s, want %s: core", got.Target, got.Region, AgentsFile)
	}
	if !strings.Contains(got.Diff, "-AND MY OWN LINE") {
		t.Errorf("diff does not show the hand-written line as a removal:\n%s", got.Diff)
	}
	if len(res.Written) != 0 {
		t.Errorf("Written = %v, want nothing written", res.Written)
	}
	// Nothing at all is written, not even the untouched file target.
	if fileExists(t, dir, delegationPath) {
		t.Errorf("%s was created despite the refusal", delegationPath)
	}
	if after := snapshot(t, dir); len(after) != len(before) || after["AGENTS.md"] != before["AGENTS.md"] {
		t.Errorf("files changed despite the refusal:\n%v", after)
	}
}

func TestSyncForceOverwritesHandEdit(t *testing.T) {
	edited := "core rules\nAND MY OWN LINE\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       "# Head\n\n" + region("core", testHash(coreBody), edited) + "\n" + fresh("principles", principlesBody),
	})

	res := mustSync(t, mustOpen(t, dir), true)

	want := "# Head\n\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody)
	if got := readFixture(t, dir, "AGENTS.md"); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	if len(res.Written) != 1 {
		t.Errorf("Written = %v, want AGENTS.md", res.Written)
	}
}

func TestSyncRendersFileModule(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: manifestFile})

	res := mustSync(t, mustOpen(t, dir), false)

	if want := fresh("delegation", delegationBody); readFixture(t, dir, delegationPath) != want {
		t.Errorf("%s:\n got %q\nwant %q", delegationPath, readFixture(t, dir, delegationPath), want)
	}
	if len(res.Written) != 1 || res.Written[0] != delegationPath {
		t.Errorf("Written = %v, want [%s]", res.Written, delegationPath)
	}
	// AGENTS.md has no inline modules here, so it must not be created.
	if fileExists(t, dir, "AGENTS.md") {
		t.Error("AGENTS.md was created for a file-only manifest")
	}

	setMtime(t, dir, delegationPath)
	again := mustSync(t, mustOpen(t, dir), false)
	if len(again.Written) != 0 {
		t.Errorf("second Sync wrote %v, want nothing", again.Written)
	}
	if got := mtime(t, dir, delegationPath); !got.Equal(past) {
		t.Errorf("%s was rewritten by the second sync", delegationPath)
	}
}

func TestSyncPreservesFileMode(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       stale("core", oldCoreBody) + "\n" + fresh("principles", principlesBody),
	})
	const mode os.FileMode = 0o640
	if err := os.Chmod(filepath.Join(dir, "AGENTS.md"), mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	mustSync(t, mustOpen(t, dir), false)

	info, err := os.Stat(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != mode {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), mode)
	}
}

// New files get 0644 rather than whatever the umask-free temp file had.
func TestSyncNewFileMode(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: manifestFile})
	mustSync(t, mustOpen(t, dir), false)
	info, err := os.Stat(filepath.Join(dir, delegationPath))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != newFileMode {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), newFileMode)
	}
}

// AGENTS.md is normally the real file, but if a repo has made it a symlink the
// atomic temp+rename must write through the link rather than replace it.
func TestSyncWritesThroughSymlink(t *testing.T) {
	head := "# Real file\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"CLAUDE.md":       head,
	})
	if err := os.Symlink("CLAUDE.md", filepath.Join(dir, AgentsFile)); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	mustSync(t, mustOpen(t, dir), false)

	info, err := os.Lstat(filepath.Join(dir, AgentsFile))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("AGENTS.md is no longer a symlink; the rename replaced the link")
	}
	want := head + "\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody)
	if got := readFixture(t, dir, "CLAUDE.md"); got != want {
		t.Errorf("CLAUDE.md (the link target):\n got %q\nwant %q", got, want)
	}
}

// A kind:file target is a document like any other: text below its end marker is
// the project's, and survives a rewrite of the region above it.
func TestSyncPreservesFileTargetTail(t *testing.T) {
	const tail = "\n## Project specifics\n\nlocal rule\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestFile,
		delegationPath:    fresh("delegation", delegationBody) + tail,
	})
	setMtime(t, dir, delegationPath)

	r := mustOpen(t, dir)
	rep, err := r.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	regions := targetNamed(t, rep.Targets, delegationPath).Report.Regions
	if len(regions) != 1 {
		t.Fatalf("regions = %+v, want one", regions)
	}
	if !regions[0].Fresh() {
		t.Errorf("region = %+v, want Fresh with a project-owned tail below it", regions[0])
	}

	res := mustSync(t, r, false)
	if len(res.Written) != 0 {
		t.Errorf("Written = %v, want nothing: the tail is not staleness", res.Written)
	}
	if got := mtime(t, dir, delegationPath); !got.Equal(past) {
		t.Errorf("%s was rewritten (mtime %v)", delegationPath, got)
	}

	// Now the module moves: the region is rewritten, the tail is not.
	newBody := delegationBody + "\nand one more rule\n"
	set, err := module.Load(fstest.MapFS{
		"delegation.md": &fstest.MapFile{Data: []byte("---\nkind: file\npath: " + delegationPath + "\n---\n" + newBody)},
	})
	if err != nil {
		t.Fatalf("module.Load: %v", err)
	}
	moved, err := Open(dir, set)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if res := mustSync(t, moved, false); len(res.Written) != 1 || res.Written[0] != delegationPath {
		t.Errorf("Written = %v, want [%s]", res.Written, delegationPath)
	}
	want := fresh("delegation", newBody) + tail
	if got := readFixture(t, dir, delegationPath); got != want {
		t.Errorf("%s:\n got %q\nwant %q", delegationPath, got, want)
	}
}

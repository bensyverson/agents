package repo

import (
	"reflect"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

// gitignoreHeader is the line the tool writes above the lines it appends,
// spelled out so the test says for itself what lands in the file.
const gitignoreHeader = "# agents\n"

func mustGitignore(t *testing.T, r *Repo, mode ApplyMode) GitignoreResult {
	t.Helper()
	res, err := r.Gitignore(mode)
	if err != nil {
		t.Fatalf("Gitignore: %v", err)
	}
	return res
}

// Lines come in manifest order, each once however many modules declare it.
func TestIgnoreLinesInModuleOrderWithoutDuplicates(t *testing.T) {
	r := openDeclaring(t, declaringRepo(t, nil))
	if got, want := r.IgnoreLines(), []string{scratchDir, outDir, cacheDir}; !reflect.DeepEqual(got, want) {
		t.Errorf("IgnoreLines() = %v, want %v", got, want)
	}
}

func TestGitignoreCreatesTheFile(t *testing.T) {
	dir := declaringRepo(t, nil)
	res := mustGitignore(t, openDeclaring(t, dir), ApplyWrite)

	if want := []string{scratchDir, outDir, cacheDir}; !reflect.DeepEqual(res.Added, want) {
		t.Errorf("Added = %v, want %v", res.Added, want)
	}
	if res.Path != GitignoreFile {
		t.Errorf("Path = %q, want %q", res.Path, GitignoreFile)
	}
	want := gitignoreHeader + scratchDir + "\n" + outDir + "\n" + cacheDir + "\n"
	if got := readFixture(t, dir, GitignoreFile); got != want {
		t.Errorf("%s:\n got %q\nwant %q", GitignoreFile, got, want)
	}
}

// Only missing lines are appended, under the header, after a blank line; the
// project's own lines keep their place, and a missing final newline is mended.
func TestGitignoreAppendsOnlyMissingLines(t *testing.T) {
	dir := declaringRepo(t, map[string]string{GitignoreFile: "node_modules\n" + outDir})
	res := mustGitignore(t, openDeclaring(t, dir), ApplyWrite)

	if want := []string{scratchDir, cacheDir}; !reflect.DeepEqual(res.Added, want) {
		t.Errorf("Added = %v, want %v", res.Added, want)
	}
	want := "node_modules\n" + outDir + "\n\n" + gitignoreHeader + scratchDir + "\n" + cacheDir + "\n"
	if got := readFixture(t, dir, GitignoreFile); got != want {
		t.Errorf("%s:\n got %q\nwant %q", GitignoreFile, got, want)
	}
}

// A line a module gains later joins the block an earlier run wrote, wherever
// it sits, rather than starting a second block.
func TestGitignoreExtendsItsOwnBlock(t *testing.T) {
	dir := declaringRepo(t, map[string]string{
		GitignoreFile: "# build\nbin/\n\n" + gitignoreHeader + scratchDir + "\n\n# editor\n*.swp\n",
	})
	mustGitignore(t, openDeclaring(t, dir), ApplyWrite)

	want := "# build\nbin/\n\n" + gitignoreHeader + scratchDir + "\n" + outDir + "\n" + cacheDir + "\n\n# editor\n*.swp\n"
	if got := readFixture(t, dir, GitignoreFile); got != want {
		t.Errorf("%s:\n got %q\nwant %q", GitignoreFile, got, want)
	}
}

func TestGitignoreTwiceChangesNothing(t *testing.T) {
	dir := declaringRepo(t, map[string]string{GitignoreFile: "node_modules\n"})
	if first := mustGitignore(t, openDeclaring(t, dir), ApplyWrite); len(first.Added) == 0 {
		t.Fatal("first run added nothing; the second run would prove nothing")
	}
	before := snapshot(t, dir)

	res := mustGitignore(t, openDeclaring(t, dir), ApplyWrite)

	if len(res.Added) != 0 {
		t.Errorf("second run Added = %v, want nothing", res.Added)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("second run changed the repo:\nbefore %v\nafter  %v", before, after)
	}
}

func TestGitignoreDryRunWritesNothing(t *testing.T) {
	dir := declaringRepo(t, map[string]string{GitignoreFile: "node_modules\n"})
	before := snapshot(t, dir)

	res := mustGitignore(t, openDeclaring(t, dir), ApplyDryRun)

	if want := []string{scratchDir, outDir, cacheDir}; !reflect.DeepEqual(res.Added, want) {
		t.Errorf("dry run Added = %v, want %v", res.Added, want)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("dry run changed the repo:\nbefore %v\nafter  %v", before, after)
	}
}

// Modules that declare nothing leave a repo without a .gitignore without one.
func TestGitignoreWithNothingDeclaredCreatesNoFile(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: "modules:\n  - core\n"})
	res := mustGitignore(t, mustOpen(t, dir), ApplyWrite)

	if len(res.Added) != 0 || res.Path != GitignoreFile {
		t.Errorf("Added = %v, Path = %q; want nothing added to %s", res.Added, res.Path, GitignoreFile)
	}
	if fileExists(t, dir, GitignoreFile) {
		t.Errorf("%s was created with nothing to put in it", GitignoreFile)
	}
}

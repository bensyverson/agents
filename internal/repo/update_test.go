package repo

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

// The second generation of the git fixture's module, and a module the repo
// never enables, so a commit can change one without touching the other.
const (
	movedModuleBody = "rules from the git source, revised\n"
	extraModuleBody = "a module nobody enabled\n"
)

// gitOnlyManifest names the git fixture as the only source, at ref.
func gitOnlyManifest(url, ref, modules string) string {
	return "sources:\n  - name: house\n    git: " + url + "\n    ref: " + ref + "\n" +
		"modules:\n" + modules
}

// pushCommit writes files into a clone of the bare fixture and pushes them to
// main, returning the new commit's sha — a source that moved on since the
// manifest was pinned.
func pushCommit(t *testing.T, url string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "clone", url, "work")
	work := filepath.Join(root, "work")
	writeTree(t, work, files)
	git(t, work, "add", "-A")
	git(t, work, "commit", "-m", "two")
	git(t, work, "push", "origin", "HEAD:main")
	return git(t, work, "rev-parse", "HEAD")
}

// tagCommit pushes a commit and tags it, for pinning with --ref.
func tagCommit(t *testing.T, url, tag string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "clone", url, "work")
	work := filepath.Join(root, "work")
	writeTree(t, work, files)
	git(t, work, "add", "-A")
	git(t, work, "commit", "-m", "tagged")
	git(t, work, "tag", tag)
	git(t, work, "push", "origin", "HEAD:main")
	git(t, work, "push", "origin", tag)
	return git(t, work, "rev-parse", "HEAD")
}

// syncedRepo is a repo pinned to ref, opened once with FetchMissing so the
// cache holds the pinned tree, and rendered — the state `agents update` runs
// against.
func syncedRepo(t *testing.T, url, ref, modules string) (dir, cache string) {
	t.Helper()
	dir = newRepo(t, map[string]string{manifest.FileName: gitOnlyManifest(url, ref, modules)})
	cache = t.TempDir()
	r, err := Open(dir, testOptionsWith(cache, FetchMissing))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r.Seeds = SeedsCreated
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return dir, cache
}

// mustUpdate opens the repo and updates it.
func mustUpdate(t *testing.T, dir, cache string, up UpdateOptions) UpdateResult {
	t.Helper()
	opts := testOptionsWith(cache, FetchMissing)
	r, err := Open(dir, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r.Seeds = SeedsCreated
	res, err := r.Update(opts, up)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	return res
}

// A commit that changes an enabled module moves the pin to that sha, reports
// the module's diff, and re-renders the region.
func TestUpdateMovesThePinAndDiffsTheChangedModule(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, fx.sha, "  - one\n")
	newSha := pushCommit(t, fx.url, map[string]string{"modules/one.md": "---\nseeds: [" + gitSeedFile + "]\n---\n" + movedModuleBody})

	res := mustUpdate(t, dir, cache, UpdateOptions{})

	if len(res.Updated) != 1 {
		t.Fatalf("Updated = %+v, want one source", res.Updated)
	}
	up := res.Updated[0]
	if up.Name != "house" {
		t.Errorf("Updated[0].Name = %q, want %q", up.Name, "house")
	}
	if up.FromCommit != fx.sha || up.To != newSha {
		t.Errorf("Updated[0] moved %q -> %q, want %q -> %q", up.FromCommit, up.To, fx.sha, newSha)
	}
	if !up.Moved() {
		t.Error("Updated[0].Moved() = false, want true for a source that moved commit")
	}
	if len(up.Changes) != 1 || up.Changes[0].Module != "one" {
		t.Fatalf("Changes = %+v, want one change to module one", up.Changes)
	}
	diff := up.Changes[0].Diff
	if !strings.Contains(diff, "-"+strings.TrimSuffix(gitModuleBody, "\n")) ||
		!strings.Contains(diff, "+"+strings.TrimSuffix(movedModuleBody, "\n")) {
		t.Errorf("diff does not show the module's old and new body:\n%s", diff)
	}

	written := readFixture(t, dir, manifest.FileName)
	if !strings.Contains(written, "ref: "+newSha) {
		t.Errorf("%s is not pinned to the new sha:\n%s", manifest.FileName, written)
	}
	if got, want := readFixture(t, dir, AgentsFile), fresh("one", movedModuleBody); got != want {
		t.Errorf("%s:\n got %q\nwant %q", AgentsFile, got, want)
	}
}

// A commit that touches only a module the repo does not enable still moves the
// pin, and reports no diff: what changed is not in this repo's rules.
func TestUpdateReportsNoChangeForAModuleTheRepoDoesNotEnable(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, fx.sha, "  - one\n")
	newSha := pushCommit(t, fx.url, map[string]string{"modules/extra.md": extraModuleBody})

	res := mustUpdate(t, dir, cache, UpdateOptions{})

	if len(res.Updated) != 1 {
		t.Fatalf("Updated = %+v, want one source", res.Updated)
	}
	if got := res.Updated[0].Changes; len(got) != 0 {
		t.Errorf("Changes = %+v, want none: no enabled module changed", got)
	}
	if got := res.Updated[0].To; got != newSha {
		t.Errorf("To = %q, want the new sha %q", got, newSha)
	}
	if written := readFixture(t, dir, manifest.FileName); !strings.Contains(written, "ref: "+newSha) {
		t.Errorf("%s is not pinned to the new sha:\n%s", manifest.FileName, written)
	}
}

// A source already at the commit its ref names is left entirely alone: no
// report, and not one byte of the manifest rewritten.
func TestUpdateOnACurrentSourceReportsNothing(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, fx.sha, "  - one\n")
	before := readFixture(t, dir, manifest.FileName)

	res := mustUpdate(t, dir, cache, UpdateOptions{})

	if len(res.Updated) != 0 {
		t.Errorf("Updated = %+v, want nothing for a source already at its commit", res.Updated)
	}
	if got := readFixture(t, dir, manifest.FileName); got != before {
		t.Errorf("%s was rewritten:\n got %q\nwant %q", manifest.FileName, got, before)
	}
}

// Decision 7: a branch a human wrote by hand is replaced by the sha it names,
// even when that commit is the one already rendered.
func TestUpdateReplacesAHandWrittenBranchWithASha(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, "main", "  - one\n")

	res := mustUpdate(t, dir, cache, UpdateOptions{})

	if len(res.Updated) != 1 {
		t.Fatalf("Updated = %+v, want the re-pin of the branch", res.Updated)
	}
	up := res.Updated[0]
	if up.From != "main" {
		t.Errorf("From = %q, want the branch the manifest held", up.From)
	}
	if up.To != fx.sha {
		t.Errorf("To = %q, want the branch's commit %q", up.To, fx.sha)
	}
	if up.Moved() {
		t.Error("Moved() = true, want false: the commit did not change, only the pin's spelling")
	}
	written := readFixture(t, dir, manifest.FileName)
	if !strings.Contains(written, "ref: "+fx.sha) || strings.Contains(written, "ref: main") {
		t.Errorf("%s still names the branch:\n%s", manifest.FileName, written)
	}
}

// --ref pins to whatever that ref names, a tag included.
func TestUpdateWithARefPinsToThatRefsCommit(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, fx.sha, "  - one\n")
	tagged := tagCommit(t, fx.url, "v1", map[string]string{"modules/one.md": movedModuleBody})
	// A later commit on the default branch, so pinning the tag is visibly not
	// the same as taking HEAD.
	pushCommit(t, fx.url, map[string]string{"modules/extra.md": extraModuleBody})

	res := mustUpdate(t, dir, cache, UpdateOptions{Ref: "v1"})

	if len(res.Updated) != 1 || res.Updated[0].To != tagged {
		t.Fatalf("Updated = %+v, want the tag's commit %q", res.Updated, tagged)
	}
	if written := readFixture(t, dir, manifest.FileName); !strings.Contains(written, "ref: "+tagged) {
		t.Errorf("%s is not pinned to the tag's commit:\n%s", manifest.FileName, written)
	}
}

// A manifest with nothing to pin — a path source, or none at all — is a
// no-op: there is no ref to move.
func TestUpdateWithNoGitSourceDoesNothing(t *testing.T) {
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{
		manifest.FileName: "sources:\n  - name: local\n    path: " + pathDir + "\nmodules:\n  - two\n",
	})
	before := readFixture(t, dir, manifest.FileName)

	res := mustUpdate(t, dir, t.TempDir(), UpdateOptions{})

	if len(res.Updated) != 0 {
		t.Errorf("Updated = %+v, want nothing: a path source has no pin", res.Updated)
	}
	if got := readFixture(t, dir, manifest.FileName); got != before {
		t.Errorf("%s was rewritten:\n%s", manifest.FileName, got)
	}
}

// Naming one source updates that one alone; naming a source the manifest does
// not list is an error rather than a silent no-op.
func TestUpdateNamesSelectSources(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{
		manifest.FileName: sourcesManifest(fx, pathDir, "  - one\n  - local/two\n"),
	})
	cache := t.TempDir()
	opts := testOptionsWith(cache, FetchMissing)
	r, err := Open(dir, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r.Seeds = SeedsCreated
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	newSha := pushCommit(t, fx.url, map[string]string{"modules/one.md": movedModuleBody})

	// A path source names nothing to move, so asking for it changes nothing.
	res := mustUpdate(t, dir, cache, UpdateOptions{Sources: []string{"local"}})
	if len(res.Updated) != 0 {
		t.Errorf("Updated = %+v, want nothing for a path source", res.Updated)
	}
	if written := readFixture(t, dir, manifest.FileName); strings.Contains(written, "ref: "+newSha) {
		t.Errorf("updating the path source moved the git source's pin:\n%s", written)
	}

	res = mustUpdate(t, dir, cache, UpdateOptions{Sources: []string{"house"}})
	if len(res.Updated) != 1 || res.Updated[0].To != newSha {
		t.Fatalf("Updated = %+v, want house at %q", res.Updated, newSha)
	}

	r, err = Open(dir, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := r.Update(opts, UpdateOptions{Sources: []string{"nosuch"}}); err == nil {
		t.Error("Update with an unlisted source name returned nil, want an error naming it")
	} else if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error %v does not name the source", err)
	}
}

// A hand-edited region stops the render exactly as it stops sync, and the
// result carries the diff the caller prints.
func TestUpdateRefusesAHandEditedRegion(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, fx.sha, "  - one\n")
	// The edit lands inside the region, so its marker hash no longer matches.
	current := readFixture(t, dir, AgentsFile)
	writeFixture(t, dir, AgentsFile, strings.Replace(current, gitModuleBody, gitModuleBody+"my own line\n", 1))
	pushCommit(t, fx.url, map[string]string{"modules/one.md": movedModuleBody})

	opts := testOptionsWith(cache, FetchMissing)
	r, err := Open(dir, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	res, err := r.Update(opts, UpdateOptions{})
	if err == nil {
		t.Fatal("Update = nil error over a hand-edited region")
	}
	if !errors.Is(err, ErrHandEdited) {
		t.Errorf("Update error = %v, want it to wrap ErrHandEdited", err)
	}
	if !res.Sync.Refused() {
		t.Errorf("Sync result does not carry the refusal: %+v", res.Sync)
	}
}

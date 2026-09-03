package repo

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// manifestSituational enables one inline module and both situational ones, in
// an order the alphabet would not give, so the table's order is the manifest's.
const manifestSituational = "modules:\n  - core\n  - beta\n  - alpha\n"

const situationalHead = "# Head\n\n"

// TestSyncRendersTheIndex is the whole feature end to end: two when: modules
// put one index region, with a row each in manifest order, after the last
// inline region and above the project's tail — and syncing again does nothing.
func TestSyncRendersTheIndex(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestSituational,
		AgentsFile:        situationalHead + fresh("core", coreBody) + projectTail,
	})

	res := mustSync(t, mustOpen(t, dir), false)

	want := situationalHead + fresh("core", coreBody) + "\n" +
		fresh(indexRegion, indexBody(betaRow, alphaRow)) + projectTail
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(strings.Join(res.Written, ","), AgentsFile) {
		t.Errorf("Written = %v, want %s among them", res.Written, AgentsFile)
	}
	wantClean(t, dir)

	setMtime(t, dir, AgentsFile)
	before := snapshot(t, dir)
	again := mustSync(t, mustOpen(t, dir), false)
	if len(again.Written) != 0 {
		t.Errorf("second Sync wrote %v, want a no-op", again.Written)
	}
	if got := mtime(t, dir, AgentsFile); !got.Equal(past) {
		t.Errorf("%s was rewritten by the second sync", AgentsFile)
	}
	if got := snapshot(t, dir); len(got) != len(before) {
		t.Errorf("file set changed: %v -> %v", before, got)
	}
}

// A repo with no situational module has no index region, and a stray one is an
// orphan the sync removes — the index behaves exactly like a dropped module.
func TestSyncRendersNoIndexWithoutAWhenModule(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		AgentsFile: situationalHead + fresh("core", coreBody) + "\n" +
			fresh("principles", principlesBody) + "\n" + fresh(indexRegion, indexBody(alphaRow)),
		delegationPath: fresh("delegation", delegationBody),
	})

	mustSync(t, mustOpen(t, dir), false)

	// The blank line that separated the dropped region from the one above it
	// stays, exactly as it does for any other module that leaves the manifest.
	want := situationalHead + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody) + "\n"
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	wantClean(t, dir)
}

// Remove is not a sync — it touches only the named regions — but the index is
// derived from the manifest, so it has to follow: a row goes when one of two
// situational modules leaves, and the whole region goes with the last one.
func TestRemoveDropsTheIndexRowThenTheRegion(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestSituational,
		AgentsFile: situationalHead + fresh("core", coreBody) + "\n" +
			fresh(indexRegion, indexBody(betaRow, alphaRow)),
		alphaPath: fresh("alpha", alphaBody),
		betaPath:  fresh("beta", betaBody),
	})

	mustRemove(t, mustOpen(t, dir), "beta")

	oneRow := situationalHead + fresh("core", coreBody) + "\n" + fresh(indexRegion, indexBody(alphaRow))
	if got := readFixture(t, dir, AgentsFile); got != oneRow {
		t.Errorf("after removing beta, AGENTS.md:\n got %q\nwant %q", got, oneRow)
	}
	wantClean(t, dir)

	mustRemove(t, mustOpen(t, dir), "alpha")

	none := situationalHead + fresh("core", coreBody) + "\n"
	if got := readFixture(t, dir, AgentsFile); got != none {
		t.Errorf("after removing alpha, AGENTS.md:\n got %q\nwant %q", got, none)
	}
	wantClean(t, dir)

	mustAdd(t, mustOpen(t, dir), "alpha")

	got := readFixture(t, dir, AgentsFile)
	if !strings.Contains(got, fresh(indexRegion, indexBody(alphaRow))) {
		t.Errorf("adding alpha back did not re-create the index:\n%q", got)
	}
	if !fileExists(t, dir, alphaPath) {
		t.Errorf("%s was not re-rendered", alphaPath)
	}
	wantClean(t, dir)
}

// The index is checked like any other region: a moved when: phrase makes it
// stale, and an edit inside it makes it hand-edited.
func TestCheckReportsTheIndexStaleAndHandEdited(t *testing.T) {
	const oldIndex = "modules:\n  - core\n  - alpha\n"
	current := indexBody(alphaRow)
	oldBody := indexPre + "| When alpha used to apply | `" + alphaPath + "` |\n"

	t.Run("stale", func(t *testing.T) {
		dir := newRepo(t, map[string]string{
			manifest.FileName: oldIndex,
			AgentsFile:        situationalHead + fresh("core", coreBody) + "\n" + stale(indexRegion, oldBody),
			alphaPath:         fresh("alpha", alphaBody),
		})
		want := []string{"AGENTS.md: index stale"}
		if got := problemLines(mustCheck(t, dir)); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("problems = %v, want %v", got, want)
		}
	})

	t.Run("hand-edited", func(t *testing.T) {
		dir := newRepo(t, map[string]string{
			manifest.FileName: oldIndex,
			AgentsFile: situationalHead + fresh("core", coreBody) + "\n" +
				region(indexRegion, testHash(current), current+"| Mine | `project/agents/mine.md` |\n"),
			alphaPath: fresh("alpha", alphaBody),
		})
		want := []string{"AGENTS.md: index hand-edited"}
		if got := problemLines(mustCheck(t, dir)); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("problems = %v, want %v", got, want)
		}
	})
}

// Diff has to have a body to diff against, which an orphan region would not
// give it: the index is a module to Diff like everything else.
func TestDiffShowsAnEditedIndex(t *testing.T) {
	current := indexBody(alphaRow)
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n  - alpha\n",
		AgentsFile: situationalHead + fresh("core", coreBody) + "\n" +
			region(indexRegion, testHash(current), current+"| Mine | `project/agents/mine.md` |\n"),
		alphaPath: fresh("alpha", alphaBody),
	})

	rep, err := mustOpen(t, dir).Diff()
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(rep.Edits) != 1 {
		t.Fatalf("Edits = %+v, want one", rep.Edits)
	}
	if rep.Edits[0].Region != indexRegion || rep.Edits[0].Target != AgentsFile {
		t.Errorf("Edits[0] = %s: %s, want %s: index", rep.Edits[0].Target, rep.Edits[0].Region, AgentsFile)
	}
	if !strings.Contains(rep.Edits[0].Diff, "+| Mine |") {
		t.Errorf("diff does not show the hand-written row as an addition:\n%s", rep.Edits[0].Diff)
	}
}

// The index is generated, not enabled: naming it in .agents.yaml names a
// module that does not exist.
func TestIndexIsNotAManifestEntry(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: "modules:\n  - core\n  - index\n"})

	_, err := Open(dir, testSet(t))

	var unknown *manifest.UnknownModulesError
	if !errors.As(err, &unknown) {
		t.Fatalf("got %v, want *manifest.UnknownModulesError", err)
	}
	if len(unknown.Names) != 1 || unknown.Names[0] != indexRegion {
		t.Errorf("Names = %v, want [index]", unknown.Names)
	}
}

// "index" is the generated region's name, so a source may not define a module
// that would collide with it.
func TestAModuleNamedIndexIsRefused(t *testing.T) {
	_, err := module.Load(fstest.MapFS{
		"core.md":  &fstest.MapFile{Data: []byte(coreBody)},
		"index.md": &fstest.MapFile{Data: []byte("my own index\n")},
	})
	if err == nil {
		t.Fatal("want an error loading a module named index, got nil")
	}
	if !strings.Contains(err.Error(), indexRegion) {
		t.Errorf("error %q does not name the module", err)
	}
}

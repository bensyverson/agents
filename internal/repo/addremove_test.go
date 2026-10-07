package repo

import (
	"errors"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

// The three kinds of project-owned text a managed AGENTS.md can carry. Not one
// byte of any of them may move when a module is added or removed.
const (
	projectHead = "# Project\n\nDocumentation list, rulings, whatever the project says.\n\n"
	handWritten = "\n## A hand-written section\n\nIt sits between two regions.\n\n"
	projectTail = "\n## A tail\n\nBelow the last region.\n"
)

func mustAdd(t *testing.T, r *Repo, names ...string) AddResult {
	t.Helper()
	res, err := r.Add(names)
	if err != nil {
		t.Fatalf("Add(%v): %v", names, err)
	}
	return res
}

func mustRemove(t *testing.T, r *Repo, names ...string) RemoveResult {
	t.Helper()
	res, err := r.Remove(names)
	if err != nil {
		t.Fatalf("Remove(%v): %v", names, err)
	}
	return res
}

// wantManifest asserts the manifest on disk lists exactly these modules, in order.
func wantManifest(t *testing.T, dir string, names ...string) {
	t.Helper()
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	if strings.Join(m.Entries(), ",") != strings.Join(names, ",") {
		t.Errorf("manifest = %v, want %v", m.Entries(), names)
	}
}

// wantClean asserts a sync would now do nothing: add and remove both have to
// leave the repo in the state the renderer would produce, or every later
// `agents check` fails.
func wantClean(t *testing.T, dir string) {
	t.Helper()
	rep, err := mustOpen(t, dir).Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !rep.OK() {
		t.Errorf("repo is not clean afterwards: %v", rep.Problems)
	}
}

// baseRepo is a managed repo with two inline regions and a project-owned head.
func baseRepo(t *testing.T) string {
	t.Helper()
	return newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       projectHead + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		GotchasFile:       templateBody,
	})
}

// openRemoving opens a repo the way `agents remove` does.
func openRemoving(t *testing.T, dir string, opts Options) *Repo {
	t.Helper()
	r, err := OpenForRemove(dir, opts)
	if err != nil {
		t.Fatalf("OpenForRemove(%s): %v", dir, err)
	}
	return r
}

// A fresh `modules: []` is the state a repo is left in before its first
// module: add fills it, where every verb that renders refuses it.
func TestAddToAnEmptyManifest(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: "modules: []\n"})

	res := mustAdd(t, mustOpenSeeding(t, dir), "core")

	if strings.Join(res.Added, ",") != "core" {
		t.Errorf("Added = %v, want [core]", res.Added)
	}
	wantManifest(t, dir, "core")
	if got := readFixture(t, dir, "AGENTS.md"); !strings.Contains(got, fresh("core", coreBody)) {
		t.Errorf("AGENTS.md does not hold the core region:\n%s", got)
	}
	wantClean(t, dir)
}

func TestRenderingAnEmptyManifestIsRefused(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: "modules: []\n"})
	r := mustOpen(t, dir)

	if _, err := r.Targets(); !errors.Is(err, manifest.ErrNoModules) || !strings.Contains(err.Error(), "agents add") {
		t.Errorf("Targets error = %v, want ErrNoModules", err)
	}
	if _, err := r.Sync(false); !errors.Is(err, manifest.ErrNoModules) {
		t.Errorf("Sync error = %v, want ErrNoModules", err)
	}
}

// A manifest with sources: has left the embedded modules behind; a module only
// the binary supplies is explained rather than blamed on a source that never
// held it.
func TestOpenExplainsEmbeddedModulesUnderSources(t *testing.T) {
	hermeticGit(t)
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{
		manifest.FileName: "sources:\n  - name: local\n    path: " + pathDir + "\nmodules:\n  - two\n  - core\n  - delegation\n  - nosuch\n",
	})

	_, err := Open(dir, testOptionsWith(t.TempDir(), FetchNever))

	if _, ok := errors.AsType[*manifest.UnknownModulesError](err); !ok {
		t.Fatalf("Open error = %v, want *manifest.UnknownModulesError", err)
	}
	for _, want := range []string{
		"unknown modules in .agents.yaml: local/nosuch",
		"core and delegation are example modules built into the binary, which a manifest with sources: does not use",
		"run agents remove core delegation",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "local/core") {
		t.Errorf("error %q qualifies an embedded module with a source", err)
	}
}

// The error above tells the user to run remove, so remove has to work on the
// manifest that error describes — without resolving what it is removing.
func TestRemoveEmbeddedModulesFromASourcesManifest(t *testing.T) {
	hermeticGit(t)
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{
		manifest.FileName: "sources:\n  - name: local\n    path: " + pathDir + "\nmodules:\n  - two\n  - core\n  - delegation\n",
		"AGENTS.md":       projectHead + fresh("two", pathModuleBody) + "\n" + fresh("core", coreBody),
		delegationPath:    fresh("delegation", delegationBody),
	})
	opts := testOptionsWith(t.TempDir(), FetchNever)
	if _, err := Open(dir, opts); err == nil {
		t.Fatal("Open succeeded on a manifest listing embedded modules under sources:")
	}

	res, err := openRemoving(t, dir, opts).Remove([]string{"core", "delegation"})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if len(res.Regions) != 2 || !res.Regions[0].Present || !res.Regions[1].FileDeleted {
		t.Errorf("Regions = %+v, want core's region removed and delegation's file deleted", res.Regions)
	}
	wantManifest(t, dir, "two")
	got := readFixture(t, dir, "AGENTS.md")
	if strings.Contains(got, "agents:begin core") || !strings.HasPrefix(got, projectHead) || !strings.Contains(got, fresh("two", pathModuleBody)) {
		t.Errorf("AGENTS.md after remove:\n%s", got)
	}
	if fileExists(t, dir, delegationPath) {
		t.Errorf("%s survived its module's removal", delegationPath)
	}
	if _, err := Open(dir, opts); err != nil {
		t.Errorf("Open after remove: %v", err)
	}
}

// A listed module that no source supplies and the binary does not either can
// still be dropped: its region is found by name.
func TestRemoveAnUnresolvableModuleByName(t *testing.T) {
	hermeticGit(t)
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{
		manifest.FileName: "sources:\n  - name: local\n    path: " + pathDir + "\nmodules:\n  - two\n  - ghost\n",
		"AGENTS.md":       projectHead + fresh("two", pathModuleBody) + "\n" + fresh("ghost", "ghost rules\n"),
	})

	if _, err := openRemoving(t, dir, testOptionsWith(t.TempDir(), FetchNever)).Remove([]string{"ghost"}); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	wantManifest(t, dir, "two")
	if got := readFixture(t, dir, "AGENTS.md"); strings.Contains(got, "ghost") {
		t.Errorf("ghost's region survived:\n%s", got)
	}
}

// Remove still refuses a name the manifest does not list, whether or not any
// source supplies it.
func TestRemoveStillRefusesWhatIsNotListed(t *testing.T) {
	dir := baseRepo(t)
	r := openRemoving(t, dir, testOptions())

	if _, err := r.Remove([]string{"nosuch"}); !errors.Is(err, ErrUnknownModule) {
		t.Errorf("Remove(nosuch) error = %v, want ErrUnknownModule", err)
	}
	if _, err := r.Remove([]string{"docs"}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Remove(docs) error = %v, want ErrNotEnabled", err)
	}
}

func TestAddEnablesRendersAndSeeds(t *testing.T) {
	dir := baseRepo(t)

	res := mustAdd(t, mustOpenSeeding(t, dir), "docs", "delegation")

	if strings.Join(res.Added, ",") != "docs,delegation" {
		t.Errorf("Added = %v, want [docs delegation]", res.Added)
	}
	if len(res.Already) != 0 {
		t.Errorf("Already = %v, want nothing", res.Already)
	}
	wantManifest(t, dir, "core", "principles", "docs", "delegation")

	wantAgents := projectHead + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody) +
		"\n" + fresh("docs", docsBody)
	if got := readFixture(t, dir, "AGENTS.md"); got != wantAgents {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, wantAgents)
	}
	if got := readFixture(t, dir, delegationPath); got != fresh("delegation", delegationBody) {
		t.Errorf("%s:\n got %q\nwant %q", delegationPath, got, fresh("delegation", delegationBody))
	}
	if !strings.Contains(strings.Join(res.Sync.Written, ","), delegationPath) {
		t.Errorf("Written = %v, want %s among them", res.Sync.Written, delegationPath)
	}
	if strings.Join(res.Sync.Seeded, ",") != backlogFile {
		t.Errorf("Seeded = %v, want [%s]", res.Sync.Seeded, backlogFile)
	}
	if got := readFixture(t, dir, backlogFile); got != backlogTemplateBody {
		t.Errorf("%s = %q, want the template body", backlogFile, got)
	}
	wantClean(t, dir)
}

// The head, a hand-written section between two regions, and a tail below the
// last one all have to survive an add byte for byte — and the new region has
// to land in manifest order, after the last region and above the tail.
func TestAddKeepsEveryByteOfProjectText(t *testing.T) {
	doc := projectHead + fresh("core", coreBody) + handWritten + fresh("principles", principlesBody) + projectTail
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       doc,
		GotchasFile:       templateBody,
	})

	mustAdd(t, mustOpenSeeding(t, dir), "docs")

	want := projectHead + fresh("core", coreBody) + handWritten + fresh("principles", principlesBody) +
		"\n" + fresh("docs", docsBody) + projectTail
	got := readFixture(t, dir, "AGENTS.md")
	if got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	if !strings.HasPrefix(got, projectHead) {
		t.Errorf("the project-owned head changed:\n%q", got)
	}
	if !strings.Contains(got, handWritten) {
		t.Errorf("the hand-written section between the regions was disturbed:\n%q", got)
	}
	wantClean(t, dir)
}

func TestAddOnAlreadyEnabledModuleChangesNothing(t *testing.T) {
	dir := baseRepo(t)
	before := snapshot(t, dir)

	res := mustAdd(t, mustOpenSeeding(t, dir), "core")

	if strings.Join(res.Already, ",") != "core" {
		t.Errorf("Already = %v, want [core]", res.Already)
	}
	if len(res.Added) != 0 {
		t.Errorf("Added = %v, want nothing", res.Added)
	}
	if len(res.Sync.Written) != 0 {
		t.Errorf("Written = %v, want nothing", res.Sync.Written)
	}
	if got := snapshot(t, dir); !maps.Equal(got, before) {
		t.Errorf("files changed: %v -> %v", before, got)
	}
}

func TestAddUnknownModuleIsAnError(t *testing.T) {
	dir := baseRepo(t)
	before := snapshot(t, dir)

	_, err := mustOpenSeeding(t, dir).Add([]string{"docs", "nosuch"})
	if !errors.Is(err, ErrUnknownModule) {
		t.Fatalf("Add error = %v, want ErrUnknownModule", err)
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error %q does not name the unknown module", err)
	}
	if got := snapshot(t, dir); !maps.Equal(got, before) {
		t.Errorf("a failed add wrote something: %v -> %v", before, got)
	}
}

// Add renders through the same sync, so it refuses on a hand edit the same
// way — and must leave the manifest alone when it does.
func TestAddRefusesOnHandEditedRegion(t *testing.T) {
	edited := coreBody + "AND MY OWN LINE\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       projectHead + region("core", testHash(coreBody), edited) + "\n" + fresh("principles", principlesBody),
		GotchasFile:       templateBody,
	})
	before := snapshot(t, dir)

	res, err := mustOpenSeeding(t, dir).Add([]string{"docs"})
	if !errors.Is(err, ErrHandEdited) {
		t.Fatalf("Add error = %v, want ErrHandEdited", err)
	}
	if len(res.Sync.Edited) != 1 || res.Sync.Edited[0].Region != "core" {
		t.Errorf("Edited = %v, want the core region", res.Sync.Edited)
	}
	if got := snapshot(t, dir); !maps.Equal(got, before) {
		t.Errorf("a refused add wrote something: %v -> %v", before, got)
	}
}

func TestRemoveDropsRegionAndDeletesItsFile(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		"AGENTS.md":       projectHead + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		delegationPath:    fresh("delegation", delegationBody),
		GotchasFile:       templateBody,
	})
	agentsBefore := readFixture(t, dir, "AGENTS.md")

	res := mustRemove(t, mustOpen(t, dir), "delegation")

	if strings.Join(res.Removed, ",") != "delegation" {
		t.Errorf("Removed = %v, want [delegation]", res.Removed)
	}
	if len(res.Regions) != 1 {
		t.Fatalf("Regions = %v, want one", res.Regions)
	}
	got := res.Regions[0]
	if got.Target != delegationPath || !got.Present || !got.FileDeleted || got.FileKept || got.HandEdited {
		t.Errorf("Regions[0] = %+v, want the delegation file deleted", got)
	}
	if fileExists(t, dir, delegationPath) {
		t.Errorf("%s still exists", delegationPath)
	}
	wantManifest(t, dir, "core", "principles")
	if now := readFixture(t, dir, "AGENTS.md"); now != agentsBefore {
		t.Errorf("AGENTS.md changed:\n got %q\nwant %q", now, agentsBefore)
	}
	wantClean(t, dir)
}

// A file the project has written in is not the tool's to delete: the region
// goes, the file and every project-owned byte around it stay, and the caller
// is told so it can warn.
func TestRemoveKeepsAHandExtendedFile(t *testing.T) {
	fileHead := "# Delegating here\n\n"
	fileTail := "\n## Our own additions\n\nProject-written.\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		"AGENTS.md":       projectHead + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		delegationPath:    fileHead + fresh("delegation", delegationBody) + fileTail,
		GotchasFile:       templateBody,
	})

	res := mustRemove(t, mustOpen(t, dir), "delegation")

	if got := res.Regions[0]; !got.FileKept || got.FileDeleted {
		t.Errorf("Regions[0] = %+v, want the file kept", got)
	}
	want := fileHead + strings.TrimPrefix(fileTail, "\n")
	if got := readFixture(t, dir, delegationPath); got != want {
		t.Errorf("%s:\n got %q\nwant %q", delegationPath, got, want)
	}
	wantManifest(t, dir, "core", "principles")
}

func TestRemoveInlineRegionKeepsEveryOtherByte(t *testing.T) {
	doc := projectHead + fresh("core", coreBody) + handWritten + fresh("principles", principlesBody) + projectTail
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md":       doc,
		GotchasFile:       templateBody,
	})

	res := mustRemove(t, mustOpen(t, dir), "principles")

	want := projectHead + fresh("core", coreBody) + handWritten + strings.TrimPrefix(projectTail, "\n")
	got := readFixture(t, dir, "AGENTS.md")
	if got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	if !strings.HasPrefix(got, projectHead) {
		t.Errorf("the project-owned head changed:\n%q", got)
	}
	if r := res.Regions[0]; r.Target != AgentsFile || !r.Present || r.FileDeleted || r.FileKept {
		t.Errorf("Regions[0] = %+v, want the AGENTS.md region removed in place", r)
	}
	wantManifest(t, dir, "core")
	wantClean(t, dir)
}

// Removing is a decision, not a sync: a hand-edited region still goes, and the
// result says so — while a stale region that stays is left exactly as it was,
// because remove is not sync.
func TestRemoveTakesAHandEditedRegionAndLeavesTheRest(t *testing.T) {
	edited := coreBody + "AND MY OWN LINE\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		"AGENTS.md": projectHead + region("core", testHash(coreBody), edited) + "\n" +
			stale("principles", "old principles\n"),
		GotchasFile: templateBody,
	})

	res := mustRemove(t, mustOpen(t, dir), "core")

	if !res.Regions[0].HandEdited {
		t.Errorf("Regions[0] = %+v, want HandEdited", res.Regions[0])
	}
	want := projectHead + stale("principles", "old principles\n")
	if got := readFixture(t, dir, "AGENTS.md"); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
}

func TestRemoveUnknownOrDisabledModuleIsAnError(t *testing.T) {
	dir := baseRepo(t)
	before := snapshot(t, dir)

	if _, err := mustOpen(t, dir).Remove([]string{"nosuch"}); !errors.Is(err, ErrUnknownModule) {
		t.Errorf("Remove(nosuch) error = %v, want ErrUnknownModule", err)
	}
	_, err := mustOpen(t, dir).Remove([]string{"docs"})
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Remove(docs) error = %v, want ErrNotEnabled", err)
	}
	if !strings.Contains(err.Error(), "docs") {
		t.Errorf("error %q does not name the module", err)
	}
	if got := snapshot(t, dir); !maps.Equal(got, before) {
		t.Errorf("a failed remove wrote something: %v -> %v", before, got)
	}
}

// Removing the last module leaves an empty list, which `agents add` refills:
// the same state `agents source add` leaves when it swaps out the examples.
func TestRemoveEveryModuleLeavesAnEmptyList(t *testing.T) {
	dir := baseRepo(t)

	if _, err := mustOpen(t, dir).Remove([]string{"core", "principles"}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(m.Modules) != 0 {
		t.Errorf("modules = %v, want none", m.Entries())
	}
	if got := readFile(t, filepath.Join(dir, AgentsFile)); strings.Contains(got, "agents:begin") {
		t.Errorf("AGENTS.md still holds a region:\n%s", got)
	}
}

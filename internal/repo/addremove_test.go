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

// An empty manifest is not a manifest the tool can write, so emptying it is a
// refusal rather than a broken repo.
func TestRemoveEveryModuleIsRefused(t *testing.T) {
	dir := baseRepo(t)
	before := snapshot(t, dir)

	_, err := mustOpen(t, dir).Remove([]string{"core", "principles"})
	if !errors.Is(err, ErrNoModulesLeft) {
		t.Fatalf("Remove error = %v, want ErrNoModulesLeft", err)
	}
	if got := snapshot(t, dir); !maps.Equal(got, before) {
		t.Errorf("a refused remove wrote something: %v -> %v", before, got)
	}
}

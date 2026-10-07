package repo

import (
	"maps"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

const (
	// manifestSwapFrom and manifestSwapTo are the same repo before and after a
	// hand edit of .agents.yaml: both carry situational modules, so both
	// render an index, but the inline modules around it differ.
	manifestSwapFrom = "modules:\n  - core\n  - alpha\n  - beta\n"
	manifestSwapTo   = "modules:\n  - principles\n  - stage-build\n  - delegation\n  - alpha\n"
)

// A hand-edited manifest followed by one sync must leave nothing to sync: the
// index region used to keep the slot the old first region held, then move to
// the end on the next run.
func TestOneSyncConvergesAfterManifestSwap(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: manifestSwapFrom})
	mustSync(t, mustOpen(t, dir), false)
	writeFixture(t, dir, manifest.FileName, manifestSwapTo)

	res := mustSync(t, mustOpen(t, dir), false)
	if len(res.Written) == 0 {
		t.Fatal("the swap sync wrote nothing")
	}
	if rep := mustCheck(t, dir); !rep.OK() {
		t.Errorf("check after one sync: %v", problemLines(rep))
	}
	before := snapshot(t, dir)
	if again := mustSync(t, mustOpen(t, dir), false); len(again.Written) != 0 {
		t.Errorf("second sync wrote %v", again.Written)
	}
	if after := snapshot(t, dir); !maps.Equal(before, after) {
		t.Error("second sync changed the repo")
	}
	body := readFixture(t, dir, AgentsFile)
	if strings.Index(body, "agents:begin index") < strings.Index(body, "agents:begin stage-build") {
		t.Errorf("index is not after the inline regions:\n%s", body)
	}
}

// Status has to see everything check sees: a repo whose regions are all fresh
// but misordered used to read stale=0 and fail check.
func TestStatusCountsUnrenderedFiles(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("principles", principlesBody) + "\n" + fresh("core", coreBody),
	})
	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.UnrenderedCount != 1 {
		t.Errorf("UnrenderedCount = %d, want 1", st.UnrenderedCount)
	}
	if st.StaleCount != 0 {
		t.Errorf("StaleCount = %d, want 0", st.StaleCount)
	}
}

// A file a region flag already explains is not also counted as unrendered,
// mirroring check, which prints one line per cause.
func TestStatusUnrenderedIsZeroWhenARegionExplainsIt(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        stale("core", oldCoreBody) + "\n" + fresh("principles", principlesBody),
	})
	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.UnrenderedCount != 0 || st.StaleCount != 1 {
		t.Errorf("unrendered=%d stale=%d, want 0 and 1", st.UnrenderedCount, st.StaleCount)
	}
}

// orphanFixture is a repo whose manifest no longer lists beta, with beta's
// generated file still on disk.
func orphanFixture(betaFile string, extra map[string]string) map[string]string {
	files := map[string]string{
		manifest.FileName: "modules:\n  - core\n  - alpha\n",
		AgentsFile: situationalHead + fresh("core", coreBody) + "\n" +
			fresh(indexRegion, indexBody(alphaRow)),
		alphaPath: fresh("alpha", alphaBody),
		betaPath:  betaFile,
	}
	maps.Copy(files, extra)
	return files
}

func TestOrphanedGeneratedFileIsCountedReportedAndRemoved(t *testing.T) {
	dir := newRepo(t, orphanFixture(fresh("beta", betaBody), nil))

	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.OrphanCount != 1 {
		t.Errorf("OrphanCount = %d, want 1", st.OrphanCount)
	}
	got := problemLines(mustCheck(t, dir))
	if want := []string{betaPath + ": beta orphaned"}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems = %v, want %v", got, want)
	}

	res := mustSync(t, mustOpen(t, dir), false)
	if len(res.Removed) != 1 || res.Removed[0] != betaPath {
		t.Errorf("Removed = %v, want [%s]", res.Removed, betaPath)
	}
	if fileExists(t, dir, betaPath) {
		t.Error("the orphaned file survived sync")
	}
	if rep := mustCheck(t, dir); !rep.OK() {
		t.Errorf("check after sync: %v", problemLines(rep))
	}
}

func TestOrphanedFileKeepsTextTheProjectWrote(t *testing.T) {
	mine := "# Beta, but mine\n\nproject prose\n\n"
	dir := newRepo(t, orphanFixture(mine+fresh("beta", betaBody), nil))

	res := mustSync(t, mustOpen(t, dir), false)
	if len(res.Kept) != 1 || res.Kept[0] != betaPath {
		t.Errorf("Kept = %v, want [%s]", res.Kept, betaPath)
	}
	if len(res.Removed) != 0 {
		t.Errorf("Removed = %v, want none", res.Removed)
	}
	got := readFixture(t, dir, betaPath)
	if !strings.Contains(got, "project prose") || strings.Contains(got, "agents:begin") {
		t.Errorf("file after sync = %q, want the prose and no region", got)
	}
	if rep := mustCheck(t, dir); !rep.OK() {
		t.Errorf("check after sync: %v", problemLines(rep))
	}
}

func TestSyncNeverTouchesAFileWithoutARegion(t *testing.T) {
	plain := "# Plain\n\nno markers here\n"
	dir := newRepo(t, orphanFixture(fresh("beta", betaBody), map[string]string{
		"project/agents/notes.md": plain,
		"project/agents/sub/x.md": plain,
	}))

	res := mustSync(t, mustOpen(t, dir), false)
	for _, rel := range []string{"project/agents/notes.md", "project/agents/sub/x.md"} {
		if got := readFixture(t, dir, rel); got != plain {
			t.Errorf("%s changed to %q", rel, got)
		}
	}
	if len(res.Removed) != 1 {
		t.Errorf("Removed = %v, want only beta", res.Removed)
	}
	if rep := mustCheck(t, dir); !rep.OK() {
		t.Errorf("check: %v", problemLines(rep))
	}
}

// A hand edit inside an orphaned region is refused like any other hand edit
// until --force, so sync never silently discards what someone wrote.
func TestOrphanedFileWithHandEditedRegionRefusesSync(t *testing.T) {
	edited := region("beta", testHash(betaBody), betaBody+"mine\n")
	dir := newRepo(t, orphanFixture(edited, nil))

	res, err := mustOpen(t, dir).Sync(false)
	if err == nil || !res.Refused() {
		t.Fatalf("Sync = %v, %v, want a refusal", res, err)
	}
	if !fileExists(t, dir, betaPath) {
		t.Error("the file was removed despite the refusal")
	}
	if _, err := mustOpen(t, dir).Sync(true); err != nil {
		t.Fatalf("forced sync: %v", err)
	}
	if fileExists(t, dir, betaPath) {
		t.Error("forced sync left the file")
	}
}

// The current file modules' own targets are never orphans, even though the
// scan walks the same directory.
func TestOrphanScanSkipsCurrentTargets(t *testing.T) {
	dir := newRepo(t, orphanFixture(fresh("beta", betaBody), nil))
	res := mustSync(t, mustOpen(t, dir), false)
	if !fileExists(t, dir, alphaPath) {
		t.Errorf("alpha's file was removed; Removed = %v", res.Removed)
	}
}

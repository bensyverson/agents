package repo

import (
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

// problemLines renders a report the way the CLI prints it, so the tests pin
// the wording a hook's user actually reads.
func problemLines(rep CheckReport) []string {
	out := make([]string, 0, len(rep.Problems))
	for _, p := range rep.Problems {
		out = append(out, p.String())
	}
	return out
}

func mustCheck(t *testing.T, dir string) CheckReport {
	t.Helper()
	rep, err := mustOpen(t, dir).Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return rep
}

func TestCheckCleanRepoHasNoProblems(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		AgentsFile:        "# Head\n\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		delegationPath:    fresh("delegation", delegationBody),
	})

	rep := mustCheck(t, dir)
	if !rep.OK() {
		t.Errorf("OK = false on a clean repo: %v", problemLines(rep))
	}
	if rep.Dir != dir {
		t.Errorf("Dir = %q, want %q", rep.Dir, dir)
	}
}

func TestCheckReportsStaleEditedMissingAndOrphan(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		AgentsFile: "# Head\n\n" +
			stale("core", oldCoreBody) + "\n" +
			region("principles", testHash(principlesBody), principlesBody+"mine\n") + "\n" +
			region("docs", testHash("orphan\n"), "orphan\n"),
		// delegation's file is absent, so its region is missing.
	})

	rep := mustCheck(t, dir)
	if rep.OK() {
		t.Fatal("OK = true on a stale, hand-edited, orphaned and missing repo")
	}
	got := problemLines(rep)
	want := []string{
		"AGENTS.md: core stale",
		"AGENTS.md: principles hand-edited",
		"AGENTS.md: docs orphaned",
		"project/agents/delegation.md: delegation missing",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A region can be stale and hand-edited at once; sync rewrites for one and
// refuses for the other, so check must name both.
func TestCheckReportsStaleAndEditedSeparately(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        region("core", "000000", coreBody+"mine\n") + "\n" + fresh("principles", principlesBody),
	})

	got := problemLines(mustCheck(t, dir))
	want := []string{"AGENTS.md: core stale", "AGENTS.md: core hand-edited"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems = %v, want %v", got, want)
	}
}

// check is a pre-commit gate, so it must fail whenever sync would write —
// including a document whose regions are all fresh but in the wrong order,
// which no per-region flag catches.
func TestCheckFailsWhenSyncWouldStillWrite(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("principles", principlesBody) + "\n" + fresh("core", coreBody),
	})

	rep := mustCheck(t, dir)
	if rep.OK() {
		t.Fatal("OK = true on a document sync would rewrite")
	}
	if got, want := problemLines(rep), []string{"AGENTS.md: not what the modules render"}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems = %v, want %v", got, want)
	}
}

// Every failing check corresponds to a write sync would make, and vice versa:
// the hook's promise is "committing this leaves nothing to sync".
func TestCheckAgreesWithSync(t *testing.T) {
	cases := map[string]map[string]string{
		"clean": {
			manifest.FileName: manifestInline,
			AgentsFile:        fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
		},
		"stale": {
			manifest.FileName: manifestInline,
			AgentsFile:        stale("core", oldCoreBody) + "\n" + fresh("principles", principlesBody),
		},
		"edited": {
			manifest.FileName: manifestInline,
			AgentsFile:        region("core", testHash(coreBody), coreBody+"mine\n") + "\n" + fresh("principles", principlesBody),
		},
		"unrendered": {
			manifest.FileName: manifestInline,
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t, files)
			rep, err := mustOpen(t, dir).Inspect()
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if got, want := !mustCheck(t, dir).OK(), rep.NeedsWrite(); got != want {
				t.Errorf("check fails = %v, but sync would write = %v", got, want)
			}
		})
	}
}

// The two questions the CLI asks a report, to pick the advice it prints.
func TestCheckReportSeparatesEditsFromSyncableProblems(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        stale("core", oldCoreBody) + "\n" + fresh("principles", principlesBody),
	})
	rep := mustCheck(t, dir)
	if rep.HasEdits() {
		t.Errorf("HasEdits = true with only a stale region: %v", problemLines(rep))
	}
	if !rep.NeedsSync() {
		t.Errorf("NeedsSync = false with a stale region: %v", problemLines(rep))
	}

	// A hand edit on its own is not something sync will fix.
	dir = newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        region("core", testHash(coreBody), coreBody+"mine\n") + "\n" + fresh("principles", principlesBody),
	})
	rep = mustCheck(t, dir)
	if !rep.HasEdits() {
		t.Errorf("HasEdits = false with a hand-edited region: %v", problemLines(rep))
	}
	if rep.NeedsSync() {
		t.Errorf("NeedsSync = true with only a hand edit: %v", problemLines(rep))
	}
}

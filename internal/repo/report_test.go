package repo

import (
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/agents/internal/manifest"
)

const gotchasContent = `# Gotchas

preamble

---

## 2026-07-01 — the build is slow

It just is.

## 2026-08-01 — rule: TDD wording is ambiguous

The rule says X but means Y.
`

func TestDiffReportsHandEditsAndRules(t *testing.T) {
	edited := coreBody + "AND MY OWN LINE\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		AgentsFile:        "# Head\n\n" + region("core", testHash(coreBody), edited) + "\n" + fresh("principles", principlesBody),
		delegationPath:    region("delegation", testHash(delegationBody), delegationBody+"agent note\n"),
		GotchasFile:       gotchasContent,
	})

	rep, err := mustOpen(t, dir).Diff()
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if rep.Empty() {
		t.Fatal("Empty = true with hand edits present")
	}
	if len(rep.Edits) != 2 {
		t.Fatalf("Edits = %+v, want one per hand-edited region", rep.Edits)
	}
	first := rep.Edits[0]
	if first.Target != AgentsFile || first.Region != "core" {
		t.Errorf("Edits[0] = %s: %s, want %s: core", first.Target, first.Region, AgentsFile)
	}
	// The module is the "a" side, so what the agent wrote shows up as additions.
	if !strings.Contains(first.Diff, "+AND MY OWN LINE") {
		t.Errorf("diff should show the hand-written line as an addition:\n%s", first.Diff)
	}
	if rep.Edits[1].Target != delegationPath {
		t.Errorf("Edits[1].Target = %s, want %s", rep.Edits[1].Target, delegationPath)
	}

	if len(rep.Rules) != 1 {
		t.Fatalf("Rules = %+v, want the one rule: entry", rep.Rules)
	}
	if !strings.HasPrefix(rep.Rules[0].Headline, "rule:") {
		t.Errorf("Rules[0].Headline = %q, want the rule: headline", rep.Rules[0].Headline)
	}
}

func TestDiffEmptyOnCleanRepo(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
	})

	rep, err := mustOpen(t, dir).Diff()
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !rep.Empty() {
		t.Errorf("Empty = false: %+v", rep)
	}
}

func TestStatusCounts(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		AgentsFile: "# Head\n\n" +
			stale("core", oldCoreBody) + "\n" + // stale only
			region("principles", testHash(principlesBody), principlesBody+"mine\n") + "\n" + // edited only
			region("docs", testHash("orphan\n"), "orphan\n"), // orphan
		GotchasFile: gotchasContent,
	})

	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got, want := strings.Join(st.Modules, ","), "core,principles,delegation"; got != want {
		t.Errorf("Modules = %q, want %q", got, want)
	}
	checkCount(t, "stale", st.StaleCount, 1)
	checkCount(t, "edited", st.EditedCount, 1)
	checkCount(t, "orphan", st.OrphanCount, 1)
	// delegation's file does not exist, so its one region is missing.
	checkCount(t, "missing", st.MissingCount, 1)
	checkCount(t, "gotchas", st.GotchaCount, 2)
	if !st.HasGotchasFile {
		t.Error("HasGotchasFile = false")
	}
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !st.HasOldestGotcha || !st.OldestGotcha.Equal(want) {
		t.Errorf("OldestGotcha = %v (%v), want %v", st.OldestGotcha, st.HasOldestGotcha, want)
	}
}

// A region can be stale and hand-edited at once, and both matter.
func TestStatusCountsStaleAndEditedDistinctly(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        region("core", "000000", coreBody+"mine\n") + "\n" + fresh("principles", principlesBody),
	})

	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	checkCount(t, "stale", st.StaleCount, 1)
	checkCount(t, "edited", st.EditedCount, 1)
}

func TestStatusWithoutGotchasFile(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
	})

	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.HasGotchasFile {
		t.Error("HasGotchasFile = true with no file")
	}
	if st.HasOldestGotcha || st.GotchaCount != 0 {
		t.Errorf("gotchas = %d, oldest ok = %v; want none", st.GotchaCount, st.HasOldestGotcha)
	}
	checkCount(t, "stale", st.StaleCount, 0)
	checkCount(t, "edited", st.EditedCount, 0)
	if st.Dir != dir {
		t.Errorf("Dir = %q, want %q", st.Dir, dir)
	}
}

func TestInspectAggregates(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestAll,
		AgentsFile:        stale("core", oldCoreBody) + "\n" + fresh("principles", principlesBody),
	})

	rep, err := mustOpen(t, dir).Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(rep.Targets) != 2 {
		t.Fatalf("Targets = %v, want 2", targetPaths(rep.Targets))
	}
	if !rep.NeedsWrite() {
		t.Error("NeedsWrite = false with a stale region and a missing file")
	}
	if rep.AnyEdited() {
		t.Error("AnyEdited = true with no hand edits")
	}
}

func checkCount(t *testing.T, what string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s count = %d, want %d", what, got, want)
	}
}

// The head is the project's own rules; status reports its size.
func TestStatusCountsHeadLines(t *testing.T) {
	head := "# Project\n\nOne local rule.\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        head + "\n" + fresh("core", coreBody) + "\n" + fresh("principles", principlesBody),
	})

	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	checkCount(t, "head lines", st.HeadLines, 3)
}

// A repo with no head, no regions or no AGENTS.md at all reports zero rather
// than counting the renderer's own separator or a missing file.
func TestStatusHeadLinesZero(t *testing.T) {
	// A file with no regions is ALL head — the largest possible one — so it
	// must not read as an empty head: that is the pre-migration case the
	// gauge exists to show.
	cases := map[string]struct {
		content string
		want    int
	}{
		"no head":    {fresh("core", coreBody) + "\n" + fresh("principles", principlesBody), 0},
		"no regions": {"# Project\n\nAll ours, nothing generated.\n", 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t, map[string]string{
				manifest.FileName: manifestInline,
				AgentsFile:        tc.content,
			})
			st, err := mustOpen(t, dir).Status()
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			checkCount(t, "head lines", st.HeadLines, tc.want)
		})
	}

	t.Run("no file", func(t *testing.T) {
		dir := newRepo(t, map[string]string{manifest.FileName: manifestInline})
		st, err := mustOpen(t, dir).Status()
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		checkCount(t, "head lines", st.HeadLines, 0)
	})
}

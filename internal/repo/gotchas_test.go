package repo

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/gotchas"
	"github.com/bensyverson/agents/internal/manifest"
)

// reseedTemplate has the shape the shipped template has: prose, then the rule
// that separates the preamble from the entries.
const reseedTemplate = `# Gotchas

Traps that cost real time.

**Delete anything that becomes obvious.** Keep this list short.

---
`

func reseedTemplates() fstest.MapFS {
	return fstest.MapFS{GotchasFile: &fstest.MapFile{Data: []byte(reseedTemplate)}}
}

// bigGotchas builds a gotchas file with n entries.
func bigGotchas(n int) string {
	var b strings.Builder
	b.WriteString("# Gotchas\n\npreamble\n\n---\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "\n## 2026-08-%02d — entry %d\n\nbody\n", (i%28)+1, i)
	}
	return b.String()
}

func TestStatusCountsGotchaLines(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
		GotchasFile:       gotchasContent,
	})
	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if want := strings.Count(gotchasContent, "\n"); st.GotchaLines != want {
		t.Errorf("GotchaLines = %d, want %d", st.GotchaLines, want)
	}
	if st.GotchasOverBudget {
		t.Errorf("GotchasOverBudget = true for %d entries / %d lines",
			st.GotchaCount, st.GotchaLines)
	}
}

func TestStatusFlagsOverBudgetGotchas(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
		GotchasFile:       bigGotchas(gotchas.BudgetEntries + 1),
	})
	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.GotchaCount != gotchas.BudgetEntries+1 {
		t.Fatalf("GotchaCount = %d, want %d", st.GotchaCount, gotchas.BudgetEntries+1)
	}
	if !st.GotchasOverBudget {
		t.Errorf("GotchasOverBudget = false at %d entries", st.GotchaCount)
	}
}

// A repo with no gotchas file at all is not over budget; the count is zero and
// there is nothing to warn about.
func TestStatusWithoutGotchasFileIsNotOverBudget(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
	})
	st, err := mustOpen(t, dir).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.GotchasOverBudget || st.GotchaLines != 0 {
		t.Errorf("lines = %d, over = %v; want 0, false", st.GotchaLines, st.GotchasOverBudget)
	}
}

func TestReseedGotchasReplacesAnOldPreamble(t *testing.T) {
	const old = "# Gotchas\n\nold header\n\n---\n\n## 2026-07-27 — an entry\n\nbody\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
		GotchasFile:       old,
	})
	r := mustOpen(t, dir)

	changed, err := r.ReseedGotchas(reseedTemplates())
	if err != nil {
		t.Fatalf("ReseedGotchas: %v", err)
	}
	if !changed {
		t.Fatal("changed = false over a stale preamble")
	}
	got := readFixture(t, dir, GotchasFile)
	if !strings.HasPrefix(got, reseedTemplate) {
		t.Errorf("preamble was not replaced:\n%q", got)
	}
	if !strings.HasSuffix(got, "## 2026-07-27 — an entry\n\nbody\n") {
		t.Errorf("entries were not preserved:\n%q", got)
	}

	// Re-running is a no-op, and writes nothing.
	before := snapshot(t, dir)
	changed, err = r.ReseedGotchas(reseedTemplates())
	if err != nil {
		t.Fatalf("second ReseedGotchas: %v", err)
	}
	if changed {
		t.Error("changed = true on a file that already matches the template")
	}
	if after := snapshot(t, dir); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Error("the second reseed changed bytes on disk")
	}
}

// Reseeding never creates a gotchas file: seeding a missing one is init's job.
func TestReseedGotchasWithoutAFile(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
	})
	changed, err := mustOpen(t, dir).ReseedGotchas(reseedTemplates())
	if err != nil {
		t.Fatalf("ReseedGotchas: %v", err)
	}
	if changed {
		t.Error("changed = true with no gotchas file")
	}
	if fileExists(t, dir, GotchasFile) {
		t.Error("ReseedGotchas created a gotchas file")
	}
}

func TestReseedGotchasWithoutTemplates(t *testing.T) {
	const content = "# Gotchas\n\nours\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
		GotchasFile:       content,
	})
	changed, err := mustOpen(t, dir).ReseedGotchas(nil)
	if err != nil {
		t.Fatalf("ReseedGotchas(nil): %v", err)
	}
	if changed {
		t.Error("changed = true with no template FS")
	}
	if got := readFixture(t, dir, GotchasFile); got != content {
		t.Errorf("%s = %q, want it untouched", GotchasFile, got)
	}
}

// Gotchas is the one read of the repo's gotchas file that status, diff and
// sync all share.
func TestRepoGotchas(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: manifestInline,
		AgentsFile:        fresh("core", coreBody) + fresh("principles", principlesBody),
		GotchasFile:       gotchasContent,
	})
	f, exists, err := mustOpen(t, dir).Gotchas()
	if err != nil || !exists {
		t.Fatalf("Gotchas = (_, %v, %v)", exists, err)
	}
	if len(f.Entries) != 2 {
		t.Errorf("Entries = %d, want 2", len(f.Entries))
	}
	if want := strings.Count(gotchasContent, "\n"); f.Lines != want {
		t.Errorf("Lines = %d, want %d", f.Lines, want)
	}
}

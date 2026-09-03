package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/gotchas"
)

// writeGotchas replaces the repo's seeded gotchas file.
func writeGotchas(t *testing.T, repoDir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "project", "gotchas.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write gotchas: %v", err)
	}
}

// overBudgetGotchas is a file one entry past the budget.
func overBudgetGotchas() string {
	var b strings.Builder
	b.WriteString("# Gotchas\n\npreamble\n\n---\n")
	for i := 1; i <= gotchas.BudgetEntries+1; i++ {
		fmt.Fprintf(&b, "\n## 2026-08-%02d — entry %d\n\nbody\n", (i%28)+1, i)
	}
	return b.String()
}

// wantWarning is the warning both verbs print, built from the same constants
// the code quotes, so the numbers can never drift from the bounds.
func wantWarning(entries, lines int) string {
	return fmt.Sprintf("gotchas.md: %d entries, %d lines (budget %d/%d) — "+
		"retire entries that are fixed or obvious; promote general ones to a module "+
		"(`agents diff` lists the `rule:` candidates)",
		entries, lines, gotchas.BudgetEntries, gotchas.BudgetLines)
}

func TestStatusWarnsOnlyOverBudget(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)

	stdout, _, err := run(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.Contains(stdout, "budget") {
		t.Errorf("status warned about a freshly seeded gotchas file:\n%s", stdout)
	}

	content := overBudgetGotchas()
	writeGotchas(t, repoDir, content)
	stdout, _, err = run(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	want := wantWarning(gotchas.BudgetEntries+1, strings.Count(content, "\n"))
	if !strings.Contains(stdout, want) {
		t.Errorf("status output missing\n%s\ngot:\n%s", want, stdout)
	}
	// The warning is appended to the repo's own line, not printed under it.
	if lines := strings.Count(strings.TrimSpace(stdout), "\n"); lines != 0 {
		t.Errorf("status printed more than one line:\n%s", stdout)
	}
	if !strings.Contains(stdout, repoDir) {
		t.Errorf("the warning is not on the repo's line:\n%s", stdout)
	}
}

func TestSyncWarnsAfterItsRenderedLines(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	content := overBudgetGotchas()
	writeGotchas(t, repoDir, content)

	// Nothing stale: the nag still fires, because sync is the moment the rules
	// are in front of someone.
	stdout, _, err := run(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	want := wantWarning(gotchas.BudgetEntries+1, strings.Count(content, "\n"))
	if !strings.Contains(stdout, want) {
		t.Errorf("sync output missing\n%s\ngot:\n%s", want, stdout)
	}
	if got := strings.Count(stdout, "budget"); got != 1 {
		t.Errorf("warning printed %d times, want once:\n%s", got, stdout)
	}

	// With something stale, the warning follows the rendered lines.
	if err := os.WriteFile(filepath.Join(sourceDir, "modules", "core.md"), []byte("newer core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = run(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	rendered, warning := strings.Index(stdout, "rendered AGENTS.md"), strings.Index(stdout, "budget")
	if rendered < 0 || warning < 0 || rendered > warning {
		t.Errorf("want the rendered line before the warning:\n%s", stdout)
	}
	// Warning only: the exit status is untouched.
	if err != nil {
		t.Errorf("sync returned %v; the warning must not fail the command", err)
	}
}

func TestSyncAllPrefixesTheWarningWithTheDir(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	writeGotchas(t, repoDir, overBudgetGotchas())

	stdout, _, err := run(t, "sync", "--all")
	if err != nil {
		t.Fatalf("sync --all: %v", err)
	}
	if !strings.Contains(stdout, repoDir+": gotchas.md: ") {
		t.Errorf("warning is not prefixed with the repo dir:\n%s", stdout)
	}
}

func TestSyncReseedGotchas(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)

	// The seeded file is the template: reseeding it prints nothing.
	stdout, _, err := run(t, "sync", "--reseed-gotchas")
	if err != nil {
		t.Fatalf("sync --reseed-gotchas: %v", err)
	}
	if stdout != "" {
		t.Errorf("reseeding an up-to-date file printed %q, want silence", stdout)
	}

	const entry = "## 2026-07-27 — the oldest entry\n\nbody\n"
	writeGotchas(t, repoDir, "# Gotchas\n\nold two-line header.\n\n---\n\n"+entry)
	stdout, _, err = run(t, "sync", "--reseed-gotchas")
	if err != nil {
		t.Fatalf("sync --reseed-gotchas: %v", err)
	}
	if !strings.Contains(stdout, "reseeded project/gotchas.md") {
		t.Errorf("sync did not report the reseed:\n%s", stdout)
	}
	got, err := os.ReadFile(filepath.Join(repoDir, "project", "gotchas.md"))
	if err != nil {
		t.Fatalf("read gotchas: %v", err)
	}
	if strings.Contains(string(got), "old two-line header") {
		t.Errorf("the old preamble survived:\n%s", got)
	}
	if !strings.HasPrefix(string(got), templatePreamble) {
		t.Errorf("the template preamble was not installed:\n%s", got)
	}
	if !strings.HasSuffix(string(got), entry) {
		t.Errorf("the entry was not preserved:\n%s", got)
	}

	// Without the flag, sync never touches the file.
	writeGotchas(t, repoDir, "# Gotchas\n\nold two-line header.\n\n---\n\n"+entry)
	stdout, _, err = run(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if strings.Contains(stdout, "reseeded") {
		t.Errorf("sync reseeded without the flag:\n%s", stdout)
	}
	after, _ := os.ReadFile(filepath.Join(repoDir, "project", "gotchas.md"))
	if !strings.Contains(string(after), "old two-line header") {
		t.Errorf("sync rewrote the gotchas file without the flag:\n%s", after)
	}
}

// A file with no rule at all — the pre-template shape — gets the preamble and a
// rule above everything it holds.
func TestSyncReseedGotchasWithNoRule(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	const old = "# Gotchas\n\nno rule anywhere.\n\n## 2026-07-27 — the oldest entry\n\nbody\n"
	writeGotchas(t, repoDir, old)

	stdout, _, err := run(t, "sync", "--reseed-gotchas")
	if err != nil {
		t.Fatalf("sync --reseed-gotchas: %v", err)
	}
	if !strings.Contains(stdout, "reseeded project/gotchas.md") {
		t.Errorf("sync did not report the reseed:\n%s", stdout)
	}
	got, err := os.ReadFile(filepath.Join(repoDir, "project", "gotchas.md"))
	if err != nil {
		t.Fatalf("read gotchas: %v", err)
	}
	if !strings.Contains(string(got), "\n---\n") {
		t.Errorf("no rule line was added:\n%s", got)
	}
	if !strings.HasSuffix(string(got), old) {
		t.Errorf("the old file is not preserved below the preamble:\n%s", got)
	}
	if !strings.HasPrefix(string(got), templatePreamble) {
		t.Errorf("the template preamble was not installed:\n%s", got)
	}
}

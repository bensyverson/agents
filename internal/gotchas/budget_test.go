package gotchas

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCountsLines(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"empty file", "", 0},
		{"one unterminated line", "a", 1},
		{"one terminated line", "a\n", 1},
		{"two unterminated", "a\nb", 2},
		{"two terminated", "a\nb\n", 2},
		{"a blank line counts", "a\n\n", 2},
		{"only a newline", "\n", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Parse(c.content).Lines; got != c.want {
				t.Errorf("Parse(%q).Lines = %d, want %d", c.content, got, c.want)
			}
		})
	}
}

// The line count is the whole file's, preamble included: the budget is about
// how much an agent reads, not how much of it is entries.
func TestParseCountsPreambleLines(t *testing.T) {
	f := Parse(docTwoEntries)
	if len(f.Entries) != 2 {
		t.Fatalf("Entries = %d, want 2", len(f.Entries))
	}
	want := strings.Count(docTwoEntries, "\n")
	if f.Lines != want {
		t.Errorf("Lines = %d, want %d (the whole file)", f.Lines, want)
	}
}

func TestReadReportsLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gotchas.md")
	if err := os.WriteFile(path, []byte(docTwoEntries), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, exists, err := Read(path)
	if err != nil || !exists {
		t.Fatalf("Read = (_, %v, %v)", exists, err)
	}
	if f.Lines != strings.Count(docTwoEntries, "\n") {
		t.Errorf("Lines = %d, want %d", f.Lines, strings.Count(docTwoEntries, "\n"))
	}
	if len(f.Entries) != 2 {
		t.Errorf("Entries = %d, want 2", len(f.Entries))
	}
}

// The budget bounds are the numbers the warning quotes, so pin them here: a
// file at the bound is fine and one past it is not.
func TestOverBudget(t *testing.T) {
	cases := []struct {
		name           string
		entries, lines int
		want           bool
	}{
		{"empty", 0, 0, false},
		{"at both bounds", 15, 150, false},
		{"one entry past", 16, 10, true},
		{"one line past", 2, 151, true},
		{"past both", 47, 622, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := OverBudget(c.entries, c.lines); got != c.want {
				t.Errorf("OverBudget(%d, %d) = %v, want %v", c.entries, c.lines, got, c.want)
			}
		})
	}
	if BudgetEntries != 15 || BudgetLines != 150 {
		t.Errorf("budget = %d/%d, want 15/150", BudgetEntries, BudgetLines)
	}
}

// File.OverBudget must agree with the package function on its own numbers,
// so a caller may use either.
func TestFileOverBudget(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Gotchas\n\n---\n")
	for i := 1; i <= BudgetEntries; i++ {
		fmt.Fprintf(&b, "\n## 2026-08-%02d — entry %d\n\nbody\n", i, i)
	}
	within := b.String()
	if f := Parse(within); f.OverBudget() {
		t.Errorf("OverBudget = true at %d entries / %d lines", len(f.Entries), f.Lines)
	}
	fmt.Fprintf(&b, "\n## 2026-08-%02d — one too many\n\nbody\n", BudgetEntries+1)
	f := Parse(b.String())
	if len(f.Entries) != BudgetEntries+1 {
		t.Fatalf("Entries = %d, want %d", len(f.Entries), BudgetEntries+1)
	}
	if !f.OverBudget() {
		t.Errorf("OverBudget = false at %d entries", len(f.Entries))
	}
}

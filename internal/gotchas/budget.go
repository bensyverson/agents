package gotchas

import "strings"

// The size a gotchas file is expected to stay inside. Past either bound the
// tool nags on every `status` and every `sync`: appending has a trigger —
// something just cost you time — and deleting has none, so the count has to be
// put in front of someone. They are advisory; nothing ever refuses over them.
const (
	// BudgetEntries is the most dated entries a file should carry.
	BudgetEntries int = 15
	// BudgetLines is the most lines it should occupy, preamble included: the
	// budget is about how much an agent reads at session start.
	BudgetLines int = 150
)

// File is a parsed gotchas file: its entries and its size.
type File struct {
	Entries Entries
	Lines   int
}

// OverBudget reports whether a file of this size has outgrown either bound.
func OverBudget(entries, lines int) bool {
	return entries > BudgetEntries || lines > BudgetLines
}

// OverBudget reports whether this file has outgrown either bound.
func (f File) OverBudget() bool { return OverBudget(len(f.Entries), f.Lines) }

// countLines counts lines the way `wc -l` does when the file ends in a
// newline, and still counts an unterminated last line.
func countLines(content string) int {
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

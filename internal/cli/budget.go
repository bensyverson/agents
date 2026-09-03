package cli

import (
	"fmt"
	"io"

	"github.com/bensyverson/agents/internal/gotchas"
	"github.com/bensyverson/agents/internal/repo"
)

// budgetWarning is the nag `status` appends to a repo's line and `sync` prints
// under it, or "" when the file is inside the budget. The numbers come from
// the same constants the bounds do, so the advice can never quote a budget the
// tool is not enforcing.
func budgetWarning(entries, lines int) string {
	if !gotchas.OverBudget(entries, lines) {
		return ""
	}
	return fmt.Sprintf("gotchas.md: %d entries, %d lines (budget %d/%d) — "+
		"retire entries that are fixed or obvious; promote general ones to a module "+
		"(`agents diff` lists the `rule:` candidates)",
		entries, lines, gotchas.BudgetEntries, gotchas.BudgetLines)
}

// prefixed labels a line with the repo it came from, which only `--all` needs.
func prefixed(all bool, dir, line string) string {
	if all {
		return dir + ": " + line
	}
	return line
}

// gotchasReport is sync's pass over the repo's gotchas file: refresh the
// preamble when asked, then warn if the file has outgrown its budget. The
// reseed comes first so the warning counts what the file now holds. Both are
// advisory — neither changes sync's exit status.
func gotchasReport(out io.Writer, r *repo.Repo, reseed, all bool) error {
	if reseed {
		changed, err := r.ReseedGotchas()
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintln(out, prefixed(all, r.Dir, "reseeded "+repo.GotchasFile))
		}
	}
	file, exists, err := r.Gotchas()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if warning := budgetWarning(len(file.Entries), file.Lines); warning != "" {
		fmt.Fprintln(out, prefixed(all, r.Dir, warning))
	}
	return nil
}

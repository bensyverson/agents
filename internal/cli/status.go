package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/repo"
)

// Column padding for the aligned one-line-per-repo output.
const statusPadding = 2

func statusCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "One line per repo: modules, stale and hand-edited regions, gotchas",
		Long: "stale = the module moved since the region was rendered; edited = the region\n" +
			"was changed by hand since. A region can be both, and is counted in both.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			dirs, err := repoDirs(all)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, statusPadding, ' ', 0)
			for _, dir := range dirs {
				r, err := repo.Open(dir, opts)
				if err != nil {
					// Without --all this is the user's mistake; with it, a
					// registry entry that has rotted is still worth a line.
					if !all {
						return err
					}
					fmt.Fprintf(w, "%s\t%s\n", dir, unmanagedReason(err))
					continue
				}
				st, err := r.Status()
				if err != nil {
					return err
				}
				fmt.Fprintln(w, statusLine(st))
				if hint, ok := repo.HookHint(dir); ok {
					fmt.Fprintln(cmd.ErrOrStderr(), hint)
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

func unmanagedReason(err error) string {
	switch {
	case errors.Is(err, repo.ErrMissingDir):
		return "(missing)"
	case errors.Is(err, repo.ErrNotManaged):
		return "(not managed)"
	default:
		return fmt.Sprintf("(%v)", err)
	}
}

// statusLine is one repo's tab-separated columns; the caller aligns them.
func statusLine(st repo.Status) string {
	fields := []string{
		st.Dir,
		"modules=" + strings.Join(st.Modules, ","),
		// The head is where project-specific rules live: the balloon gauge.
		fmt.Sprintf("head=%d", st.HeadLines),
		fmt.Sprintf("stale=%d", st.StaleCount),
		fmt.Sprintf("edited=%d", st.EditedCount),
		fmt.Sprintf("missing=%d", st.MissingCount),
		fmt.Sprintf("orphans=%d", st.OrphanCount),
		gotchaField(st),
	}
	// The nag rides on the repo's own line: one line per repo is the contract.
	if warning := budgetWarning(st.GotchaCount, st.GotchaLines); warning != "" {
		fields = append(fields, warning)
	}
	return strings.Join(fields, "\t")
}

// gotchaField distinguishes a repo with no gotchas file from one whose file is
// simply empty: the first has not been initialised, the second is just tidy.
func gotchaField(st repo.Status) string {
	if !st.HasGotchasFile {
		return "gotchas=none"
	}
	if !st.HasOldestGotcha {
		return "gotchas=0"
	}
	return fmt.Sprintf("gotchas=%d (oldest %s)", st.GotchaCount, st.OldestGotcha.Format(dateLayout))
}

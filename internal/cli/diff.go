package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/repo"
)

// dateLayout is how a gotcha's date is printed.
const dateLayout = "2006-01-02"

func diffCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show hand-edits inside generated regions, and rule: gotchas",
		Long: "The review queue for the shared rules: what agents changed inside a region\n" +
			"(\"+\" lines are theirs) and every `rule:` entry in " + repo.GotchasFile + ".\n" +
			"Silent when there is nothing to review.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			return forEachRepo(cmd, opts, all, func(r *repo.Repo) error {
				rep, err := r.Diff()
				if err != nil {
					return err
				}
				printDiff(cmd.OutOrStdout(), rep, all)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

func printDiff(out io.Writer, rep repo.DiffReport, withHeader bool) {
	if rep.Empty() {
		return
	}
	if withHeader {
		fmt.Fprintln(out, rep.Dir)
	}
	for _, edit := range rep.Edits {
		fmt.Fprintf(out, "%s: %s\n", edit.Target, edit.Region)
		fmt.Fprint(out, edit.Diff)
	}
	if len(rep.Rules) > 0 {
		fmt.Fprintln(out, "gotchas: rule entries:")
		for _, entry := range rep.Rules {
			fmt.Fprintf(out, "  %s  %s\n", entry.Date.Format(dateLayout), entry.Headline)
			// The body, indented, so the review needs no second file open.
			for line := range strings.SplitSeq(entry.Body, "\n") {
				fmt.Fprintf(out, "    %s\n", line)
			}
		}
	}
	if withHeader {
		fmt.Fprintln(out)
	}
}

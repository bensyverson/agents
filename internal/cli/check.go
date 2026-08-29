package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/repo"
)

// The advice a failing check ends with: what the user does next depends on
// whether anything was hand-edited, because sync will not touch those.
const (
	adviceSync = "out of date: run `agents sync`"
	adviceDiff = "hand-edited: review with `agents diff`, then `agents sync --force` to overwrite"
	adviceBoth = "out of date: review the hand edits with `agents diff`, then run `agents sync`"
)

func checkCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Exit non-zero when a region is stale, hand-edited or missing",
		Long: "Silent and exit 0 when every region is current and unedited and no generated\n" +
			"file is missing; otherwise one line per problem and exit 1. It writes nothing,\n" +
			"which makes it a pre-commit line:\n\n" +
			"    agents check || exit 1\n\n" +
			"With --all every registered repo is checked and each line names its repo.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			set, err := moduleSet(cmd)
			if err != nil {
				return err
			}
			var failed []repo.CheckReport
			err = forEachRepo(cmd, set, all, func(r *repo.Repo) error {
				rep, err := r.Check()
				if err != nil {
					return err
				}
				if rep.OK() {
					return nil
				}
				failed = append(failed, rep)
				for _, p := range rep.Problems {
					if all {
						fmt.Fprintln(cmd.OutOrStdout(), r.Dir+": "+p.String())
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), p.String())
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			if len(failed) == 0 {
				return nil
			}
			return errors.New(advice(failed))
		},
	}
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

// advice names the one verb that resolves what was found, so the hook's
// message is an instruction rather than a verdict.
func advice(failed []repo.CheckReport) string {
	edits, syncable := false, false
	for _, rep := range failed {
		edits = edits || rep.HasEdits()
		syncable = syncable || rep.NeedsSync()
	}
	switch {
	case edits && syncable:
		return adviceBoth
	case edits:
		return adviceDiff
	default:
		return adviceSync
	}
}

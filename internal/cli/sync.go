package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/repo"
)

const (
	forceFlag = "force"
	// reseedGotchasFlag refreshes each repo's gotchas preamble from the
	// template. Opt-in: it rewrites a project-owned file.
	reseedGotchasFlag = "reseed-gotchas"
)

// errRefused is what sync returns after printing the blocking diffs: the
// message is the advice, and cobra turns a non-nil error into exit 1.
var errRefused = errors.New("refusing to sync; review with `agents diff` or re-run with --force")

func syncCmd() *cobra.Command {
	var force, all, reseedGotchas bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-render every stale region and generated file",
		Long: "Silent and exit 0 when nothing is stale. A region edited by hand since its\n" +
			"last render stops that repo's sync — no file is written — unless --force.\n" +
			"With --all every registered repo is visited; refusals are reported under the\n" +
			"repo's path and the exit status is non-zero if any repo refused.\n\n" +
			"A repo whose " + repo.GotchasFile + " has outgrown its budget gets a warning line\n" +
			"after its rendered lines; the warning never changes the exit status.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			set, err := moduleSet(cmd)
			if err != nil {
				return err
			}
			refused := false
			err = forEachRepo(cmd, set, all, func(r *repo.Repo) error {
				// sync, like init, creates any seed the repo is missing.
				r.Templates = agents.Templates()
				res, err := r.Sync(force)
				if errors.Is(err, repo.ErrHandEdited) {
					if !all {
						printRefusal(cmd.OutOrStdout(), res)
						return errRefused
					}
					refused = true
					fmt.Fprintln(cmd.OutOrStdout(), r.Dir)
					printRefusal(cmd.OutOrStdout(), res)
					fmt.Fprintln(cmd.OutOrStdout())
					return nil
				}
				if err != nil {
					return err
				}
				for _, path := range res.Written {
					if all {
						fmt.Fprintln(cmd.OutOrStdout(), r.Dir+": rendered "+path)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "rendered "+path)
					}
				}
				for _, path := range res.Seeded {
					if all {
						fmt.Fprintln(cmd.OutOrStdout(), r.Dir+": seeded "+path)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "seeded "+path)
					}
				}
				return gotchasReport(cmd.OutOrStdout(), r, reseedGotchas, all)
			})
			if err != nil {
				return err
			}
			if refused {
				return errRefused
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, forceFlag, false, "overwrite hand-edited regions")
	cmd.Flags().BoolVar(&reseedGotchas, reseedGotchasFlag, false,
		"replace "+repo.GotchasFile+"'s preamble with the template's, keeping every entry")
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

// printRefusal shows what --force would throw away: "-" lines are the hand
// edits, "+" lines what the module says now.
func printRefusal(out io.Writer, res repo.SyncResult) {
	for _, edit := range res.Edited {
		fmt.Fprintf(out, "%s: %s (hand-edited)\n", edit.Target, edit.Region)
		fmt.Fprint(out, edit.Diff)
	}
}

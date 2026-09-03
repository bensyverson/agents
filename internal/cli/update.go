package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

// refFlag names the ref every selected source is moved to.
const refFlag = "ref"

// shaDisplay is how much of a commit sha a header shows.
const shaDisplay = 7

func updateCmd() *cobra.Command {
	var ref string
	var all bool
	cmd := &cobra.Command{
		Use:   "update [source…]",
		Short: "Move each git source's pin to a new commit, showing what changed",
		Long: "Fetch each git source the manifest names — or only the ones named as\n" +
			"arguments — resolve --ref, or the remote's default branch, to a commit, and pin\n" +
			"it in " + manifest.FileName + ". A source that moves prints one header line,\n" +
			"`<source>: <old> -> <new>`, then a diff of every module this repo enables whose\n" +
			"body changed, and the regions are re-rendered from the new commit.\n\n" +
			"This is the only verb that moves a ref: `agents sync` renders whatever the\n" +
			"manifest pins, so the rules never change without the diff above. A source\n" +
			"already at that commit prints nothing; a path source and the source built into\n" +
			"the binary have no pin to move and are skipped. It never prompts.\n\n" +
			"With --all every registered repo is visited; a repo that refuses to overwrite a\n" +
			"hand-edited region is reported under its path and makes the exit status\n" +
			"non-zero, after its pin has moved — review it with `agents diff` and finish\n" +
			"with `agents sync --force`.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := openOptions(repo.FetchMissing)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			refused := false
			err = forEachRepo(cmd, opts, all, func(r *repo.Repo) error {
				// update renders, so like sync it creates any seed the repo's
				// modules declare and does not have yet.
				r.Seeds = repo.SeedsCreated
				res, err := r.Update(opts, repo.UpdateOptions{Sources: args, Ref: ref})
				printUpdates(out, res.Updated, all, r.Dir)
				if errors.Is(err, repo.ErrHandEdited) {
					if !all {
						printRefusal(out, res.Sync)
						return errRefused
					}
					refused = true
					fmt.Fprintln(out, r.Dir)
					printRefusal(out, res.Sync)
					fmt.Fprintln(out)
					return nil
				}
				if err != nil {
					return err
				}
				for _, path := range res.Sync.Written {
					fmt.Fprintln(out, prefixed(all, r.Dir, "rendered "+path))
				}
				for _, path := range res.Sync.Seeded {
					fmt.Fprintln(out, prefixed(all, r.Dir, "seeded "+path))
				}
				return nil
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
	cmd.Flags().StringVar(&ref, refFlag, "", "the branch, tag or sha to pin (default: the remote's default branch)")
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

// printUpdates writes one header per source that moved and the diff of every
// enabled module whose body changed. Only the header carries the repo's path
// under --all: a diff line is the module's own text, and prefixing it would
// stop it reading as a diff.
func printUpdates(out io.Writer, updates []repo.SourceUpdate, all bool, dir string) {
	for _, u := range updates {
		fmt.Fprintln(out, prefixed(all, dir, header(u)))
		for _, change := range u.Changes {
			fmt.Fprintf(out, "%s:\n", change.Module)
			fmt.Fprint(out, change.Diff)
		}
	}
}

// header is one source's move in a line. A pin that only changed spelling —
// the sha replacing the branch a human wrote — says so rather than showing a
// commit moving to itself.
func header(u repo.SourceUpdate) string {
	if !u.Moved() {
		if u.From == "" {
			return fmt.Sprintf("%s: pinned to %s", u.Name, short(u.To))
		}
		return fmt.Sprintf("%s: pinned %s to %s", u.Name, u.From, short(u.To))
	}
	from := short(u.FromCommit)
	if u.FromCommit == "" {
		// Nothing in the cache at the old ref, so there is no commit to name:
		// the manifest's own text is the most this can say.
		from = u.From
	}
	return fmt.Sprintf("%s: %s -> %s", u.Name, from, short(u.To))
}

// short abbreviates a commit sha for a header.
func short(sha string) string {
	if len(sha) <= shaDisplay {
		return sha
	}
	return sha[:shaDisplay]
}

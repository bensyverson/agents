package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

func addCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <module>...",
		Short: "Enable modules in this repo and render them",
		Long: "Each module is named `name`, from the manifest's first source, or `source/name`\n" +
			"for any other source it lists.\n\n" +
			"Append each module to " + manifest.FileName + ", in the order given, then render:\n" +
			"an inline module's region goes after every region already in " + repo.AgentsFile + ",\n" +
			"a kind:file module gets its file, and any file the new modules seed is\n" +
			"created. A module the manifest already lists is a message, not an error.\n\n" +
			"Nothing above the first region is touched. A region edited by hand since\n" +
			"its render stops the whole thing — nothing is written, the manifest\n" +
			"included — the same way `agents sync` does.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := openOptions(repo.FetchMissing)
			if err != nil {
				return err
			}
			return forEachRepo(cmd, opts, false, func(r *repo.Repo) error {
				r.Seeds = repo.SeedsCreated
				res, err := r.Add(args)
				if errors.Is(err, repo.ErrHandEdited) {
					printRefusal(cmd.OutOrStdout(), res.Sync)
					return errRefused
				}
				if err != nil {
					return err
				}
				printAdd(cmd.OutOrStdout(), res)
				return nil
			})
		},
	}
}

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <module>...",
		Short: "Disable modules in this repo and delete their regions",
		Long: "Each module is named `name` or `source/name`, as in `agents add`.\n\n" +
			"Drop each module from " + manifest.FileName + " and delete its region. A kind:file\n" +
			"module's file goes too when the region was all it held; a file the project\n" +
			"has written in keeps every one of its own bytes, loses the region, and is\n" +
			"named in a warning.\n\n" +
			"Removing is a decision, not a sync: a region edited by hand since its render\n" +
			"is removed anyway (and said so), and no other module's region is touched.\n" +
			"Nothing above the first region is touched either.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			return forEachRepo(cmd, opts, false, func(r *repo.Repo) error {
				res, err := r.Remove(args)
				if err != nil {
					return err
				}
				printRemove(cmd.OutOrStdout(), res)
				return nil
			})
		},
	}
}

func printAdd(out io.Writer, res repo.AddResult) {
	for _, name := range res.Already {
		fmt.Fprintln(out, name+" is already enabled")
	}
	for _, name := range res.Added {
		fmt.Fprintln(out, "enabled "+name)
	}
	for _, path := range res.Sync.Written {
		fmt.Fprintln(out, "rendered "+path)
	}
	for _, path := range res.Sync.Seeded {
		fmt.Fprintln(out, "seeded "+path)
	}
}

// printRemove reports each module with what became of its region and its file,
// so a file left behind is named where it cannot be missed.
func printRemove(out io.Writer, res repo.RemoveResult) {
	for i, name := range res.Removed {
		fmt.Fprintln(out, "disabled "+name)
		if i >= len(res.Regions) {
			continue
		}
		region := res.Regions[i]
		switch {
		case !region.Present:
			fmt.Fprintln(out, "no "+name+" region in "+region.Target)
		case region.FileDeleted:
			fmt.Fprintln(out, "deleted "+region.Target)
		case region.FileKept:
			fmt.Fprintln(out, "kept "+region.Target+": it holds text the project wrote; removed the region only")
		default:
			fmt.Fprintln(out, "removed "+name+" from "+region.Target)
		}
		if region.HandEdited {
			fmt.Fprintln(out, name+" had been edited by hand; removed it anyway")
		}
	}
}

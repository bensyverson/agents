package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

// nameFlag overrides the name a source is given. The ref flag is update's.
const nameFlag = "name"

func sourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Add or remove the sources this repo takes modules from",
		Long: "A source is where modules come from, listed under sources: in " + manifest.FileName + ".\n" +
			"These verbs edit that list; enabling a module from a source is `agents add`.",
	}
	cmd.AddCommand(sourceAddCmd(), sourceRemoveCmd())
	return cmd
}

func sourceAddCmd() *cobra.Command {
	var name, ref string
	cmd := &cobra.Command{
		Use:   "add <spec>",
		Short: "Add a source to this repo's manifest",
		Long: "spec is read as `agents init` reads its source: a directory that exists is a\n" +
			"path source, a host/path such as github.com/owner/repo is fetched over HTTPS,\n" +
			"and anything else is handed to git as typed. The source is named after its\n" +
			"last path segment unless --name says otherwise. A git source's --ref (a\n" +
			"branch, tag or sha; the remote's default branch when omitted) is resolved on\n" +
			"the remote and the sha written into " + manifest.FileName + ".\n\n" +
			"No module is enabled: that is `agents add <source>/<module>`. A repo that\n" +
			"renders from the example modules built into the binary has them removed, as\n" +
			"`agents remove` would, and the command says so; a manifest that lists\n" +
			"sources is only appended to.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := manifest.ParseSource(args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed(nameFlag) {
				src.Name = name
			}
			if cmd.Flags().Changed(refFlag) {
				if src.Git == "" {
					return fmt.Errorf("--%s applies to a git source, and %q is a directory", refFlag, args[0])
				}
				src.Ref = ref
			}
			opts, err := openOptions(repo.FetchMissing)
			if err != nil {
				return err
			}
			return forEachRepo(cmd, opts, false, func(r *repo.Repo) error {
				res, err := r.AddSource(src, opts)
				if err != nil {
					return err
				}
				printSourceAdd(cmd.OutOrStdout(), res)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&name, nameFlag, "", "name the source goes by (default: the last segment of spec)")
	cmd.Flags().StringVar(&ref, refFlag, "", "branch, tag or sha to pin a git source to (default: the remote's default branch)")
	return cmd
}

func sourceRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a source from this repo's manifest",
		Long: "Drop the named source from " + manifest.FileName + ". Refused while an enabled module\n" +
			"comes from it, naming those modules: removing rules is `agents remove`'s\n" +
			"decision, not a side effect of this one.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			return forEachRepo(cmd, opts, false, func(r *repo.Repo) error {
				res, err := r.RemoveSource(args[0])
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "removed source "+res.Source.Name)
				return nil
			})
		},
	}
}

// printSourceAdd says what was added and, when the swap took the example
// modules out, one line naming them and why — plus a warning for anything it
// could not remove cleanly — then the defaults it enabled in their place.
func printSourceAdd(out io.Writer, res repo.AddSourceResult) {
	fmt.Fprintln(out, "added "+sourceLine(res.Source))
	if len(res.Removed) == 0 {
		return
	}
	defer printAdd(out, repo.AddResult{Added: res.Enabled, Sync: res.Sync})
	fmt.Fprintln(out, "removed the example modules "+strings.Join(res.Removed, ", ")+
		": they are built into the binary, which a manifest with sources: does not use")
	for i, name := range res.Removed {
		region := res.Regions[i]
		if region.FileKept {
			fmt.Fprintln(out, "kept "+region.Target+": it holds text the project wrote; removed the region only")
		}
		if region.HandEdited {
			fmt.Fprintln(out, name+" had been edited by hand; removed it anyway")
		}
	}
}

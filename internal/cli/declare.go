package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

// dryRunFlag makes a declaration verb report what it would write.
const dryRunFlag = "dry-run"

// errHookNotInstalled is the exit status of `hooks install` when a repo's hook
// was left alone; the lines above it say why.
var errHookNotInstalled = errors.New("pre-commit hook not installed")

func gitignoreCmd() *cobra.Command {
	var dryRun, all bool
	cmd := &cobra.Command{
		Use:   "gitignore",
		Short: "Append the ignore lines the repo's modules declare to .gitignore",
		Long: "Append to the " + repo.GitignoreFile + " beside " + manifest.FileName + " every line the enabled\n" +
			"modules declare under ignore: that it lacks, under a `# agents` header,\n" +
			"creating the file if there is anything to add. Lines are only ever added:\n" +
			"none is removed or moved, so a second run changes nothing. Silent when\n" +
			"nothing is missing. init and update run this themselves.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			mode := applyMode(dryRun)
			return forEachRepo(cmd, opts, all, func(r *repo.Repo) error {
				res, err := r.Gitignore(mode)
				if err != nil {
					return err
				}
				printGitignore(cmd.OutOrStdout(), res, mode, all, r.Dir)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, dryRunFlag, false, "print what would be added, and write nothing")
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

func hooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage the pre-commit hook composed from the repo's modules",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(hooksInstallCmd())
	return cmd
}

func hooksInstallCmd() *cobra.Command {
	var dryRun, force, all bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Write the pre-commit hook the repo's modules declare",
		Long: "Write the pre-commit hook git runs for this repo: a marked region running every\n" +
			"command the enabled modules declare under pre-commit:, in manifest order, then\n" +
			"`agents check`. Lines outside the region are the project's and are kept.\n" +
			"The hook follows core.hooksPath when the repo sets it; a hooks directory\n" +
			"outside the repo is shared with other repos and is never written.\n\n" +
			"A hook agents did not write, or whose region was edited by hand, is left\n" +
			"alone and the command exits non-zero. --force replaces it: a hand-written\n" +
			"hook is kept beside the new one as pre-commit.bak, and a hand-edited region\n" +
			"is rewritten. Silent when the hook is current. init and update run this\n" +
			"themselves, without --force.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			mode := applyMode(dryRun)
			policy := repo.HookLeaveOthers
			if force {
				policy = repo.HookReplaceOthers
			}
			out := cmd.OutOrStdout()
			failed := false
			err = forEachRepo(cmd, opts, all, func(r *repo.Repo) error {
				res, err := r.InstallHook(mode, policy)
				if err != nil {
					return err
				}
				switch res.State {
				case repo.HookNoGit:
					fmt.Fprintln(out, prefixed(all, r.Dir, "not a git repository: no pre-commit hook to write"))
					failed = true
				case repo.HookForeign, repo.HookHandEdited, repo.HookOutside:
					failed = true
				}
				printHook(out, res, mode, all, r.Dir)
				return nil
			})
			if err != nil {
				return err
			}
			if failed {
				return errHookNotInstalled
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, dryRunFlag, false, "print the change, and write nothing")
	cmd.Flags().BoolVar(&force, forceFlag, false, "replace a hook agents did not write, or a hand-edited region")
	cmd.Flags().BoolVar(&all, allFlag, false, "every registered repo, not just this one")
	return cmd
}

// applyMode turns the --dry-run flag into the mode the library takes.
func applyMode(dryRun bool) repo.ApplyMode {
	if dryRun {
		return repo.ApplyDryRun
	}
	return repo.ApplyWrite
}

// printGitignore writes one line per ignore line added, or that would be.
func printGitignore(out io.Writer, res repo.GitignoreResult, mode repo.ApplyMode, all bool, dir string) {
	verb := "added "
	if mode == repo.ApplyDryRun {
		verb = "would add "
	}
	for _, line := range res.Added {
		fmt.Fprintln(out, prefixed(all, dir, verb+line+" to "+res.Path))
	}
}

// printHook says what became of the hook. A fresh hook, or no git repo at all,
// says nothing: init and update run this on every repo, and neither is news.
// A dry run also prints the diff, since the change is the point of asking.
func printHook(out io.Writer, res repo.HookResult, mode repo.ApplyMode, all bool, dir string) {
	path := relTo(dir, res.Path)
	switch res.State {
	case repo.HookWritten:
		if mode == repo.ApplyDryRun {
			fmt.Fprintln(out, prefixed(all, dir, "would install "+path))
			fmt.Fprint(out, res.Diff())
			return
		}
		fmt.Fprintln(out, prefixed(all, dir, "installed "+path))
		if res.Backup != "" {
			fmt.Fprintln(out, prefixed(all, dir, "kept the hook it replaced at "+relTo(dir, res.Backup)))
		}
	case repo.HookForeign:
		fmt.Fprintln(out, prefixed(all, dir, "left "+path+" alone: agents did not write it; "+
			"`agents hooks install --force` replaces it, keeping a copy at "+path+".bak"))
	case repo.HookHandEdited:
		fmt.Fprintln(out, prefixed(all, dir, "left "+path+" alone: its agents region was edited by hand; "+
			"`agents hooks install --force` rewrites it"))
	case repo.HookOutside:
		fmt.Fprintln(out, prefixed(all, dir, "left "+path+" alone: core.hooksPath points outside the repo, "+
			"at hooks other repos share"))
	}
}

// relTo shows path relative to dir when it lies inside it — the usual case,
// .git/hooks/pre-commit — and whole otherwise.
func relTo(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.ToSlash(rel)
}

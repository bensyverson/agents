package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

const (
	// withFlag names the module list.
	withFlag = "with"
	// sourceFlag names where those modules come from.
	sourceFlag = "source"
)

func initCmd() *cobra.Command {
	var (
		with []string
		spec string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Make the current directory a managed repo",
		Long: "Write " + manifest.FileName + ", render " + repo.AgentsFile + " (existing content becomes the\n" +
			"project-owned head), create any file the chosen modules seed (" + repo.GotchasFile + "\n" +
			"among them), link " + repo.ClaudeFile + ", and register the repo. Every step is\n" +
			"skipped, not repeated, if it is already done.\n\n" +
			"--source names where the modules come from: a directory that exists is a path\n" +
			"source, anything else a git URL, and the source is named after its last path\n" +
			"segment. A git ref is resolved and the sha written into " + manifest.FileName + ". It\n" +
			"requires --with, since only the source knows what it holds. Without --source\n" +
			"the modules built into the binary are used and no source is listed; more\n" +
			"sources can be added to " + manifest.FileName + " by hand at any time.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := openOptions(repo.FetchMissing)
			if err != nil {
				return err
			}
			var src manifest.Source
			if spec != "" {
				if src, err = repo.ParseSource(spec); err != nil {
					return err
				}
			}
			path, err := registryPath()
			if err != nil {
				return err
			}
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("locating the working directory: %w", err)
			}
			// The flag carries the defaults so `--help` names them, but init
			// must know whether the caller actually said which modules it
			// wants: with a --source of someone else's, the defaults are the
			// wrong answer rather than a fallback.
			var modules []string
			if cmd.Flags().Changed(withFlag) {
				modules = with
			}
			res, err := repo.Init(dir, repo.InitOptions{
				Modules:      modules,
				Source:       src,
				Open:         opts,
				RegistryPath: path,
			})
			// init renders through the same sync path, so it refuses on hand
			// edits the same way — and must show the same diffs.
			if errors.Is(err, repo.ErrHandEdited) {
				printRefusal(cmd.OutOrStdout(), res.Sync)
				return errRefused
			}
			if err != nil {
				return err
			}
			printInit(cmd.OutOrStdout(), res)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&with, withFlag, repo.DefaultModules(), "modules to render, in order")
	cmd.Flags().StringVar(&spec, sourceFlag, "",
		"git URL or directory holding modules/ and templates/, and requires --with;\nomitted, the example modules built into the binary are used")
	return cmd
}

// printInit reports one line per step, saying either what it did or that it
// found the step already done.
func printInit(out io.Writer, res repo.InitResult) {
	fmt.Fprintln(out, sourceLine(res.Source))
	fmt.Fprintln(out, did(res.ManifestWritten, "wrote "+manifest.FileName, "already had "+manifest.FileName))
	if res.HeadSeeded {
		fmt.Fprintln(out, "seeded "+repo.AgentsFile+" head from template")
	}
	for _, path := range res.Sync.Written {
		fmt.Fprintln(out, "rendered "+path)
	}
	for _, path := range res.Sync.Fresh {
		fmt.Fprintln(out, "already rendered "+path)
	}
	for _, path := range res.Sync.Seeded {
		fmt.Fprintln(out, "seeded "+path)
	}

	link := repo.ClaudeFile + " -> " + repo.AgentsFile
	switch res.Link {
	case repo.LinkCreated:
		fmt.Fprintln(out, "linked "+link)
	case repo.LinkAlready:
		fmt.Fprintln(out, "already linked "+link)
	case repo.LinkForeign:
		fmt.Fprintln(out, repo.ClaudeFile+" is not a link to "+repo.AgentsFile+"; left alone")
	}

	fmt.Fprintln(out, did(res.Registered, "registered in "+res.RegistryPath, "already registered in "+res.RegistryPath))
}

// sourceLine names the source the repo renders from. With none listed that is
// the one the binary carries, and the line says where others would go.
func sourceLine(src manifest.Source) string {
	if src.Name == "" {
		return "using the modules built into the binary; list other sources in " + manifest.FileName
	}
	if src.Path != "" {
		return "source " + src.Name + ": " + src.Path
	}
	return "source " + src.Name + ": " + src.Git + " at " + src.Ref
}

func did(done bool, action, already string) string {
	if done {
		return action
	}
	return already
}

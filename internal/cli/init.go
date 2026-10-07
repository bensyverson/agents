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

// withFlag names the module list.
const withFlag = "with"

func initCmd() *cobra.Command {
	var with []string
	cmd := &cobra.Command{
		Use:   "init [source]",
		Short: "Make the current directory a managed repo",
		Long: "Write " + manifest.FileName + ", render " + repo.AgentsFile + " (existing content becomes the\n" +
			"project-owned head), create any file the chosen modules seed (" + repo.GotchasFile + "\n" +
			"among them), link " + repo.ClaudeFile + ", and register the repo. Every step is\n" +
			"skipped, not repeated, if it is already done.\n\n" +
			"source is where the modules come from: a directory that exists is a path\n" +
			"source, a host/path such as github.com/owner/repo is fetched over HTTPS, and\n" +
			"anything else is handed to git as typed. The source is named after its last\n" +
			"path segment, and its ref is resolved and the sha written into " + manifest.FileName + ".\n" +
			"Without a source the modules built into the binary are used and none is\n" +
			"listed; `agents source add` adds one later.\n\n" +
			"Which modules are enabled is --with, else the defaults the source declares in\n" +
			"its source.yaml; a source that declares none requires --with.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := openOptions(repo.FetchMissing)
			if err != nil {
				return err
			}
			var src manifest.Source
			if len(args) == 1 {
				if src, err = manifest.ParseSource(args[0]); err != nil {
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
			res, err := repo.Init(dir, repo.InitOptions{
				Modules:      with,
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
	cmd.Flags().StringSliceVar(&with, withFlag, nil, "modules to render, in order (default: the source's declared defaults)")
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
// the one the binary carries, and the line says how to add another.
func sourceLine(src manifest.Source) string {
	if src.Name == "" {
		return "using the modules built into the binary; `agents source add` adds another source to " + manifest.FileName
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

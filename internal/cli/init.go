package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

// withFlag names the module list. `--modules` is already the directory
// override, so the list of modules to render is `--with`.
const withFlag = "with"

func initCmd() *cobra.Command {
	var with []string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Make the current directory a managed repo",
		Long: "Write " + manifest.FileName + ", render " + repo.AgentsFile + " (existing content becomes the\n" +
			"project-owned head), create any file the chosen modules seed (" + repo.GotchasFile + "\n" +
			"among them), link " + repo.ClaudeFile + ", and register the repo. Every step is\n" +
			"skipped, not repeated, if it is already done.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			set, err := moduleSet(cmd)
			if err != nil {
				return err
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
				Set:          set,
				Templates:    agents.Templates(),
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
	return cmd
}

// printInit reports one line per step, saying either what it did or that it
// found the step already done.
func printInit(out io.Writer, res repo.InitResult) {
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

func did(done bool, action, already string) string {
	if done {
		return action
	}
	return already
}

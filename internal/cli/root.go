// Package cli wires the verbs onto a cobra root; all logic lives in the
// internal packages, so a command body is flag parsing plus printing.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/registry"
	"github.com/bensyverson/agents/internal/repo"
)

const (
	// modulesFlag overrides the modules built into the binary, for iterating on
	// a rule without reinstalling.
	modulesFlag = "modules"
	// allFlag makes a verb walk the registry instead of the working directory.
	allFlag = "all"
	// registryEnvVar overrides the registry file, so a test run never touches
	// the real ~/.config/agents/repos.
	registryEnvVar = "AGENTS_REGISTRY"
)

// Root builds the `agents` command tree.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "agents",
		Short: "Render shared house rules into each repo's AGENTS.md and keep them in sync",
		Long: "Render shared house rules into each repo's AGENTS.md and keep them in sync.\n\n" +
			"Verbs run on the current directory, which must hold " + manifest.FileName + " (agents does\n" +
			"not search parent directories); --all runs over every registered repo instead.\n" +
			"The registry lives at ~/.config/agents/repos, or wherever $" + registryEnvVar + " points.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.PersistentFlags().String(modulesFlag, "",
		"read modules from this directory instead of the ones built into the binary")
	root.AddCommand(initCmd(), addCmd(), removeCmd(), listCmd(), syncCmd(), diffCmd(), statusCmd(), checkCmd())
	return root
}

// moduleSet resolves --modules once per run.
func moduleSet(cmd *cobra.Command) (module.Set, error) {
	dir, err := cmd.Flags().GetString(modulesFlag)
	if err != nil {
		return module.Set{}, err
	}
	if dir != "" {
		return module.LoadDir(dir)
	}
	return module.Load(agents.Modules())
}

// registryPath is $AGENTS_REGISTRY when set, else the default under the user's
// config directory.
func registryPath() (string, error) {
	if path := os.Getenv(registryEnvVar); path != "" {
		return path, nil
	}
	return registry.DefaultPath()
}

// repoDirs is the directories a verb runs over: the working directory, or
// every registered repo.
func repoDirs(all bool) ([]string, error) {
	if !all {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("locating the working directory: %w", err)
		}
		return []string{dir}, nil
	}
	path, err := registryPath()
	if err != nil {
		return nil, err
	}
	return registry.Read(path)
}

// forEachRepo opens each directory in turn. Without --all a directory that is
// not a managed repo is the user's mistake and stops the command; with --all it
// is a stale registry entry, so it warns and carries on.
//
// v1 does not walk up from the working directory: the repo is where you are.
func forEachRepo(cmd *cobra.Command, set module.Set, all bool, fn func(*repo.Repo) error) error {
	dirs, err := repoDirs(all)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		r, err := repo.Open(dir, set)
		if err != nil {
			if !all {
				return err
			}
			// repo.Open's message already leads with the directory.
			fmt.Fprintf(cmd.ErrOrStderr(), "skipping %v\n", err)
			continue
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	return nil
}

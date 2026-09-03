// Package cli wires the verbs onto a cobra root; all logic lives in the
// internal packages, so a command body is flag parsing plus printing.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/registry"
	"github.com/bensyverson/agents/internal/repo"
	"github.com/bensyverson/agents/internal/source"
)

const (
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
			"The registry lives at ~/.config/agents/repos, or wherever $" + registryEnvVar + " points.\n\n" +
			"Modules come from the sources " + manifest.FileName + " names — a git URL or a local\n" +
			"directory, each pinned by ref — or, when it names none, from the source built\n" +
			"into the binary. Fetched sources are cached under $" + source.CacheEnv + ".",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(initCmd(), addCmd(), removeCmd(), listCmd(), syncCmd(), updateCmd(), diffCmd(), statusCmd(), checkCmd())
	return root
}

// openOptions is how every verb opens a repo: one loader over the shared
// cache, the source the binary embeds, and the verb's fetch policy.
//
// Only sync, add and init pass FetchMissing. Everything else runs offline, so
// `agents check` in a pre-commit hook never waits on a network it may not have.
func openOptions(fetch repo.FetchPolicy) (repo.Options, error) {
	cache, err := source.DefaultCacheDir()
	if err != nil {
		return repo.Options{}, err
	}
	return repo.Options{Loader: source.New(cache), Embedded: agents.Embedded(), Fetch: fetch}, nil
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
// is a stale registry entry, so it warns and carries on. A source the cache
// lacks is neither: the repo is fine and the machine has not fetched, so it is
// reported like a skip but fails the run — a pre-commit `check --all` on a
// fresh machine must not warn and pass.
//
// v1 does not walk up from the working directory: the repo is where you are.
func forEachRepo(cmd *cobra.Command, opts repo.Options, all bool, fn func(*repo.Repo) error) error {
	dirs, err := repoDirs(all)
	if err != nil {
		return err
	}
	var unfetched []error
	for _, dir := range dirs {
		r, err := repo.Open(dir, opts)
		if err != nil {
			if !all {
				return err
			}
			if errors.Is(err, source.ErrNotFetched) {
				unfetched = append(unfetched, err)
			}
			// repo.Open's message already leads with the directory.
			fmt.Fprintf(cmd.ErrOrStderr(), "skipping %v\n", err)
			continue
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	if len(unfetched) > 0 {
		return fmt.Errorf("%d repo(s) name a source that is not in the cache: %w", len(unfetched), unfetched[0])
	}
	return nil
}

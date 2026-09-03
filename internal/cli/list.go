package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

// enabledMark leads the line of a module this repo has enabled.
const enabledMark = "*"

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every module this repo's sources hold, marking the ones it enables",
		Long: "One line per module, sorted, with " + enabledMark + " on the ones " + manifest.FileName + " lists\n" +
			"and the file path of every kind:file module. A module from any source but the\n" +
			"first is printed as source/name — the form `agents add` takes back.\n\n" +
			"Unlike the other verbs this one runs anywhere: outside a managed repo it lists\n" +
			"the modules built into the binary and marks nothing. It never fetches.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := openOptions(repo.FetchNever)
			if err != nil {
				return err
			}
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("locating the working directory: %w", err)
			}
			sources, def, enabled, err := listing(dir, opts)
			if err != nil {
				return err
			}
			printList(cmd.OutOrStdout(), repo.List(sources, def, enabled))
			return nil
		},
	}
}

// listing is the repo's sources and enabled modules, or the source the binary
// embeds alone when the working directory is not a managed repo — `agents
// list` has to answer "what could I enable?" anywhere.
func listing(dir string, opts repo.Options) (repo.Sources, string, []manifest.ModuleRef, error) {
	r, err := repo.Open(dir, opts)
	switch {
	case err == nil:
		return r.Sources, r.Manifest.DefaultSource(), r.Manifest.Modules, nil
	case errors.Is(err, repo.ErrNotManaged):
		embedded, err := repo.LoadSource(manifest.ExampleSource, agents.Embedded())
		if err != nil {
			return nil, "", nil, err
		}
		return repo.Sources{manifest.ExampleSource: embedded}, manifest.ExampleSource, nil, nil
	}
	return nil, "", nil, err
}

func printList(out io.Writer, modules []repo.ModuleInfo) {
	width := 0
	for _, m := range modules {
		width = max(width, len(m.Name))
	}
	for _, m := range modules {
		mark := strings.Repeat(" ", len(enabledMark))
		if m.Enabled {
			mark = enabledMark
		}
		line := mark + " " + m.Name
		if m.Path != "" {
			line += strings.Repeat(" ", width-len(m.Name)) + "  " + m.Path
		}
		fmt.Fprintln(out, line)
	}
}

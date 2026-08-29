package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/repo"
)

// enabledMark leads the line of a module this repo has enabled.
const enabledMark = "*"

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every module, marking the ones this repo enables",
		Long: "One line per module the binary knows, sorted, with " + enabledMark + " on the ones\n" +
			manifest.FileName + " lists and the file path of every kind:file module.\n\n" +
			"Unlike the other verbs this one runs anywhere: outside a managed repo it\n" +
			"lists what could be enabled and marks nothing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			set, err := moduleSet(cmd)
			if err != nil {
				return err
			}
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("locating the working directory: %w", err)
			}
			enabled, err := repo.EnabledModules(dir)
			if err != nil {
				return err
			}
			printList(cmd.OutOrStdout(), repo.List(set, enabled))
			return nil
		},
	}
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

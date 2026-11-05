package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bensyverson/agents/internal/repo"
)

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Move a project's own " + repo.ClaudeFile + " into " + repo.AgentsFile + ", before init",
		Long: "Adopt a " + repo.ClaudeFile + " that is a file of its own, so `agents init` can link it.\n" +
			"With no " + repo.AgentsFile + ", the file is renamed to it. Otherwise its text goes at\n" +
			"the end of " + repo.AgentsFile + "'s project-owned head, above the generated regions,\n" +
			"under a marker saying where it came from; merge it into the rules around it\n" +
			"and delete the marker. Either way " + repo.ClaudeFile + " then links to " + repo.AgentsFile + ".\n\n" +
			"A " + repo.ClaudeFile + " that links anywhere else is refused: it may share rules with\n" +
			"other projects, and copying it in would cut this one off from them.\n" +
			"Needs no .agents.yaml; run `agents init` next.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("locating the working directory: %w", err)
			}
			res, err := repo.Migrate(dir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			link := "linked " + repo.ClaudeFile + " -> " + repo.AgentsFile
			switch res {
			case repo.MigrateNothing:
				fmt.Fprintln(out, "nothing to migrate: "+repo.ClaudeFile+" is absent or already links to "+repo.AgentsFile)
				return nil
			case repo.MigrateMoved:
				fmt.Fprintln(out, "moved "+repo.ClaudeFile+" to "+repo.AgentsFile)
			case repo.MigrateAppended:
				fmt.Fprintln(out, "added "+repo.ClaudeFile+"'s text to the end of "+repo.AgentsFile+"'s head, under a marker; merge it by hand")
			}
			fmt.Fprintln(out, link)
			fmt.Fprintln(out, "next: agents init")
			return nil
		},
	}
}

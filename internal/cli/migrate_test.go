package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project with a CLAUDE.md of its own: init refuses and names migrate,
// migrate moves the file, and init then keeps its text as the head.
func TestMigrateThenInit(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	const rules = "# Our rules\n\nUse tabs.\n"
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := run(t, "init", sourceDir, "--with", "core,principles")
	if err == nil || !strings.Contains(err.Error(), "agents migrate") {
		t.Fatalf("init err = %v, want one that points to `agents migrate`", err)
	}

	stdout, _, err := run(t, "migrate")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, want := range []string{"AGENTS.md", "linked CLAUDE.md -> AGENTS.md", "agents init"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("migrate output does not mention %q:\n%s", want, stdout)
		}
	}

	initRepo(t, sourceDir)
	agents, err := os.ReadFile(filepath.Join(repoDir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(agents), rules) {
		t.Errorf("AGENTS.md does not open with the moved rules:\n%s", agents)
	}
}

func TestMigrateWithNothingToDo(t *testing.T) {
	fixture(t)
	stdout, _, err := run(t, "migrate")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !strings.Contains(stdout, "nothing to migrate") {
		t.Errorf("migrate printed %q, want it to say there was nothing to migrate", stdout)
	}
}

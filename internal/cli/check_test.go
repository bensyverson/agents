package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staleModule rewrites a module on disk so the rendered region is one version
// behind, the way a module commit leaves every repo.
func staleModule(t *testing.T, modulesDir string) {
	t.Helper()
	path := filepath.Join(modulesDir, "core.md")
	if err := os.WriteFile(path, []byte(coreBody+"and a new rule\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCheckSilentOnCleanRepo(t *testing.T) {
	_, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)

	stdout, stderr, err := run(t, "check", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("check on a clean repo: %v (stderr %s)", err, stderr)
	}
	if stdout != "" {
		t.Errorf("check printed %q, want silence", stdout)
	}
}

func TestCheckFailsOnStaleAndAdvisesSync(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	before := readAgents(t, repoDir)
	staleModule(t, modulesDir)

	stdout, _, err := run(t, "check", "--modules", modulesDir)
	if err == nil {
		t.Fatal("check returned nil on a stale repo; the process would exit 0")
	}
	if !strings.Contains(stdout, "AGENTS.md: core stale") {
		t.Errorf("check output does not name the stale region:\n%s", stdout)
	}
	if !strings.Contains(err.Error(), "agents sync") {
		t.Errorf("error = %q, want the `agents sync` advice", err)
	}
	// A check must never touch the tree; that is sync's job.
	if after := readAgents(t, repoDir); after != before {
		t.Error("check rewrote AGENTS.md")
	}
}

func TestCheckFailsOnHandEditAndAdvisesDiff(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	handEdit(t, repoDir)

	stdout, _, err := run(t, "check", "--modules", modulesDir)
	if err == nil {
		t.Fatal("check returned nil on a hand-edited repo; the process would exit 0")
	}
	if !strings.Contains(stdout, "AGENTS.md: core hand-edited") {
		t.Errorf("check output does not name the hand-edited region:\n%s", stdout)
	}
	if !strings.Contains(err.Error(), "agents diff") {
		t.Errorf("error = %q, want the `agents diff` advice", err)
	}
	// The diff itself belongs to `agents diff`; check stays one line per problem.
	if strings.Contains(stdout, "AND MY OWN LINE") {
		t.Errorf("check printed a diff, not just the problem:\n%s", stdout)
	}
}

func TestCheckAllPrefixesTheRepoDir(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	staleModule(t, modulesDir)

	stdout, _, err := run(t, "check", "--all", "--modules", modulesDir)
	if err == nil {
		t.Fatal("check --all returned nil with a stale repo registered")
	}
	want := repoDir + ": AGENTS.md: core stale"
	if !strings.Contains(stdout, want) {
		t.Errorf("check --all output does not carry %q:\n%s", want, stdout)
	}
}

func TestCheckAllSilentWhenEveryRepoIsClean(t *testing.T) {
	_, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)

	stdout, _, err := run(t, "check", "--all", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("check --all on a clean fleet: %v", err)
	}
	if stdout != "" {
		t.Errorf("check --all printed %q, want silence", stdout)
	}
}

func readAgents(t *testing.T, repoDir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	return string(b)
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	coreBody       = "core rules\n"
	principlesBody = "principles\n"
	// coreModule is core as a file: the seed it declares is what makes init
	// create project/gotchas.md, whose template ships in the binary.
	coreModule = "---\nseeds: [project/gotchas.md]\n---\n" + coreBody
)

// run executes the command tree the way main does, with output captured.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	root := Root()
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errBuf.String(), err
}

// fixture makes a modules directory and an empty repo, chdirs into the repo and
// points the registry at a temp file.
func fixture(t *testing.T) (repoDir, modulesDir, registryPath string) {
	t.Helper()
	modulesDir = t.TempDir()
	for name, body := range map[string]string{"core.md": coreModule, "principles.md": principlesBody} {
		if err := os.WriteFile(filepath.Join(modulesDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	repoDir = t.TempDir()
	registryPath = filepath.Join(t.TempDir(), "repos")
	t.Setenv(registryEnvVar, registryPath)
	t.Chdir(repoDir)
	return repoDir, modulesDir, registryPath
}

func initRepo(t *testing.T, modulesDir string) {
	t.Helper()
	stdout, stderr, err := run(t, "init", "--modules", modulesDir, "--with", "core,principles")
	if err != nil {
		t.Fatalf("init: %v (stderr %s)", err, stderr)
	}
	for _, want := range []string{".agents.yaml", "AGENTS.md", "project/gotchas.md", "CLAUDE.md", "registered"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("init output does not mention %q:\n%s", want, stdout)
		}
	}
}

func handEdit(t *testing.T, repoDir string) {
	t.Helper()
	path := filepath.Join(repoDir, "AGENTS.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	edited := strings.Replace(string(b), coreBody, coreBody+"AND MY OWN LINE\n", 1)
	if edited == string(b) {
		t.Fatalf("fixture did not apply; AGENTS.md is:\n%s", b)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
}

func TestInitThenSyncStatusDiff(t *testing.T) {
	repoDir, modulesDir, registryPath := fixture(t)
	initRepo(t, modulesDir)

	if _, err := os.Stat(filepath.Join(repoDir, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md: %v", err)
	}
	if b, err := os.ReadFile(registryPath); err != nil || !strings.Contains(string(b), repoDir) {
		t.Errorf("registry %q does not list %s (err %v)", b, repoDir, err)
	}

	// Sync on a freshly initialised repo is silent.
	stdout, _, err := run(t, "sync", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if stdout != "" {
		t.Errorf("sync printed %q, want silence", stdout)
	}

	stdout, _, err = run(t, "status", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, want := range []string{"modules=core,principles", "head=", "stale=0", "edited=0", "gotchas="} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status output missing %q:\n%s", want, stdout)
		}
	}
	// init on an empty dir seeds the head from templates/head.md, so the
	// gauge must read non-zero without pinning the template's length.
	if strings.Contains(stdout, "head=0 ") {
		t.Errorf("seeded head reads as empty:\n%s", stdout)
	}
	if lines := strings.Count(strings.TrimSpace(stdout), "\n"); lines != 0 {
		t.Errorf("status printed more than one line:\n%s", stdout)
	}

	stdout, _, err = run(t, "diff", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if stdout != "" {
		t.Errorf("diff printed %q on a clean repo, want silence", stdout)
	}
}

func TestSyncRefusesHandEditThenForces(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	handEdit(t, repoDir)

	stdout, stderr, err := run(t, "sync", "--modules", modulesDir)
	if err == nil {
		t.Fatal("sync returned nil error on a hand-edited region; the process would exit 0")
	}
	if !strings.Contains(stdout, "AGENTS.md: core (hand-edited)") {
		t.Errorf("sync output has no region header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "AND MY OWN LINE") {
		t.Errorf("sync output has no diff:\n%s", stdout)
	}
	if !strings.Contains(err.Error(), "refusing to sync") {
		t.Errorf("error = %q, want the refusal message", err)
	}
	if strings.Count(stdout+stderr, "refusing to sync") > 1 {
		t.Errorf("refusal message printed twice:\nstdout %s\nstderr %s", stdout, stderr)
	}

	stdout, _, err = run(t, "sync", "--modules", modulesDir, "--force")
	if err != nil {
		t.Fatalf("sync --force: %v", err)
	}
	if !strings.Contains(stdout, "AGENTS.md") {
		t.Errorf("sync --force did not report the write:\n%s", stdout)
	}
	b, _ := os.ReadFile(filepath.Join(repoDir, "AGENTS.md"))
	if strings.Contains(string(b), "AND MY OWN LINE") {
		t.Error("sync --force left the hand edit in place")
	}
}

func TestDiffShowsEditsAndRules(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	handEdit(t, repoDir)
	gotchas := "# Gotchas\n\n## 2026-07-01 — rule: the TDD rule is ambiguous\n\nfirst body line\nsecond body line\n"
	if err := os.WriteFile(filepath.Join(repoDir, "project", "gotchas.md"), []byte(gotchas), 0o644); err != nil {
		t.Fatalf("write gotchas: %v", err)
	}

	stdout, _, err := run(t, "diff", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	// The body prints indented under the headline, so the review can happen
	// without opening every repo's gotchas file.
	for _, want := range []string{"AGENTS.md: core", "+AND MY OWN LINE", "rule entries", "2026-07-01", "TDD rule is ambiguous", "\n    first body line\n    second body line\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("diff output missing %q:\n%s", want, stdout)
		}
	}
}

func TestStatusAllWalksRegistry(t *testing.T) {
	repoDir, modulesDir, registryPath := fixture(t)
	initRepo(t, modulesDir)
	missing := filepath.Join(t.TempDir(), "gone")
	unmanaged := t.TempDir()
	appendRegistry(t, registryPath, missing, unmanaged)

	stdout, _, err := run(t, "status", "--all", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("status --all: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("want one line per registered repo, got:\n%s", stdout)
	}
	if !strings.Contains(lines[0], repoDir) || !strings.Contains(lines[0], "modules=") {
		t.Errorf("first line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "(missing)") {
		t.Errorf("missing repo line = %q", lines[1])
	}
	if !strings.Contains(lines[2], "(not managed)") {
		t.Errorf("unmanaged repo line = %q", lines[2])
	}
}

func TestDiffAllWarnsAndContinues(t *testing.T) {
	repoDir, modulesDir, registryPath := fixture(t)
	initRepo(t, modulesDir)
	handEdit(t, repoDir)
	missing := filepath.Join(t.TempDir(), "gone")
	appendRegistry(t, registryPath, missing)

	stdout, stderr, err := run(t, "diff", "--all", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("diff --all: %v", err)
	}
	if !strings.Contains(stderr, missing) {
		t.Errorf("stderr does not warn about %s:\n%s", missing, stderr)
	}
	if !strings.Contains(stdout, repoDir) {
		t.Errorf("stdout does not carry the repo header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "+AND MY OWN LINE") {
		t.Errorf("stdout does not carry the diff:\n%s", stdout)
	}
}

func TestVerbsRefuseAnUnmanagedDirectory(t *testing.T) {
	_, modulesDir, _ := fixture(t)
	for _, verb := range []string{"sync", "diff", "status"} {
		_, _, err := run(t, verb, "--modules", modulesDir)
		if err == nil {
			t.Errorf("%s in an unmanaged directory returned nil error", verb)
			continue
		}
		if !strings.Contains(err.Error(), ".agents.yaml") {
			t.Errorf("%s error = %q, want it to name .agents.yaml", verb, err)
		}
	}
}

func TestRootListsTheVerbs(t *testing.T) {
	stdout, _, err := run(t, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	for _, verb := range []string{"init", "sync", "diff", "status", "add", "remove", "list"} {
		if !strings.Contains(stdout, verb) {
			t.Errorf("--help does not list %q:\n%s", verb, stdout)
		}
	}
}

func appendRegistry(t *testing.T, path string, repos ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	defer f.Close()
	for _, repo := range repos {
		if _, err := f.WriteString(repo + "\n"); err != nil {
			t.Fatalf("append registry: %v", err)
		}
	}
}

// init re-run over a repo whose regions were hand-edited must refuse the same
// way sync does, rather than emit a bare error with no diff.
func TestInitRefusesHandEditedRegions(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	handEdit(t, repoDir)

	stdout, _, err := run(t, "init", "--modules", modulesDir, "--with", "core,principles")
	if err == nil {
		t.Fatal("init returned nil error over a hand-edited region")
	}
	if !strings.Contains(stdout, "AGENTS.md: core (hand-edited)") {
		t.Errorf("init output has no region header:\n%s", stdout)
	}
	if !strings.Contains(err.Error(), "refusing to sync") {
		t.Errorf("error = %q, want the refusal message", err)
	}
}

// A refusal in one repo must not stop the others: every repo is visited, each
// refusal is printed under its path, and the exit is non-zero at the end.
func TestSyncAllContinuesPastRefusal(t *testing.T) {
	repoA, modulesDir, _ := fixture(t)
	initRepo(t, modulesDir)
	repoB := t.TempDir()
	t.Chdir(repoB)
	initRepo(t, modulesDir)
	t.Chdir(repoA)

	handEdit(t, repoA)
	// Move the module so repo B is stale.
	if err := os.WriteFile(filepath.Join(modulesDir, "principles.md"), []byte("newer principles\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := run(t, "sync", "--all", "--modules", modulesDir)
	if err == nil {
		t.Fatalf("sync --all with a hand edit should fail; stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, repoA) || !strings.Contains(stdout, "AGENTS.md: core (hand-edited)") {
		t.Errorf("refusal for repo A not reported under its path:\n%s", stdout)
	}
	if !strings.Contains(stdout, repoB) || !strings.Contains(stdout, "rendered AGENTS.md") {
		t.Errorf("repo B was not rendered:\n%s", stdout)
	}
	b, _ := os.ReadFile(filepath.Join(repoB, "AGENTS.md"))
	if !strings.Contains(string(b), "newer principles") {
		t.Errorf("repo B's AGENTS.md was not rewritten:\n%s", b)
	}
	a, _ := os.ReadFile(filepath.Join(repoA, "AGENTS.md"))
	if !strings.Contains(string(a), "AND MY OWN LINE") {
		t.Errorf("repo A's hand edit was overwritten:\n%s", a)
	}
}

// The head column is the size of the project-owned prose above the regions, and
// it sits between the module list and the staleness counts.
func TestStatusReportsHeadSize(t *testing.T) {
	repoDir, modulesDir, _ := fixture(t)
	head := "# Project\n\nOne local rule.\n"
	if err := os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte(head), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	initRepo(t, modulesDir)

	stdout, _, err := run(t, "status", "--modules", modulesDir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(stdout, "head=3") {
		t.Errorf("status output missing head=3:\n%s", stdout)
	}
	modules, headCol, stale := strings.Index(stdout, "modules="), strings.Index(stdout, "head="), strings.Index(stdout, "stale=")
	if !(modules < headCol && headCol < stale) {
		t.Errorf("head column is not between modules and stale:\n%s", stdout)
	}
}

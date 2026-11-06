package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/source"
)

// The declaring fixture: one module naming an ignore line and a pre-commit
// command. What either says is arbitrary; the tool applies them blind.
const (
	declaredIgnore  = "out/"
	declaredCommand = "make lint"
	declaringModule = "---\nignore: [" + declaredIgnore + "]\npre-commit: [" + declaredCommand + "]\n---\n" + coreBody
	hookRel         = ".git/hooks/pre-commit"
	handHook        = "#!/bin/sh\nmake test\n"
)

// declaringFixture is a path source whose core module declares a line and a
// command, and an empty git work tree to init, chdir'd into.
func declaringFixture(t *testing.T) (repoDir, sourceDir string) {
	t.Helper()
	sourceDir = newSource(t, sourceName, map[string]string{"modules/core.md": declaringModule})
	repoDir = t.TempDir()
	writeSourceFile(t, repoDir, ".git/config", "[core]\n")
	t.Setenv(registryEnvVar, filepath.Join(t.TempDir(), "repos"))
	t.Setenv(source.CacheEnv, t.TempDir())
	t.Chdir(repoDir)
	return repoDir, sourceDir
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	stdout, stderr, err := run(t, args...)
	if err != nil {
		t.Fatalf("%v: %v\nstdout %s\nstderr %s", args, err, stdout, stderr)
	}
	return stdout
}

func readRel(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestInitReportsIgnoreLinesAndTheHook(t *testing.T) {
	repoDir, sourceDir := declaringFixture(t)
	stdout := mustRun(t, "init", sourceDir, "--with", "core")

	for _, want := range []string{"added " + declaredIgnore + " to .gitignore", "installed " + hookRel} {
		if !strings.Contains(stdout, want) {
			t.Errorf("init output lacks %q:\n%s", want, stdout)
		}
	}
	if hook := readRel(t, repoDir, hookRel); !strings.Contains(hook, declaredCommand+"\n") {
		t.Errorf("hook does not run %q:\n%s", declaredCommand, hook)
	}
}

// init leaves a hand-written hook alone, says how to replace it, and succeeds.
func TestInitNamesAForeignHook(t *testing.T) {
	repoDir, sourceDir := declaringFixture(t)
	writeSourceFile(t, repoDir, hookRel, handHook)
	stdout := mustRun(t, "init", sourceDir, "--with", "core")

	if !strings.Contains(stdout, "left "+hookRel+" alone") || !strings.Contains(stdout, "agents hooks install --force") {
		t.Errorf("init output does not explain the foreign hook:\n%s", stdout)
	}
	if got := readRel(t, repoDir, hookRel); got != handHook {
		t.Errorf("hook changed:\n%s", got)
	}
}

func TestGitignoreVerb(t *testing.T) {
	repoDir, sourceDir := declaringFixture(t)
	mustRun(t, "init", sourceDir, "--with", "core")
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if out := mustRun(t, "gitignore", "--dry-run"); !strings.Contains(out, "would add "+declaredIgnore+" to .gitignore") {
		t.Errorf("dry run output:\n%s", out)
	}
	if got := readRel(t, repoDir, ".gitignore"); got != "bin/\n" {
		t.Errorf("dry run wrote .gitignore:\n%s", got)
	}
	if out := mustRun(t, "gitignore"); !strings.Contains(out, "added "+declaredIgnore+" to .gitignore") {
		t.Errorf("output:\n%s", out)
	}
	if out := mustRun(t, "gitignore"); out != "" {
		t.Errorf("second run printed %q, want silence", out)
	}
}

func TestHooksInstallVerb(t *testing.T) {
	repoDir, sourceDir := declaringFixture(t)
	writeSourceFile(t, repoDir, hookRel, handHook)
	mustRun(t, "init", sourceDir, "--with", "core")

	stdout, _, err := run(t, "hooks", "install")
	if err == nil {
		t.Fatalf("hooks install over a hand-written hook succeeded:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--force") {
		t.Errorf("refusal does not say how to replace it:\n%s", stdout)
	}

	out := mustRun(t, "hooks", "install", "--force", "--dry-run")
	if !strings.Contains(out, "would install "+hookRel) || !strings.Contains(out, "+"+declaredCommand) {
		t.Errorf("dry run output:\n%s", out)
	}
	if got := readRel(t, repoDir, hookRel); got != handHook {
		t.Errorf("dry run wrote the hook:\n%s", got)
	}

	out = mustRun(t, "hooks", "install", "--force")
	if !strings.Contains(out, "installed "+hookRel) || !strings.Contains(out, hookRel+".bak") {
		t.Errorf("output does not name the hook and its backup:\n%s", out)
	}
	if out := mustRun(t, "hooks", "install"); out != "" {
		t.Errorf("second run printed %q, want silence", out)
	}
}

// A repo with no hook is pointed at the verb that writes one.
func TestStatusHintNamesHooksInstall(t *testing.T) {
	repoDir, sourceDir := declaringFixture(t)
	mustRun(t, "init", sourceDir, "--with", "core")
	if err := os.Remove(filepath.Join(repoDir, hookRel)); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := run(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(stderr, "agents hooks install") {
		t.Errorf("status hint does not name `agents hooks install`:\n%s", stderr)
	}
}

func TestRootListsTheDeclarationVerbs(t *testing.T) {
	stdout := mustRun(t, "--help")
	for _, verb := range []string{"gitignore", "hooks"} {
		if !strings.Contains(stdout, verb) {
			t.Errorf("--help does not list %q:\n%s", verb, stdout)
		}
	}
}

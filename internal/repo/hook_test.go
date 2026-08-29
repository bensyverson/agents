package repo

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	// hookWithout is a plausible pre-commit hook that does not gate on agents.
	hookWithout = "#!/usr/bin/env bash\nset -euo pipefail\ngo test ./...\n"
	// hookWith gates on the verb, however it is invoked.
	hookWith = hookWithout + "agents check || exit 1\n"
)

func wantHint(t *testing.T, dir, wantPath string) string {
	t.Helper()
	hint, ok := HookHint(dir)
	if !ok {
		t.Fatalf("HookHint(%s) = %q, false; want a hint", dir, hint)
	}
	if !strings.Contains(hint, wantPath) {
		t.Errorf("hint %q does not name %s", hint, wantPath)
	}
	if !strings.Contains(hint, hookLine) {
		t.Errorf("hint %q does not carry the line to paste (%q)", hint, hookLine)
	}
	if strings.Contains(strings.TrimSuffix(hint, "\n"), "\n") {
		t.Errorf("hint is more than one line:\n%s", hint)
	}
	return hint
}

func TestHookHintWhenHookDoesNotCallCheck(t *testing.T) {
	dir := newRepo(t, map[string]string{".git/hooks/pre-commit": hookWithout})
	wantHint(t, dir, filepath.Join(dir, ".git", "hooks", "pre-commit"))
}

func TestHookHintSilentWhenHookCallsCheck(t *testing.T) {
	dir := newRepo(t, map[string]string{".git/hooks/pre-commit": hookWith})
	if hint, ok := HookHint(dir); ok {
		t.Errorf("HookHint = %q, true; want silence when the hook already calls it", hint)
	}
}

// A hook that shells out to the source tree still calls the verb.
func TestHookHintSilentWhenHookRunsFromSource(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".git/hooks/pre-commit": hookWithout + "go run ./cmd/agents check\n",
	})
	if hint, ok := HookHint(dir); ok {
		t.Errorf("HookHint = %q, true; want silence for `go run ./cmd/agents check`", hint)
	}
}

func TestHookHintWhenThereIsNoHook(t *testing.T) {
	dir := newRepo(t, map[string]string{".git/config": "[core]\n\trepositoryformatversion = 0\n"})
	hint := wantHint(t, dir, filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if !strings.Contains(hint, "no pre-commit hook") {
		t.Errorf("hint %q does not say the hook is absent", hint)
	}
}

func TestHookHintFollowsCoreHooksPath(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".git/config":                  "[core]\n\trepositoryformatversion = 0\n\thooksPath = scripts/git-hooks\n",
		"scripts/git-hooks/pre-commit": hookWithout,
		// The default location holds a passing hook, so a hint proves the
		// configured path won.
		".git/hooks/pre-commit": hookWith,
	})
	wantHint(t, dir, filepath.Join(dir, "scripts", "git-hooks", "pre-commit"))
}

func TestHookHintFollowsAbsoluteCoreHooksPath(t *testing.T) {
	hooks := t.TempDir()
	writeFixture(t, hooks, "pre-commit", hookWith)
	dir := newRepo(t, map[string]string{
		".git/config":           "[core]\n\thooksPath = " + hooks + "\n",
		".git/hooks/pre-commit": hookWithout,
	})
	if hint, ok := HookHint(dir); ok {
		t.Errorf("HookHint = %q, true; want the absolute hooksPath hook to win", hint)
	}
}

// core.hooksPath under some other section is not core.hooksPath.
func TestHookHintIgnoresHooksPathInAnotherSection(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".git/config":           "[core]\n[alias]\n\thooksPath = scripts/git-hooks\n",
		".git/hooks/pre-commit": hookWithout,
	})
	wantHint(t, dir, filepath.Join(dir, ".git", "hooks", "pre-commit"))
}

// In a linked worktree .git is a file pointing at the worktree's git dir, and
// the hooks live in the common dir it names.
func TestHookHintFollowsWorktreeGitFile(t *testing.T) {
	main := newRepo(t, map[string]string{
		".git/hooks/pre-commit":       hookWithout,
		".git/worktrees/wt/commondir": "../..\n",
		".git/worktrees/wt/gitdir":    "/nowhere/.git\n",
	})
	wt := t.TempDir()
	writeFixture(t, wt, ".git", "gitdir: "+filepath.Join(main, ".git", "worktrees", "wt")+"\n")

	wantHint(t, wt, filepath.Join(main, ".git", "hooks", "pre-commit"))
}

// A submodule's .git file names a git dir with no commondir; its own hooks
// directory is the one that runs.
func TestHookHintFollowsGitFileWithoutCommonDir(t *testing.T) {
	main := newRepo(t, map[string]string{".git/modules/sub/hooks/pre-commit": hookWithout})
	sub := t.TempDir()
	writeFixture(t, sub, ".git", "gitdir: "+filepath.Join(main, ".git", "modules", "sub")+"\n")

	wantHint(t, sub, filepath.Join(main, ".git", "modules", "sub", "hooks", "pre-commit"))
}

// Nothing to say about a directory git does not manage.
func TestHookHintSilentOutsideAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if hint, ok := HookHint(dir); ok {
		t.Errorf("HookHint = %q, true; want silence outside a git repo", hint)
	}
}

// A hooks directory that is not there at all is the same as an absent hook.
func TestHookHintWhenHooksPathIsMissing(t *testing.T) {
	dir := newRepo(t, map[string]string{".git/config": "[core]\n\thooksPath = scripts/git-hooks\n"})
	wantHint(t, dir, filepath.Join(dir, "scripts", "git-hooks", "pre-commit"))
}

package repo

import (
	"path/filepath"
	"reflect"
	"testing"
)

// init applies what the modules declare: the ignore lines and the hook.
func TestInitAppliesIgnoreLinesAndTheHook(t *testing.T) {
	dir := newRepo(t, map[string]string{".git/config": "[core]\n"})
	opts := InitOptions{
		Modules:      []string{"lint", "tools"},
		Open:         Options{Embedded: declaringSourceFS()},
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	}

	res, err := Init(dir, opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if want := []string{scratchDir, outDir, cacheDir}; !reflect.DeepEqual(res.Gitignore.Added, want) {
		t.Errorf("Gitignore.Added = %v, want %v", res.Gitignore.Added, want)
	}
	if res.Hook.State != HookWritten {
		t.Errorf("Hook.State = %v, want %v", res.Hook.State, HookWritten)
	}
	if got := readFixture(t, dir, defaultHook); got != declaringHook() {
		t.Errorf("hook:\n got %q\nwant %q", got, declaringHook())
	}

	// A second init finds both steps done.
	res, err = Init(dir, opts)
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if len(res.Gitignore.Added) != 0 || res.Hook.State != HookFresh {
		t.Errorf("second Init: Added = %v, Hook.State = %v; want nothing and fresh", res.Gitignore.Added, res.Hook.State)
	}
}

// init leaves a hand-written hook alone and still succeeds: the rest of the
// setup is not hostage to it.
func TestInitLeavesAForeignHookAlone(t *testing.T) {
	dir := newRepo(t, map[string]string{".git/config": "[core]\n", defaultHook: foreignHook})
	res, err := Init(dir, InitOptions{Modules: []string{"lint"}, Open: Options{Embedded: declaringSourceFS()}})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.Hook.State != HookForeign {
		t.Errorf("Hook.State = %v, want %v", res.Hook.State, HookForeign)
	}
	if got := readFixture(t, dir, defaultHook); got != foreignHook {
		t.Errorf("foreign hook was changed:\n%q", got)
	}
}

// When a pin moves to a commit whose module declares new lines and commands,
// update re-applies both.
func TestUpdateRefreshesIgnoreLinesAndTheHook(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir, cache := syncedRepo(t, fx.url, fx.sha, "  - one\n")
	writeFixture(t, dir, ".git/config", "[core]\n")
	r, err := Open(dir, testOptionsWith(cache, FetchNever))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	mustInstallHook(t, r, ApplyWrite, HookLeaveOthers)

	pushCommit(t, fx.url, map[string]string{"modules/one.md": "---\nseeds: [" + gitSeedFile + "]\n" +
		"ignore: [" + outDir + "]\npre-commit: [" + toolCmd + "]\n---\n" + gitModuleBody})

	res := mustUpdate(t, dir, cache, UpdateOptions{})

	if want := []string{outDir}; !reflect.DeepEqual(res.Gitignore.Added, want) {
		t.Errorf("Gitignore.Added = %v, want %v", res.Gitignore.Added, want)
	}
	if res.Hook.State != HookWritten {
		t.Errorf("Hook.State = %v, want %v", res.Hook.State, HookWritten)
	}
	if got, want := readFixture(t, dir, defaultHook), wantShebang+hookRegion(toolCmd); got != want {
		t.Errorf("hook:\n got %q\nwant %q", got, want)
	}
}

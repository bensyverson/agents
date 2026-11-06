package repo

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

// The hook the tool writes, spelled out rather than taken from the code under
// test, so these tests say for themselves what lands in .git/hooks.
const (
	wantShebang = "#!/bin/sh\n"
	wantHookPre = "# Written by `agents hooks install` from this repo's modules, which rewrites\n" +
		"# everything between these markers. Lines outside them are the project's.\n" +
		"set -e\n"
	wantHookCheck = "agents check\n"
	wantHookEnd   = "# agents:end pre-commit\n"
)

// hookRegion is the generated region running these commands, then the check.
func hookRegion(cmds ...string) string {
	body := wantHookPre
	for _, c := range cmds {
		body += c + "\n"
	}
	body += wantHookCheck
	return "# agents:begin pre-commit@" + testHash(body) + "\n" + body + wantHookEnd
}

// declaringHook is the whole hook the declaring modules produce.
func declaringHook() string { return wantShebang + hookRegion(lintCmd, fmtCmd, toolCmd) }

// defaultHook is where git runs the pre-commit hook in a plain clone.
const defaultHook = ".git/hooks/pre-commit"

// gitRepoWith lays out a declaring repo with a .git directory and files.
func gitRepoWith(t *testing.T, files map[string]string) string {
	t.Helper()
	all := map[string]string{".git/config": "[core]\n\trepositoryformatversion = 0\n"}
	maps.Copy(all, files)
	return declaringRepo(t, all)
}

func mustInstallHook(t *testing.T, r *Repo, mode ApplyMode, policy HookPolicy) HookResult {
	t.Helper()
	res, err := r.InstallHook(mode, policy)
	if err != nil {
		t.Fatalf("InstallHook: %v", err)
	}
	return res
}

// Commands come in manifest order, each once, and `agents check` is not among
// them: the hook adds it last on its own.
func TestPreCommitCommandsInModuleOrderWithoutDuplicates(t *testing.T) {
	r := openDeclaring(t, declaringRepo(t, nil))
	if got, want := r.PreCommitCommands(), []string{lintCmd, fmtCmd, toolCmd}; !reflect.DeepEqual(got, want) {
		t.Errorf("PreCommitCommands() = %v, want %v", got, want)
	}
}

func TestInstallHookWritesTheComposedHook(t *testing.T) {
	dir := gitRepoWith(t, nil)
	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)

	if res.State != HookWritten {
		t.Errorf("State = %v, want %v", res.State, HookWritten)
	}
	if want := filepath.Join(dir, defaultHook); res.Path != want {
		t.Errorf("Path = %q, want %q", res.Path, want)
	}
	if got := readFixture(t, dir, defaultHook); got != declaringHook() {
		t.Errorf("hook:\n got %q\nwant %q", got, declaringHook())
	}
	info, err := os.Stat(filepath.Join(dir, defaultHook))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0o111 {
		t.Errorf("hook mode = %v, want executable", info.Mode().Perm())
	}
}

// A repo whose modules declare no commands still gets the check.
func TestInstallHookWithNoCommandsRunsOnlyTheCheck(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n",
		".git/config":     "[core]\n",
	})
	mustInstallHook(t, mustOpen(t, dir), ApplyWrite, HookLeaveOthers)
	if got, want := readFixture(t, dir, defaultHook), wantShebang+hookRegion(); got != want {
		t.Errorf("hook:\n got %q\nwant %q", got, want)
	}
}

func TestInstallHookTwiceChangesNothing(t *testing.T) {
	dir := gitRepoWith(t, nil)
	mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)
	before := snapshot(t, dir)

	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)

	if res.State != HookFresh {
		t.Errorf("second run State = %v, want %v", res.State, HookFresh)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("second run changed the repo")
	}
}

// When the modules' commands change, the region is rewritten and every line
// the project wrote around it is kept byte for byte.
func TestInstallHookRefreshesItsRegionAndKeepsTheProjectsLines(t *testing.T) {
	head := "#!/usr/bin/env bash\n# mine\n"
	tail := "echo project check\n"
	dir := gitRepoWith(t, map[string]string{defaultHook: head + hookRegion("old command") + tail})

	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)

	if res.State != HookWritten {
		t.Errorf("State = %v, want %v", res.State, HookWritten)
	}
	want := head + hookRegion(lintCmd, fmtCmd, toolCmd) + tail
	if got := readFixture(t, dir, defaultHook); got != want {
		t.Errorf("hook:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(res.Diff(), "-old command") || !strings.Contains(res.Diff(), "+"+toolCmd) {
		t.Errorf("Diff() does not show the change:\n%s", res.Diff())
	}
}

const foreignHook = "#!/bin/sh\nmake lint\n"

// A hook with no agents region is the project's: left alone, and said so.
func TestInstallHookLeavesAForeignHookAlone(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{defaultHook: foreignHook})
	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)

	if res.State != HookForeign {
		t.Errorf("State = %v, want %v", res.State, HookForeign)
	}
	if got := readFixture(t, dir, defaultHook); got != foreignHook {
		t.Errorf("foreign hook was changed:\n%q", got)
	}
}

// With consent the foreign hook is replaced, and kept beside it: .git/hooks is
// not committed, so overwriting it would lose it for good.
func TestInstallHookReplacesAForeignHookKeepingACopy(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{defaultHook: foreignHook})
	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookReplaceOthers)

	if res.State != HookWritten {
		t.Errorf("State = %v, want %v", res.State, HookWritten)
	}
	if want := filepath.Join(dir, defaultHook+".bak"); res.Backup != want {
		t.Errorf("Backup = %q, want %q", res.Backup, want)
	}
	if got := readFixture(t, dir, defaultHook+".bak"); got != foreignHook {
		t.Errorf("backup = %q, want the foreign hook", got)
	}
	if got := readFixture(t, dir, defaultHook); got != declaringHook() {
		t.Errorf("hook:\n got %q\nwant %q", got, declaringHook())
	}
}

// A backup already there is never overwritten: it may be the only copy of an
// earlier hand-written hook.
func TestInstallHookRefusesToOverwriteABackup(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{defaultHook: foreignHook, defaultHook + ".bak": "older\n"})
	if _, err := openDeclaring(t, dir).InstallHook(ApplyWrite, HookReplaceOthers); err == nil {
		t.Fatal("InstallHook over an existing backup = nil error, want one")
	}
	if got := readFixture(t, dir, defaultHook); got != foreignHook {
		t.Errorf("hook was changed despite the refusal:\n%q", got)
	}
	if got := readFixture(t, dir, defaultHook+".bak"); got != "older\n" {
		t.Errorf("backup was changed: %q", got)
	}
}

// A region whose body no longer matches its marker was edited by hand: left
// alone unless replacing is agreed to, and then only the region is rewritten.
func TestInstallHookHandEditedRegion(t *testing.T) {
	edited := strings.Replace(hookRegion(lintCmd, fmtCmd, toolCmd), toolCmd, "tool test --quick", 1)
	tail := "echo after\n"
	dir := gitRepoWith(t, map[string]string{defaultHook: wantShebang + edited + tail})

	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)
	if res.State != HookHandEdited {
		t.Errorf("State = %v, want %v", res.State, HookHandEdited)
	}
	if got := readFixture(t, dir, defaultHook); got != wantShebang+edited+tail {
		t.Errorf("hand-edited hook was changed:\n%q", got)
	}

	res = mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookReplaceOthers)
	if res.State != HookWritten || res.Backup != "" {
		t.Errorf("forced: State = %v, Backup = %q; want written with no backup", res.State, res.Backup)
	}
	if got, want := readFixture(t, dir, defaultHook), declaringHook()+tail; got != want {
		t.Errorf("hook:\n got %q\nwant %q", got, want)
	}
}

func TestInstallHookDryRunWritesNothing(t *testing.T) {
	dir := gitRepoWith(t, nil)
	before := snapshot(t, dir)

	res := mustInstallHook(t, openDeclaring(t, dir), ApplyDryRun, HookLeaveOthers)

	if res.State != HookWritten || res.After != declaringHook() {
		t.Errorf("dry run: State = %v, After = %q; want what would be written", res.State, res.After)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("dry run changed the repo")
	}
}

func TestInstallHookOutsideAGitRepo(t *testing.T) {
	dir := declaringRepo(t, nil)
	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)
	if res.State != HookNoGit {
		t.Errorf("State = %v, want %v", res.State, HookNoGit)
	}
}

// core.hooksPath inside the repo is where git runs hooks, so that is where it goes.
func TestInstallHookFollowsCoreHooksPath(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{".git/config": "[core]\n\thooksPath = scripts/hooks\n"})
	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookLeaveOthers)

	if want := filepath.Join(dir, "scripts", "hooks", "pre-commit"); res.Path != want {
		t.Errorf("Path = %q, want %q", res.Path, want)
	}
	if got := readFixture(t, dir, "scripts/hooks/pre-commit"); got != declaringHook() {
		t.Errorf("hook:\n got %q\nwant %q", got, declaringHook())
	}
}

// A hooks directory outside the repo is shared with other repos, so one repo's
// hook is never written there, consent or not.
func TestInstallHookRefusesAHooksPathOutsideTheRepo(t *testing.T) {
	shared := t.TempDir()
	dir := gitRepoWith(t, map[string]string{".git/config": "[core]\n\thooksPath = " + shared + "\n"})

	res := mustInstallHook(t, openDeclaring(t, dir), ApplyWrite, HookReplaceOthers)

	if res.State != HookOutside {
		t.Errorf("State = %v, want %v", res.State, HookOutside)
	}
	if fileExists(t, shared, "pre-commit") {
		t.Error("a hook was written to the shared hooks directory")
	}
}

package repo

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// hookLine is what a pre-commit hook needs, and what the hint tells the
	// user to paste.
	hookLine = "agents check || exit 1"
	// hookCall is what counts as already calling the verb, however it is
	// invoked — `agents check`, `go run ./cmd/agents check`, a full path.
	hookCall = "agents check"
	// preCommitHook is the hook file git runs before a commit.
	preCommitHook = "pre-commit"
	// hooksDirName is the default hooks directory inside a git dir.
	hooksDirName = "hooks"
	// gitDirName is the repo's git directory — or, in a linked worktree or a
	// submodule, a file naming it.
	gitDirName = ".git"
	// gitDirPrefix introduces the path in that file.
	gitDirPrefix = "gitdir:"
	// commonDirFile, inside a worktree's git dir, names the shared git dir
	// whose hooks actually run.
	commonDirFile = "commondir"
	// configFile holds core.hooksPath, in the common git dir.
	configFile = "config"
	// hooksPathKey is the config key that moves the hooks directory. Git
	// matches config keys case-insensitively.
	hooksPathKey = "hookspath"
	// coreSection is the only section hooksPath is read from.
	coreSection = "core"
)

// HookHint reports what to tell the user about this repo's pre-commit hook.
//
// ok is true when there is something to say: the hook exists but never calls
// `agents check`, or there is no hook at all. It is false when the hook
// already calls the verb — and when dir is not a git repo, where there is no
// hook to talk about. The tool never writes a consumer's hook; this is the
// whole of its involvement.
func HookHint(dir string) (string, bool) {
	git, ok := gitDir(dir)
	if !ok {
		return "", false
	}
	path := hookPath(dir, git)
	body, exists, err := readIfPresent(path)
	switch {
	case err != nil:
		// An unreadable hook is not the user's problem to solve here.
		return "", false
	case !exists:
		return "no pre-commit hook at " + path + " — one line, `" + hookLine +
			"`, keeps this repo's AGENTS.md from going stale", true
	case strings.Contains(body, hookCall):
		return "", false
	}
	return path + " does not run `" + hookCall + "` — add the line `" + hookLine + "`", true
}

// gitDir resolves dir's git directory: .git itself, or the path named by a
// .git file in a linked worktree or submodule, following commondir so the
// answer is the git dir whose hooks and config git actually uses.
func gitDir(dir string) (string, bool) {
	path := filepath.Join(dir, gitDirName)
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return path, true
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	line, ok := strings.CutPrefix(strings.TrimSpace(string(b)), gitDirPrefix)
	if !ok {
		return "", false
	}
	git := resolve(dir, strings.TrimSpace(line))

	// A worktree's own git dir holds no hooks or config; commondir names the
	// one that does. A submodule has no commondir and keeps both itself.
	common, err := os.ReadFile(filepath.Join(git, commonDirFile))
	if err != nil {
		return git, true
	}
	return resolve(git, strings.TrimSpace(string(common))), true
}

// hookPath is the pre-commit hook git would run: core.hooksPath when set —
// relative to the working tree, as git resolves it — else the git dir's hooks.
func hookPath(dir, git string) string {
	if configured, ok := hooksPath(filepath.Join(git, configFile)); ok {
		return filepath.Join(resolve(dir, configured), preCommitHook)
	}
	return filepath.Join(git, hooksDirName, preCommitHook)
}

// hooksPath reads core.hooksPath out of a git config file. Reading the file
// rather than shelling out to `git config` keeps this testable, and keeps a
// hint from depending on git being on the PATH.
func hooksPath(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	section := ""
	for line := range strings.Lines(string(b)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if name, ok := sectionName(line); ok {
			section = name
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section != coreSection || !strings.EqualFold(strings.TrimSpace(key), hooksPathKey) {
			continue
		}
		if value = unquote(strings.TrimSpace(value)); value != "" {
			return value, true
		}
	}
	return "", false
}

// sectionName reads `[core]` or `[core "sub"]`, lowercased the way git treats
// a section name.
func sectionName(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	name := strings.Trim(line, "[]")
	name, _, _ = strings.Cut(name, " ")
	return strings.ToLower(strings.TrimSpace(name)), true
}

// unquote drops the quotes git allows around a config value.
func unquote(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value[1 : len(value)-1]
	}
	return value
}

// resolve reads path as git does: absolute wins, otherwise it is relative to
// base.
func resolve(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/render"
)

const (
	// hookShebang opens a hook the tool creates. A hook it refreshes keeps
	// whatever the project put above the region instead.
	hookShebang = "#!/bin/sh\n"
	// hookPreamble opens the region's body. set -e makes any declared command
	// that fails stop the commit.
	hookPreamble = "# Written by `agents hooks install` from this repo's modules, which rewrites\n" +
		"# everything between these markers. Lines outside them are the project's.\n" +
		"set -e\n"
	// hookEnd closes the region. Shell comments, not the HTML comments of
	// AGENTS.md, because the file is a script.
	hookEnd = "# agents:end pre-commit"
	// hookBackupSuffix names the copy kept of a hand-written hook replaced
	// with consent.
	hookBackupSuffix = ".bak"
	// hookExec is the permission bits a hook needs for git to run it.
	hookExec os.FileMode = 0o111
)

// hookBeginRE matches the region's opening marker and captures its hash.
var hookBeginRE = regexp.MustCompile(fmt.Sprintf(`^# agents:begin pre-commit@([0-9a-f]{%d})$`, module.HashLength))

// HookPolicy says what InstallHook does with a hook it would have to take over:
// one it did not write, or one whose region was edited by hand. A bool would
// read as "force", and grow to mean more than this one decision.
type HookPolicy int

const (
	// HookLeaveOthers leaves such a hook alone and reports it.
	HookLeaveOthers HookPolicy = iota
	// HookReplaceOthers replaces it: a hand-written hook is kept beside the
	// new one, and a hand-edited region is rewritten in place.
	HookReplaceOthers
)

// HookState is what InstallHook found, and so what it did.
type HookState int

const (
	// HookUnvisited is the zero value: nothing looked at the hook, as when
	// update moved no pin and so applied nothing.
	HookUnvisited HookState = iota
	// HookWritten means the hook was written, or would be in a dry run.
	HookWritten
	// HookFresh means the hook already says what the modules declare.
	HookFresh
	// HookForeign means a hook with no agents region is there, left alone.
	HookForeign
	// HookHandEdited means the region was edited by hand, left alone.
	HookHandEdited
	// HookNoGit means the directory is not a git work tree: no hook to write.
	HookNoGit
	// HookOutside means core.hooksPath points outside both the repo and its
	// git dir — a hooks directory other repos share, which one repo's hook
	// must never take over.
	HookOutside
)

func (s HookState) String() string {
	switch s {
	case HookUnvisited:
		return "unvisited"
	case HookWritten:
		return "written"
	case HookFresh:
		return "fresh"
	case HookForeign:
		return "foreign"
	case HookHandEdited:
		return "hand-edited"
	case HookNoGit:
		return "no git"
	case HookOutside:
		return "outside"
	default:
		return fmt.Sprintf("HookState(%d)", int(s))
	}
}

// HookResult is what InstallHook did, or would do.
type HookResult struct {
	State HookState
	// Path is the hook git runs; empty when the directory is not a git repo.
	Path string
	// Backup is where a replaced hand-written hook was kept, if one was.
	Backup string
	// Before is the hook as it stood, After as written (or as it would be).
	Before, After string
}

// Diff reads from the hook as it stood to the hook as written.
func (h HookResult) Diff() string { return render.Diff(h.Before, h.After) }

// PreCommitCommands is every command the repo's modules declare under
// pre-commit:, in manifest order, each once however many modules declare it.
func (r *Repo) PreCommitCommands() []string {
	return r.declared(func(m module.Module) []string { return m.PreCommit })
}

// InstallHook writes the pre-commit hook git runs for this repo: a marked
// region running the modules' declared commands, then `agents check`.
//
// The region is the tool's and everything around it the project's, as in
// AGENTS.md. A hook with no region is the project's entirely, and a region
// whose body no longer hashes to its marker was edited by hand; both are left
// alone under HookLeaveOthers. The hook lives in the git dir, which is not
// committed, so each clone needs its own: init and update call this too.
func (r *Repo) InstallHook(mode ApplyMode, policy HookPolicy) (HookResult, error) {
	git, ok := gitDir(r.Dir)
	if !ok {
		return HookResult{State: HookNoGit}, nil
	}
	path := hookPath(r.Dir, git)
	res := HookResult{Path: path}
	if !within(r.Dir, path) && !within(git, path) {
		res.State = HookOutside
		return res, nil
	}
	current, exists, err := readIfPresent(path)
	if err != nil {
		return res, err
	}
	res.Before = current

	region := r.hookRegion()
	parsed, found := parseHook(current)
	switch {
	case !exists:
		res.After = hookShebang + region
	case !found && policy == HookLeaveOthers:
		res.State = HookForeign
		return res, nil
	case !found:
		res.After = hookShebang + region
		res.Backup = path + hookBackupSuffix
	case parsed.edited() && policy == HookLeaveOthers:
		res.State = HookHandEdited
		return res, nil
	default:
		res.After = parsed.head + region + parsed.tail
	}
	if res.After == current {
		res.State, res.Backup = HookFresh, ""
		return res, nil
	}
	res.State = HookWritten
	if mode == ApplyDryRun {
		return res, nil
	}
	if res.Backup != "" {
		// O_EXCL: a backup already there may be the only copy of an earlier
		// hook, so it is never overwritten.
		kept, err := createIfMissing(res.Backup, []byte(current))
		if err != nil {
			return res, err
		}
		if !kept {
			return res, fmt.Errorf("%s already exists; move it aside to replace %s", res.Backup, path)
		}
	}
	return res, writeHook(path, res.After)
}

// hookRegion is the marked region the modules' commands produce.
func (r *Repo) hookRegion() string {
	var body strings.Builder
	body.WriteString(hookPreamble)
	for _, cmd := range r.PreCommitCommands() {
		body.WriteString(cmd + "\n")
	}
	body.WriteString(hookCall + "\n")
	return "# agents:begin pre-commit@" + module.Hash(body.String()) + "\n" + body.String() + hookEnd + "\n"
}

// parsedHook is a hook split around its one region.
type parsedHook struct {
	head, body, hash, tail string
}

// edited reports whether the region's body no longer matches its marker.
func (p parsedHook) edited() bool { return module.Hash(p.body) != p.hash }

// parseHook finds the hook's region: exactly one begin marker, then an end
// marker. Anything else — no markers, two regions, an unclosed one — is not a
// hook the tool can rewrite in place, so it reads as one the tool did not write.
func parseHook(hook string) (parsedHook, bool) {
	lines := strings.SplitAfter(hook, "\n")
	begin, end := -1, -1
	var hash string
	for i, line := range lines {
		trimmed := strings.TrimSuffix(line, "\n")
		if m := hookBeginRE.FindStringSubmatch(trimmed); m != nil {
			if begin >= 0 {
				return parsedHook{}, false
			}
			begin, hash = i, m[1]
			continue
		}
		if trimmed == hookEnd {
			if begin < 0 || end >= 0 {
				return parsedHook{}, false
			}
			end = i
		}
	}
	if begin < 0 || end < 0 {
		return parsedHook{}, false
	}
	return parsedHook{
		head: strings.Join(lines[:begin], ""),
		body: strings.Join(lines[begin+1:end], ""),
		hash: hash,
		tail: strings.Join(lines[end+1:], ""),
	}, true
}

// writeHook writes the hook atomically and makes sure git can run it.
func writeHook(path, content string) error {
	if err := writeFile(path, content); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspecting %s: %w", path, err)
	}
	if err := os.Chmod(path, info.Mode().Perm()|hookExec); err != nil {
		return fmt.Errorf("making %s executable: %w", path, err)
	}
	return nil
}

// within reports whether path lies inside dir.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

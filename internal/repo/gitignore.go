package repo

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/agents/internal/module"
)

const (
	// GitignoreFile is the ignore file beside the manifest, repo-relative.
	GitignoreFile = ".gitignore"
	// agentsIgnoreHeader heads the block of lines the tool appends.
	agentsIgnoreHeader = "# agents"
)

// GitignoreResult is what Gitignore appended, or would append.
type GitignoreResult struct {
	// Path is the repo-relative file, whether or not anything was added.
	Path string
	// Added are the declared lines the file lacked, in manifest order.
	Added []string
}

// IgnoreLines is every line the repo's modules declare under ignore:, in
// manifest order, each once however many modules declare it.
func (r *Repo) IgnoreLines() []string {
	return r.declared(func(m module.Module) []string { return m.Ignore })
}

// declared gathers one declaration from every enabled module, in manifest
// order, dropping repeats: two modules asking for the same line or command
// want it once.
func (r *Repo) declared(pick func(module.Module) []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range r.mods {
		for _, v := range pick(m) {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// Gitignore appends to the repo's .gitignore every declared line it lacks,
// creating the file when there is something to put in it.
//
// It only ever adds: a line is never removed or moved, so a module that drops
// a line leaves it behind for a human, and a project that ignores the same
// path its own way is matched only by the exact line. New lines join the
// block under agentsIgnoreHeader that an earlier run wrote, or start one at the
// end of the file.
func (r *Repo) Gitignore(mode ApplyMode) (GitignoreResult, error) {
	res := GitignoreResult{Path: GitignoreFile}
	path := filepath.Join(r.Dir, GitignoreFile)
	current, _, err := readIfPresent(path)
	if err != nil {
		return res, err
	}
	have := make(map[string]bool)
	for line := range strings.Lines(current) {
		have[strings.TrimSpace(line)] = true
	}
	for _, line := range r.IgnoreLines() {
		if !have[line] {
			res.Added = append(res.Added, line)
		}
	}
	if len(res.Added) == 0 || mode == ApplyDryRun {
		return res, nil
	}
	return res, writeFile(path, withIgnoreLines(current, res.Added))
}

// withIgnoreLines is current with added inserted at the end of the tool's
// block — the header and the non-blank lines that follow it — or appended as
// a new block, a blank line apart from whatever the project wrote.
func withIgnoreLines(current string, added []string) string {
	lines := strings.SplitAfter(current, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
		lines[n-1] += "\n"
	}
	block := make([]string, len(added))
	for i, line := range added {
		block[i] = line + "\n"
	}

	if at := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == agentsIgnoreHeader }); at >= 0 {
		end := at + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		return strings.Join(slices.Insert(lines, end, block...), "")
	}
	if len(lines) > 0 {
		lines = append(lines, "\n")
	}
	lines = append(lines, agentsIgnoreHeader+"\n")
	return strings.Join(append(lines, block...), "")
}

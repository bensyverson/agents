package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bensyverson/agents/internal/render"
)

// ClaudeKind is what a repo's CLAUDE.md is.
type ClaudeKind int

const (
	// ClaudeAbsent means there is no CLAUDE.md.
	ClaudeAbsent ClaudeKind = iota
	// ClaudeLinked means CLAUDE.md is the symlink to AGENTS.md.
	ClaudeLinked
	// ClaudeOwnFile means CLAUDE.md is a regular file of the project's own,
	// which Claude Code reads instead of AGENTS.md. `agents migrate` adopts it.
	ClaudeOwnFile
	// ClaudeLinkedElsewhere means CLAUDE.md links somewhere other than
	// AGENTS.md, often to rules shared between projects. Neither init nor
	// migrate replaces it: copying it in would cut the project off from them.
	ClaudeLinkedElsewhere
	// ClaudeNotAFile means CLAUDE.md is a directory or another special file.
	ClaudeNotAFile
)

// ClaudeFileError says CLAUDE.md isn't the link to AGENTS.md, and what to do.
type ClaudeFileError struct {
	Dir  string
	Kind ClaudeKind
	// Target is where the link points, for ClaudeLinkedElsewhere.
	Target string
}

func (e *ClaudeFileError) Error() string {
	path := filepath.Join(e.Dir, ClaudeFile)
	switch e.Kind {
	case ClaudeOwnFile:
		return fmt.Sprintf("%s is a file of its own, so Claude Code would read it and not the rules in %s; "+
			"run `agents migrate` to move its text into %s, then run `agents init` again",
			path, AgentsFile, AgentsFile)
	case ClaudeLinkedElsewhere:
		return fmt.Sprintf("%s links to %s, not %s, and agents won't replace a link that may share rules with other projects; "+
			"move what you want to keep into %s, delete the link, then run `agents init` again",
			path, e.Target, AgentsFile, AgentsFile)
	default:
		return fmt.Sprintf("%s is neither a file nor a link; move it aside, then run `agents init` again", path)
	}
}

// inspectClaude reports what dir's CLAUDE.md is, and the link's target when
// it is one.
func inspectClaude(dir string) (ClaudeKind, string, error) {
	path := filepath.Join(dir, ClaudeFile)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ClaudeAbsent, "", nil
	case err != nil:
		return ClaudeAbsent, "", fmt.Errorf("inspecting %s: %w", path, err)
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return ClaudeAbsent, "", fmt.Errorf("reading link %s: %w", path, err)
		}
		if filepath.Clean(target) == AgentsFile {
			return ClaudeLinked, target, nil
		}
		return ClaudeLinkedElsewhere, target, nil
	case info.Mode().IsRegular():
		return ClaudeOwnFile, "", nil
	default:
		return ClaudeNotAFile, "", nil
	}
}

// claudeReady is nil when init may link CLAUDE.md: it is absent or already
// the link. Anything else is a ClaudeFileError, returned before init writes.
func claudeReady(dir string) error {
	kind, target, err := inspectClaude(dir)
	if err != nil {
		return err
	}
	if kind == ClaudeAbsent || kind == ClaudeLinked {
		return nil
	}
	return &ClaudeFileError{Dir: dir, Kind: kind, Target: target}
}

// MigrateResult says what migrate did.
type MigrateResult int

const (
	// MigrateNothing means CLAUDE.md was absent or already the link.
	MigrateNothing MigrateResult = iota
	// MigrateMoved means CLAUDE.md became AGENTS.md, as there was none.
	MigrateMoved
	// MigrateAppended means CLAUDE.md's text was added to AGENTS.md's head.
	MigrateAppended
)

func (m MigrateResult) String() string {
	switch m {
	case MigrateNothing:
		return "nothing to migrate"
	case MigrateMoved:
		return "moved"
	case MigrateAppended:
		return "appended"
	default:
		return fmt.Sprintf("MigrateResult(%d)", int(m))
	}
}

// MovedMarker heads the text migrate moves into an existing AGENTS.md, so a
// human can find it, fold it into the rules around it, and delete the marker.
const MovedMarker = "<!-- Moved from " + ClaudeFile + " by `agents migrate`: merge it into the rules above, then delete this line. -->"

// Migrate adopts a CLAUDE.md of the project's own so init can link it: with no
// AGENTS.md the file is renamed to it, otherwise its text goes at the end of
// AGENTS.md's project-owned head, above the first region. Either way CLAUDE.md
// then links to AGENTS.md. No text is lost, and merging the two stays a human
// step, as pruning a head does for init. It needs no manifest, because it runs
// before init.
func Migrate(dir string) (MigrateResult, error) {
	kind, target, err := inspectClaude(dir)
	if err != nil {
		return MigrateNothing, err
	}
	switch kind {
	case ClaudeAbsent, ClaudeLinked:
		return MigrateNothing, nil
	case ClaudeLinkedElsewhere, ClaudeNotAFile:
		return MigrateNothing, &ClaudeFileError{Dir: dir, Kind: kind, Target: target}
	}

	claudePath := filepath.Join(dir, ClaudeFile)
	agentsPath := filepath.Join(dir, AgentsFile)
	moving, err := agentsAbsentOrLinkedToClaude(agentsPath)
	if err != nil {
		return MigrateNothing, err
	}
	if moving {
		// Rename replaces an AGENTS.md -> CLAUDE.md link in the same step.
		if err := os.Rename(claudePath, agentsPath); err != nil {
			return MigrateNothing, fmt.Errorf("moving %s to %s: %w", claudePath, agentsPath, err)
		}
		if err := os.Symlink(AgentsFile, claudePath); err != nil {
			return MigrateMoved, fmt.Errorf("linking %s: %w", claudePath, err)
		}
		return MigrateMoved, nil
	}

	claude, err := os.ReadFile(claudePath)
	if err != nil {
		return MigrateNothing, fmt.Errorf("reading %s: %w", claudePath, err)
	}
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		return MigrateNothing, fmt.Errorf("reading %s: %w", agentsPath, err)
	}
	merged, err := insertIntoHead(string(agents), string(claude))
	if err != nil {
		return MigrateNothing, fmt.Errorf("%s: %w", agentsPath, err)
	}
	// AGENTS.md holds both texts before CLAUDE.md goes, so a failure between
	// the two steps leaves a copy, never a loss.
	if err := writeFile(agentsPath, merged); err != nil {
		return MigrateNothing, err
	}
	if err := replaceWithLink(dir); err != nil {
		return MigrateAppended, err
	}
	return MigrateAppended, nil
}

// agentsAbsentOrLinkedToClaude is true when CLAUDE.md can simply become
// AGENTS.md: there is none, or it is a link back to CLAUDE.md.
func agentsAbsentOrLinkedToClaude(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("inspecting %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false, fmt.Errorf("reading link %s: %w", path, err)
	}
	return filepath.Clean(target) == ClaudeFile, nil
}

// insertIntoHead puts moved text, under MovedMarker, at the end of doc's
// project-owned head: the text before its first region. It refuses a result
// that no longer parses, such as one where the moved text repeats a region.
func insertIntoHead(doc, moved string) (string, error) {
	parsed, err := render.Parse(doc)
	if err != nil {
		return "", err
	}
	head := ""
	if len(parsed.Segments) > 0 && parsed.Segments[0].Kind == render.SegmentText {
		head = parsed.Segments[0].Text
	}
	rest := doc[len(head):]

	block := MovedMarker + "\n\n" + strings.TrimRight(moved, "\n") + "\n"
	out := block
	if trimmed := strings.TrimRight(head, "\n"); trimmed != "" {
		out = trimmed + "\n\n" + block
	}
	if rest != "" {
		out += "\n" + rest
	}
	if _, err := render.Parse(out); err != nil {
		return "", fmt.Errorf("the text moved from %s would break the generated regions (%w); merge the two by hand", ClaudeFile, err)
	}
	return out, nil
}

// replaceWithLink swaps CLAUDE.md for the link to AGENTS.md in one rename, so
// there is no moment with neither.
func replaceWithLink(dir string) error {
	path := filepath.Join(dir, ClaudeFile)
	tmp := filepath.Join(dir, "."+ClaudeFile+".agents-migrate")
	_ = os.Remove(tmp)
	if err := os.Symlink(AgentsFile, tmp); err != nil {
		return fmt.Errorf("linking %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replacing %s with a link: %w", path, err)
	}
	return nil
}

// Package registry stores the list of managed repos — one absolute path per
// line in a plain text file, so a human can edit it with any editor. Blank
// lines and "#" comments are theirs to keep; the tool only ever appends or
// deletes whole lines.
package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// dirPerm and filePerm are what the registry is created with.
	dirPerm  fs.FileMode = 0o755
	filePerm fs.FileMode = 0o644

	// commentPrefix marks a line the tool ignores and preserves.
	commentPrefix = "#"

	configDirName   = ".config"
	appDirName      = "agents"
	registryName    = "repos"
	xdgConfigEnvVar = "XDG_CONFIG_HOME"
)

// DefaultPath is $XDG_CONFIG_HOME/agents/repos, or ~/.config/agents/repos when
// that variable is unset or empty.
func DefaultPath() (string, error) {
	if base := os.Getenv(xdgConfigEnvVar); base != "" {
		return filepath.Join(base, appDirName, registryName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, configDirName, appDirName, registryName), nil
}

// Read returns the registered repo paths in file order, cleaned with
// filepath.Clean, skipping blank and comment lines. A missing file is an empty
// registry, not an error.
func Read(path string) ([]string, error) {
	lines, ok, err := readLines(path)
	if err != nil || !ok {
		return nil, err
	}
	var repos []string
	for _, line := range lines {
		if repo, ok := entryOf(line); ok {
			repos = append(repos, filepath.Clean(repo))
		}
	}
	return repos, nil
}

// Add appends repo to the registry, creating the file and its parents if
// needed, and reports whether it was added. It is idempotent: a repo already
// listed (compared after filepath.Clean) leaves the file byte-identical.
// repo must be absolute.
func Add(path, repo string) (bool, error) {
	repo, err := cleanRepo(repo)
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(repo) {
		return false, fmt.Errorf("registry: repo path must be absolute, got %q", repo)
	}
	lines, _, err := readLines(path)
	if err != nil {
		return false, err
	}
	for _, line := range lines {
		if existing, ok := entryOf(line); ok && filepath.Clean(existing) == repo {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return false, fmt.Errorf("creating registry directory: %w", err)
	}
	if err := writeLines(path, append(lines, repo)); err != nil {
		return false, err
	}
	return true, nil
}

// Remove deletes every line naming repo (compared after filepath.Clean) and
// reports whether anything went. Comments, blank lines and the other entries
// survive. A missing file is not an error and removes nothing.
func Remove(path, repo string) (bool, error) {
	repo, err := cleanRepo(repo)
	if err != nil {
		return false, err
	}
	lines, ok, err := readLines(path)
	if err != nil || !ok {
		return false, err
	}
	kept := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if existing, ok := entryOf(line); ok && filepath.Clean(existing) == repo {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return false, nil
	}
	if err := writeLines(path, kept); err != nil {
		return false, err
	}
	return true, nil
}

func cleanRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", errors.New("registry: empty repo path")
	}
	return filepath.Clean(repo), nil
}

// entryOf reports whether a raw line names a repo, and returns it trimmed.
func entryOf(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, commentPrefix) {
		return "", false
	}
	return trimmed, true
}

// readLines returns the file's lines without their terminators, and whether the
// file exists. A trailing newline does not produce a final empty line, so
// writeLines round-trips it.
func readLines(path string) ([]string, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading registry %s: %w", path, err)
	}
	text := string(content)
	if text == "" {
		return nil, true, nil
	}
	text = strings.TrimSuffix(text, "\n")
	return strings.Split(text, "\n"), true, nil
}

func writeLines(path string, lines []string) error {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), filePerm); err != nil {
		return fmt.Errorf("writing registry %s: %w", path, err)
	}
	return nil
}

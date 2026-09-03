package source

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// Git opens a git source from the cache. It never runs git: Fetch has already
// written the ref's sha and the tree it names, so a missing cache is
// ErrNotFetched and the caller tells the user to run `agents sync`.
func (l *Loader) Git(name, url, ref string) (Source, error) {
	sha, ok := l.cachedSha(url, ref)
	if !ok {
		return Source{}, fmt.Errorf("source %q at %q: %w", name, ref, ErrNotFetched)
	}
	dir := l.commitDir(url, sha)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Source{}, fmt.Errorf("source %q at %q: %w", name, ref, ErrNotFetched)
	}
	return open(name, os.DirFS(dir), sha)
}

// cachedSha maps a ref to the commit it was last fetched at, reading only the
// filesystem. A ref pinned to a full sha needs no record: `agents update`
// rewrites a branch to the sha it resolved to, and that tree is already here.
func (l *Loader) cachedSha(url, ref string) (string, bool) {
	if body, err := os.ReadFile(l.refFile(url, ref)); err == nil {
		if sha := strings.TrimSpace(string(body)); sha != "" {
			return sha, true
		}
	}
	if isFullSha(ref) {
		return ref, true
	}
	return "", false
}

// Resolve returns the full commit sha a branch, tag or (possibly abbreviated)
// sha names in url. An empty ref means the repository's default branch. It
// updates the cached clone, so it needs access to url.
func (l *Loader) Resolve(url, ref string) (string, error) {
	if err := l.mirror(url); err != nil {
		return "", err
	}
	return l.resolveCached(url, ref)
}

// RemoteHeadRef asks a remote which ref its HEAD points at — its default
// branch as it stands now, as a full refname like "refs/heads/main".
//
// Resolve with an empty ref reads the cached mirror's HEAD instead, and `git
// fetch` never refreshes that: a source that moved its default branch after
// the clone would keep resolving the old one forever. `agents update` takes
// its default from here so the pin follows the remote, not the cache.
//
// A remote that reports no symref line — an old server, or a detached HEAD —
// falls back to the commit HEAD names, which pins the same tree.
func RemoteHeadRef(url string) (string, error) {
	out, err := runGit("", "ls-remote", "--symref", "--end-of-options", url, "HEAD")
	if err != nil {
		return "", fmt.Errorf("reading HEAD from %s: %w", url, err)
	}
	var head string
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" {
			return fields[1], nil
		}
		if len(fields) == 2 && fields[1] == "HEAD" {
			head = fields[0]
		}
	}
	if head != "" {
		return head, nil
	}
	return "", fmt.Errorf("%w: HEAD in %s", ErrUnknownRef, url)
}

// Fetch populates the cache so that Git can load url at ref offline, and
// returns the commit sha the ref resolved to.
func (l *Loader) Fetch(url, ref string) (string, error) {
	if err := l.mirror(url); err != nil {
		return "", err
	}
	sha, err := l.resolveCached(url, ref)
	if err != nil {
		return "", err
	}
	if err := l.extract(url, sha); err != nil {
		return "", err
	}
	if err := writeFile(l.refFile(url, ref), sha); err != nil {
		return "", fmt.Errorf("recording %q in the cache: %w", ref, err)
	}
	return sha, nil
}

// resolveCached asks the cached clone what a ref names. Peeling to ^{commit}
// turns an annotated tag into the commit it points at.
func (l *Loader) resolveCached(url, ref string) (string, error) {
	rev := ref
	if rev == "" {
		// The mirror's HEAD: current git moves it on fetch when the remote's
		// default branch changes, older gits do not. `agents update` asks the
		// remote directly (RemoteHeadRef) and writes a sha, so a stale HEAD
		// here only affects a repeat of the same unpinned request.
		rev = "HEAD"
	}
	sha, err := runGit(l.mirrorDir(url), "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%w: %q in %s", ErrUnknownRef, ref, url)
	}
	return sha, nil
}

// mirror makes sure a bare clone of url is in the cache and current. A full
// clone rather than a shallow one: sources are small, and fetching a bare
// server by sha needs uploadpack.allowReachableSHA1InWant, which a plain local
// repository does not set.
func (l *Loader) mirror(url string) error {
	dir := l.mirrorDir(url)
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err == nil {
		if _, err := runGit(dir, "fetch", "--prune", "origin"); err != nil {
			return fmt.Errorf("fetching %s: %w", url, err)
		}
		return nil
	}
	root := l.sourceDir(url)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("preparing the cache: %w", err)
	}
	tmp, err := os.MkdirTemp(root, "clone-")
	if err != nil {
		return fmt.Errorf("preparing the cache: %w", err)
	}
	defer os.RemoveAll(tmp)

	into := filepath.Join(tmp, "git")
	if _, err := runGit(root, "clone", "--mirror", "--", url, into); err != nil {
		return fmt.Errorf("cloning %s: %w", url, err)
	}
	if err := os.Rename(into, dir); err != nil {
		return fmt.Errorf("populating the cache for %s: %w", url, err)
	}
	return nil
}

// extract writes one commit's tree into the cache so that loading it later
// needs no git at all. It is a no-op once the tree is there.
func (l *Loader) extract(url, sha string) error {
	dest := l.commitDir(url, sha)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return nil
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("preparing the cache: %w", err)
	}
	archive, err := runGitOutput(l.mirrorDir(url), "archive", "--format=tar", "--end-of-options", sha)
	if err != nil {
		return fmt.Errorf("reading %s from %s: %w", sha, url, err)
	}
	tmp, err := os.MkdirTemp(parent, "extract-")
	if err != nil {
		return fmt.Errorf("preparing the cache: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := untar(bytes.NewReader(archive), tmp); err != nil {
		return fmt.Errorf("unpacking %s from %s: %w", sha, url, err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return fmt.Errorf("preparing the cache: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("populating the cache for %s: %w", url, err)
	}
	return nil
}

// untar unpacks a git archive. Only directories and regular files are written:
// a source is prose, and a symlink in one could point out of the cache.
func untar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name, ok := safePath(hdr.Name)
		if !ok {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := fs.FileMode(hdr.Mode).Perm() | 0o600
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

// safePath rejects any archive entry that would write outside the destination.
func safePath(name string) (string, bool) {
	if name == "" || path.IsAbs(name) || filepath.IsAbs(name) {
		return "", false
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

// writeFile writes body to path, creating the parent directory.
func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body+"\n"), 0o644)
}

// isFullSha reports whether ref is a complete object name rather than a
// symbolic ref, for either hash size git supports.
func isFullSha(ref string) bool {
	if len(ref) != 40 && len(ref) != 64 {
		return false
	}
	for _, r := range ref {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// runGit runs git in dir and returns its trimmed standard output. The user's
// git configuration is deliberately left alone: credential helpers, SSH
// aliases and url.insteadOf rewrites are how a private source is reachable at
// all. Only the terminal prompt is suppressed, because the tool may run
// without a terminal to answer it.
func runGit(dir string, args ...string) (string, error) {
	out, err := runGitOutput(dir, args...)
	return strings.TrimSpace(string(out)), err
}

// runGitOutput is runGit for a command whose output is binary.
func runGitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, detail)
	}
	return stdout.Bytes(), nil
}

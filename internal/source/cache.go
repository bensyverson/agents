package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Environment variables that place the cache.
const (
	CacheEnv = "AGENTS_CACHE"
	XDGEnv   = "XDG_CACHE_HOME"
)

// cacheName is the directory the tool owns under whichever cache root applies.
const cacheName = "agents"

// DefaultCacheDir is $AGENTS_CACHE, else $XDG_CACHE_HOME/agents, else the
// platform's user cache directory plus /agents.
func DefaultCacheDir() (string, error) {
	if dir := os.Getenv(CacheEnv); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv(XDGEnv); dir != "" {
		return filepath.Join(dir, cacheName), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locating the cache directory: %w", err)
	}
	return filepath.Join(base, cacheName), nil
}

// Loader loads sources, caching git ones under a cache directory.
type Loader struct {
	cacheDir string
}

// New returns a loader that caches under cacheDir. The directory is created on
// first fetch; a path or embedded source never touches it.
func New(cacheDir string) *Loader {
	return &Loader{cacheDir: cacheDir}
}

// CacheDir is the directory this loader caches git sources under.
func (l *Loader) CacheDir() string { return l.cacheDir }

// Path opens a source held in a local directory. It is read directly and never
// cached, so an edit under dir is visible on the next render.
func (l *Loader) Path(name, dir string) (Source, error) {
	return open(name, os.DirFS(dir), "")
}

// sourceDir is where one repository's cache lives. The URL is hashed rather
// than sanitised so that any URL — ssh, https, a local path — yields one flat,
// collision-free directory name; two spellings of the same repository are two
// entries, which costs a fetch and never a wrong answer.
func (l *Loader) sourceDir(url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(l.cacheDir, "sources", hex.EncodeToString(sum[:]))
}

// mirrorDir holds the bare clone of the repository.
func (l *Loader) mirrorDir(url string) string {
	return filepath.Join(l.sourceDir(url), "git")
}

// commitDir holds one commit's tree, extracted so that loading needs no git.
func (l *Loader) commitDir(url, sha string) string {
	return filepath.Join(l.sourceDir(url), "commits", sha)
}

// refFile records the sha a ref resolved to at fetch time, so that a manifest
// pinned to a branch or tag can be loaded without asking git anything. The
// file is named by the hash of the ref because a ref may contain a slash.
func (l *Loader) refFile(url, ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return filepath.Join(l.sourceDir(url), "refs", hex.EncodeToString(sum[:]))
}

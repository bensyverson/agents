package repo

import (
	"maps"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/manifest"
)

// The declaring fixture: two modules that each name ignore lines and
// pre-commit commands, overlapping on one of each, so the tests see order and
// de-duplication across modules. The commands are arbitrary text — the tool
// must not know what any of them means.
const (
	lintCmd    = "lint --strict"
	fmtCmd     = `test -z "$(fmt -l .)"`
	toolCmd    = "tool test"
	scratchDir = ".scratch/"
	outDir     = "out/"
	cacheDir   = ".cache/"
	// declaringManifest enables both declaring modules, lint first.
	declaringManifest = "modules:\n  - lint\n  - tools\n"
)

// declaringSourceFS is the test source plus the two declaring modules.
func declaringSourceFS() fstest.MapFS {
	fsys := testSourceFS()
	fsys["modules/lint.md"] = &fstest.MapFile{Data: []byte(
		"---\nignore: [" + scratchDir + ", " + outDir + "]\n" +
			"pre-commit:\n  - " + lintCmd + "\n  - '" + fmtCmd + "'\n---\nlint rules\n")}
	fsys["modules/tools.md"] = &fstest.MapFile{Data: []byte(
		"---\nkind: file\nignore: [" + outDir + ", " + cacheDir + "]\n" +
			"pre-commit: [" + lintCmd + ", " + toolCmd + "]\n---\n# Tools\n")}
	return fsys
}

// declaringRepo lays out a repo enabling the declaring modules, plus files.
func declaringRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	all := map[string]string{manifest.FileName: declaringManifest}
	maps.Copy(all, files)
	return newRepo(t, all)
}

// openDeclaring opens a repo against the declaring source.
func openDeclaring(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Open(dir, Options{Embedded: declaringSourceFS()})
	if err != nil {
		t.Fatalf("Open(%s): %v", dir, err)
	}
	return r
}

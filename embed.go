// Package agents embeds the shared modules and templates so the binary is
// self-contained: the tool version is the standards version.
package agents

import (
	"embed"
	"io/fs"
)

//go:embed modules/*.md
var modulesFS embed.FS

// The whole tree, not templates/*.md: a seed's template lives at the path the
// module declares (templates/project/gotchas.md), so it may be nested.
//
//go:embed templates
var templatesFS embed.FS

// The example source, whole: a source is a directory with modules/ and
// templates/ at its root, so both subtrees are embedded together.
//
//go:embed example
var exampleFS embed.FS

// Example is the embedded example source — the one source the binary carries
// itself — rooted so that "modules/agents.md" and "templates/head.md" are its
// paths.
func Example() fs.FS { return mustSub(exampleFS, "example") }

// Modules is the embedded module directory, rooted so that "core.md" is a top-level entry.
func Modules() fs.FS { return mustSub(modulesFS, "modules") }

// Templates is the embedded template directory (files seeded into managed repos).
func Templates() fs.FS { return mustSub(templatesFS, "templates") }

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

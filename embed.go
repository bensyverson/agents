// Package agents embeds the shared modules and templates so the binary is
// self-contained: the tool version is the standards version.
package agents

import (
	"embed"
	"io/fs"
)

// The first-party tree, embedded whole rather than as two subtrees: modules/
// and templates/ at the root is exactly the shape of a source, so the binary
// carries its rules the way every other source supplies them. Templates are
// embedded whole and not as templates/*.md because a seed's template lives at
// the path the module declares (templates/project/gotchas.md), so it may be
// nested.
//
//go:embed modules templates
var embeddedFS embed.FS

// The example source, whole: a source is a directory with modules/ and
// templates/ at its root, so both subtrees are embedded together.
//
//go:embed example
var exampleFS embed.FS

// Embedded is the source the binary carries — what a manifest with no
// sources: block picks every module from, under the implicit name
// manifest.ExampleSource. Its root holds "modules" and "templates".
func Embedded() fs.FS { return embeddedFS }

// Example is the embedded example source, rooted so that "modules/agents.md"
// and "templates/head.md" are its paths. Until the first-party rules move to
// their own repository, Embedded is the first-party tree and this is the
// example the format documents itself with.
func Example() fs.FS { return mustSub(exampleFS, "example") }

// Modules is the embedded module directory, rooted so that "core.md" is a top-level entry.
func Modules() fs.FS { return mustSub(embeddedFS, "modules") }

// Templates is the embedded template directory (files seeded into managed repos).
func Templates() fs.FS { return mustSub(embeddedFS, "templates") }

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

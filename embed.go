// Package agents embeds the example source so the binary is self-contained:
// a fresh install renders something, offline, before it names a source of its
// own. The maintained rule sets live in their own repositories and are reached
// through `sources:` in .agents.yaml like anybody else's.
package agents

import (
	"embed"
	"io/fs"
)

// The example source, embedded whole rather than as two subtrees: modules/ and
// templates/ at the root is exactly the shape of a source, so the binary
// carries its rules the way every other source supplies them. Templates are
// embedded whole and not as templates/*.md because a seed's template lives at
// the path the module declares (templates/project/gotchas.md), so it may be
// nested.
//
//go:embed example
var embeddedFS embed.FS

// Embedded is the source the binary carries — what a manifest with no
// sources: block picks every module from, under the implicit name
// manifest.ExampleSource. Its root holds "modules" and "templates".
func Embedded() fs.FS { return mustSub(embeddedFS, "example") }

// Modules is the embedded module directory, rooted so that "agents.md" is a
// top-level entry.
func Modules() fs.FS { return mustSub(embeddedFS, "example/modules") }

// Templates is the embedded template directory (files seeded into managed repos).
func Templates() fs.FS { return mustSub(embeddedFS, "example/templates") }

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

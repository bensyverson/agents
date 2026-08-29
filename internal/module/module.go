// Package module loads the shared rule modules — markdown files with optional
// frontmatter — and computes their content hashes.
package module

// Kind says where a module renders: inline in AGENTS.md, or to its own file.
type Kind int

const (
	// KindInline renders between markers inside AGENTS.md (the default).
	KindInline Kind = iota
	// KindFile renders to Path (e.g. project/agents/delegation.md).
	KindFile
)

// Module is one shared rule set, ready to render.
type Module struct {
	// Name is the file name without ".md" and the token used in markers and .agents.yaml.
	Name string
	Kind Kind
	// Path is the repo-relative render target; set only for KindFile.
	Path string
	// Seeds are repo-relative files the module wants a managed repo to have.
	// Each is created from templates/<seed> when it is missing, and never
	// overwritten: once it exists it is the repo's own. Valid on any kind.
	Seeds []string
	// Body is the markdown with any leading frontmatter removed. It always ends in "\n".
	Body string
	// Hash is the first 6 hex of SHA-256 over Body.
	Hash string
}

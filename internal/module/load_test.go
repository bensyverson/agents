package module_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/module"
)

// embeddedModules is the module subtree of the source the binary embeds — the
// example source, which is what a repo with no sources: block renders from.
func embeddedModules(t *testing.T) fs.FS {
	t.Helper()
	return agents.Modules()
}

func TestParseWithoutFrontmatter(t *testing.T) {
	raw := "## Working rules\n\nBe good.\n"
	m, err := module.Parse("core", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != "core" {
		t.Errorf("Name = %q, want %q", m.Name, "core")
	}
	if m.Kind != module.KindInline {
		t.Errorf("Kind = %v, want KindInline", m.Kind)
	}
	if m.Path != "" {
		t.Errorf("Path = %q, want empty", m.Path)
	}
	if m.Body != raw {
		t.Errorf("Body = %q, want %q", m.Body, raw)
	}
	if want := module.Hash(raw); m.Hash != want {
		t.Errorf("Hash = %q, want %q", m.Hash, want)
	}
}

func TestParseFileFrontmatter(t *testing.T) {
	raw := "---\nkind: file\n---\n# Delegating\n\nBody.\n"
	m, err := module.Parse("delegation", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Kind != module.KindFile {
		t.Errorf("Kind = %v, want KindFile", m.Kind)
	}
	if want := "project/agents/delegation.md"; m.Path != want {
		t.Errorf("Path = %q, want %q (derived from the name)", m.Path, want)
	}
	if want := "# Delegating\n\nBody.\n"; m.Body != want {
		t.Errorf("Body = %q, want %q", m.Body, want)
	}
	if want := module.Hash("# Delegating\n\nBody.\n"); m.Hash != want {
		t.Errorf("Hash = %q, want %q — the frontmatter must be excluded", m.Hash, want)
	}
}

// Path is derived from the module's name, not read from frontmatter: two
// different names must yield two different paths under FileDir.
func TestParseDerivesPathFromName(t *testing.T) {
	tests := []struct{ name, want string }{
		{"delegation", "project/agents/delegation.md"},
		{"cli-design", "project/agents/cli-design.md"},
	}
	for _, tt := range tests {
		m, err := module.Parse(tt.name, "---\nkind: file\n---\nBody.\n")
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.name, err)
		}
		if m.Path != tt.want {
			t.Errorf("Parse(%q).Path = %q, want %q", tt.name, m.Path, tt.want)
		}
	}
}

// A file module is valid with no when: at all — it just won't appear in the
// index later.
func TestParseFileModuleWithoutWhen(t *testing.T) {
	m, err := module.Parse("x", "---\nkind: file\n---\nBody.\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.When != "" {
		t.Errorf("When = %q, want empty", m.When)
	}
}

func TestParseWhenOnFileModule(t *testing.T) {
	raw := "---\nkind: file\nwhen: Before dispatching any subagent\n---\nBody.\n"
	m, err := module.Parse("delegation", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := "Before dispatching any subagent"; m.When != want {
		t.Errorf("When = %q, want %q", m.When, want)
	}
	if want := "project/agents/delegation.md"; m.Path != want {
		t.Errorf("Path = %q, want %q", m.Path, want)
	}
}

func TestParseExplicitInlineFrontmatter(t *testing.T) {
	m, err := module.Parse("core", "---\nkind: inline\n---\nBody.\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Kind != module.KindInline {
		t.Errorf("Kind = %v, want KindInline", m.Kind)
	}
	if m.Body != "Body.\n" {
		t.Errorf("Body = %q, want %q", m.Body, "Body.\n")
	}
}

func TestParseEmptyFrontmatter(t *testing.T) {
	m, err := module.Parse("core", "---\n---\nBody.\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Kind != module.KindInline {
		t.Errorf("Kind = %v, want KindInline", m.Kind)
	}
	if m.Body != "Body.\n" {
		t.Errorf("Body = %q, want %q", m.Body, "Body.\n")
	}
}

// A --- pair that is not at the very start of the file is content, not
// frontmatter — the hand-render's sed was sloppier than this.
func TestParseLaterDashPairStaysInBody(t *testing.T) {
	raw := "# Title\n\n---\nkind: file\n---\n\nStill body.\n"
	m, err := module.Parse("docs", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Kind != module.KindInline {
		t.Errorf("Kind = %v, want KindInline", m.Kind)
	}
	if m.Body != raw {
		t.Errorf("Body = %q, want the whole file %q", m.Body, raw)
	}
}

func TestParseGuaranteesTrailingNewline(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"appends when missing", "Body.", "Body.\n"},
		{"leaves a single newline alone", "Body.\n", "Body.\n"},
		{"leaves extra blank lines alone", "Body.\n\n\n", "Body.\n\n\n"},
		{"after frontmatter", "---\nkind: inline\n---\nBody.", "Body.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := module.Parse("x", tt.raw)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if m.Body != tt.want {
				t.Errorf("Body = %q, want %q", m.Body, tt.want)
			}
			if want := module.Hash(tt.want); m.Hash != want {
				t.Errorf("Hash = %q, want %q (hash covers the normalised body)", m.Hash, want)
			}
		})
	}
}

func TestParseSeeds(t *testing.T) {
	raw := "---\nseeds: [project/gotchas.md, project/backlog.md]\n---\n## Working rules\n"
	m, err := module.Parse("core", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"project/gotchas.md", "project/backlog.md"}
	if !reflect.DeepEqual(m.Seeds, want) {
		t.Errorf("Seeds = %v, want %v", m.Seeds, want)
	}
	if m.Kind != module.KindInline {
		t.Errorf("Kind = %v, want KindInline (seeds say nothing about kind)", m.Kind)
	}
	if body := "## Working rules\n"; m.Body != body {
		t.Errorf("Body = %q, want %q", m.Body, body)
	}
	if want := module.Hash("## Working rules\n"); m.Hash != want {
		t.Errorf("Hash = %q, want %q — seeds must not change the body hash", m.Hash, want)
	}
}

// Seeds are valid on any kind: a file module may carry a template too.
func TestParseSeedsOnFileModule(t *testing.T) {
	raw := "---\nkind: file\nseeds:\n  - project/backlog.md\n---\n# Jobs\n"
	m, err := module.Parse("jobs", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Kind != module.KindFile || m.Path != "project/agents/jobs.md" {
		t.Errorf("got %+v, want KindFile at project/agents/jobs.md", m)
	}
	if want := []string{"project/backlog.md"}; !reflect.DeepEqual(m.Seeds, want) {
		t.Errorf("Seeds = %v, want %v", m.Seeds, want)
	}
}

func TestParseWithoutSeeds(t *testing.T) {
	m, err := module.Parse("core", "## Working rules\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Seeds) != 0 {
		t.Errorf("Seeds = %v, want none", m.Seeds)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{"unterminated frontmatter", "---\nkind: file\npath: a.md\n", module.ErrUnterminatedFrontmatter},
		{"unknown frontmatter key", "---\nkind: inline\nnope: 1\n---\nBody.\n", nil},
		{"path on an inline module", "---\npath: project/agents/x.md\n---\nBody.\n", module.ErrPathNotAllowed},
		{"explicit inline with a path", "---\nkind: inline\npath: x.md\n---\nBody.\n", module.ErrPathNotAllowed},
		{"path on a file module", "---\nkind: file\npath: project/agents/x.md\n---\nBody.\n", module.ErrPathNotAllowed},
		{"when on an inline module", "---\nwhen: Before doing X\n---\nBody.\n", module.ErrWhenNotAllowed},
		{"when on an explicit inline module", "---\nkind: inline\nwhen: Before doing X\n---\nBody.\n", module.ErrWhenNotAllowed},
		{"unknown kind", "---\nkind: sideways\n---\nBody.\n", module.ErrUnknownKind},
		{"empty body", "", module.ErrEmptyBody},
		{"whitespace-only body", "\n\n  \n", module.ErrEmptyBody},
		{"frontmatter with no body", "---\nkind: inline\n---\n", module.ErrEmptyBody},
		{"malformed frontmatter yaml", "---\nkind: [\n---\nBody.\n", nil},
		{"absolute seed path", "---\nseeds: [/etc/passwd]\n---\nBody.\n", module.ErrSeedPathInvalid},
		{"seed path escaping the repo", "---\nseeds: [../elsewhere/gotchas.md]\n---\nBody.\n", module.ErrSeedPathInvalid},
		{"seed path escaping via a subdirectory", "---\nseeds: [project/../../gotchas.md]\n---\nBody.\n", module.ErrSeedPathInvalid},
		{"empty seed path", "---\nseeds: [\"\"]\n---\nBody.\n", module.ErrSeedPathInvalid},
		{"whitespace around an absolute seed path", "---\nseeds: [\" /etc/passwd \"]\n---\nBody.\n", module.ErrSeedPathInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := module.Parse("x", tt.raw)
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want one", tt.raw)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Parse(%q) error = %v, want %v", tt.raw, err, tt.want)
			}
		})
	}
}

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"core.md":        &fstest.MapFile{Data: []byte("## Working rules\n")},
		"principles.md":  &fstest.MapFile{Data: []byte("## Principles\n")},
		"delegation.md":  &fstest.MapFile{Data: []byte("---\nkind: file\n---\n# Delegating\n")},
		"README.txt":     &fstest.MapFile{Data: []byte("not a module\n")},
		"notes/extra.md": &fstest.MapFile{Data: []byte("nested, ignored\n")},
	}
}

func TestLoad(t *testing.T) {
	set, err := module.Load(testFS())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"core", "delegation", "principles"}
	if got := set.Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v (sorted, root *.md only)", got, want)
	}
	m, ok := set.Get("delegation")
	if !ok {
		t.Fatal("Get(delegation) = not found")
	}
	if m.Kind != module.KindFile || m.Path != "project/agents/delegation.md" {
		t.Errorf("delegation = %+v, want KindFile at project/agents/delegation.md", m)
	}
	if m.Body != "# Delegating\n" {
		t.Errorf("delegation body = %q", m.Body)
	}
	if _, ok := set.Get("extra"); ok {
		t.Error("Get(extra) found a module from a subdirectory; Load must not recurse")
	}
	if _, ok := set.Get("README"); ok {
		t.Error("Get(README) found a non-markdown file")
	}
	if _, ok := set.Get("nope"); ok {
		t.Error("Get(nope) reported an unknown module as present")
	}
}

func TestLoadErrorNamesTheFile(t *testing.T) {
	bad := fstest.MapFS{
		"core.md":   &fstest.MapFile{Data: []byte("## Working rules\n")},
		"broken.md": &fstest.MapFile{Data: []byte("---\nkind: sideways\n---\nBody.\n")},
	}
	_, err := module.Load(bad)
	if err == nil {
		t.Fatal("Load = nil error, want one")
	}
	if !errors.Is(err, module.ErrUnknownKind) {
		t.Errorf("error = %v, want it to wrap ErrUnknownKind", err)
	}
	if got := err.Error(); !strings.Contains(got, "broken.md") {
		t.Errorf("error = %q, want it to name broken.md", got)
	}
}

func TestNamesIsACopy(t *testing.T) {
	set, err := module.Load(testFS())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	names := set.Names()
	names[0] = "clobbered"
	if got := set.Names()[0]; got != "core" {
		t.Errorf("Names()[0] = %q after the caller mutated a previous result, want %q", got, "core")
	}
}

// Load works over a real directory the same way, ignoring anything that is
// not a module file: os.DirFS is how every caller reaches one.
func TestLoadFromADirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "core.md"), "## Working rules\n")
	writeFile(t, filepath.Join(dir, "README.txt"), "ignored\n")

	set, err := module.Load(os.DirFS(dir))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := set.Names(), []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
}

func TestLoadFromAMissingDirectory(t *testing.T) {
	if _, err := module.Load(os.DirFS(filepath.Join(t.TempDir(), "nope"))); err == nil {
		t.Fatal("Load(missing directory) = nil error, want one")
	}
}

// The embedded example source carries one kind: file module, and its path is
// derived rather than declared.
func TestEmbeddedFileModulePaths(t *testing.T) {
	set, err := module.Load(embeddedModules(t))
	if err != nil {
		t.Fatalf("Load(embedded): %v", err)
	}
	found := false
	for _, name := range set.Names() {
		m, _ := set.Get(name)
		if m.Kind != module.KindFile {
			continue
		}
		found = true
		if want := module.FileDir + "/" + name + module.Extension; m.Path != want {
			t.Errorf("module %q Path = %q, want %q", name, m.Path, want)
		}
	}
	if !found {
		t.Fatal("no kind: file module found in the embedded set")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

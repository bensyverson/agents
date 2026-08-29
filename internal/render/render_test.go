package render

import (
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

func TestRender(t *testing.T) {
	a := mod("core", "alpha\n")
	b := mod("principles", "beta\n")
	c := mod("go", "gamma\n")

	tests := []struct {
		name string
		doc  string
		mods []module.Module
		want string
	}{
		{
			"empty document, one module",
			"",
			[]module.Module{a},
			wantRegion(a),
		},
		{
			"empty document, regions separated by one blank line",
			"",
			[]module.Module{a, b},
			wantRegion(a) + "\n" + wantRegion(b),
		},
		{
			"head with no regions gets a blank line then the regions",
			"# Foo\n",
			[]module.Module{a},
			"# Foo\n\n" + wantRegion(a),
		},
		{
			"head without a trailing newline is terminated first",
			"# Foo",
			[]module.Module{a},
			"# Foo\n\n" + wantRegion(a),
		},
		{
			"stale markers are refreshed and text preserved byte for byte",
			"# H\n\n" + markedRegion("core", "000000", "old body\n") + "\ntail\n",
			[]module.Module{a},
			"# H\n\n" + wantRegion(a) + "\ntail\n",
		},
		{
			"hand-edited body is overwritten",
			markedRegion("core", a.Hash, "hand written\n"),
			[]module.Module{a},
			wantRegion(a),
		},
		{
			"slots are filled in module order and the extra slot is dropped",
			wantRegion(a) + "\n" + wantRegion(b) + "\n" + wantRegion(c) + "\n",
			[]module.Module{c, a},
			wantRegion(c) + "\n" + wantRegion(a) + "\n",
		},
		{
			"removing an orphan drops one following blank line",
			"# H\n\n" + wantRegion(a) + "\n" + wantRegion(b) + "\n",
			[]module.Module{a},
			"# H\n\n" + wantRegion(a) + "\n",
		},
		{
			"removing an orphan leaves non-blank following text alone",
			"# H\n\n" + wantRegion(a) + "\n" + wantRegion(b) + "tail\n",
			[]module.Module{a},
			"# H\n\n" + wantRegion(a) + "\ntail\n",
		},
		{
			"new module is appended after the last region, separated by a blank line",
			"# H\n\n" + wantRegion(a) + "\n",
			[]module.Module{a, b},
			"# H\n\n" + wantRegion(a) + "\n" + wantRegion(b) + "\n",
		},
		{
			"new modules are appended in module order with no trailing text",
			"# H\n\n" + wantRegion(a),
			[]module.Module{a, b, c},
			"# H\n\n" + wantRegion(a) + "\n" + wantRegion(b) + "\n" + wantRegion(c),
		},
		{
			"a renamed module replaces the orphan slot without doubling the gap",
			"# H\n\n" + wantRegion(a) + "\n",
			[]module.Module{b},
			"# H\n\n" + wantRegion(b),
		},
		{
			"no modules at all removes every region",
			"# H\n\n" + wantRegion(a) + "\n",
			nil,
			"# H\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustRender(t, tt.doc, tt.mods)
			if got != tt.want {
				t.Errorf("Render:\n got %q\nwant %q", got, tt.want)
			}
			if again := mustRender(t, got, tt.mods); again != got {
				t.Errorf("not idempotent:\n once %q\ntwice %q", got, again)
			}
		})
	}
}

func TestRenderErrors(t *testing.T) {
	a := mod("core", "alpha\n")
	tests := []struct {
		name string
		doc  string
		mods []module.Module
		want string
	}{
		{"unparseable document", begCore + "a\n", []module.Module{a}, "line 1"},
		{"duplicate module names", "", []module.Module{a, a}, "core"},
		{"invalid module name", "", []module.Module{{Name: "Core", Hash: a.Hash, Body: "x\n"}}, "Core"},
		{"invalid module hash", "", []module.Module{{Name: "core", Hash: "XYZ", Body: "x\n"}}, "hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(tt.doc, tt.mods)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestRenderFile(t *testing.T) {
	m := module.Module{
		Name: "delegation",
		Kind: module.KindFile,
		Path: "project/agents/delegation.md",
		Body: "# Delegating\n\nrules\n",
	}
	m.Hash = testHash(m.Body)

	got, err := RenderFile("", m)
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}
	if want := wantRegion(m); got != want {
		t.Fatalf("RenderFile from empty:\n got %q\nwant %q", got, want)
	}

	again, err := RenderFile(got, m)
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}
	if again != got {
		t.Errorf("re-render is not a no-op:\n got %q\nwant %q", again, got)
	}

	head := "<!-- hand-written head -->\n\n"
	withHead, err := RenderFile(head+got, m)
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}
	if withHead != head+got {
		t.Errorf("hand-written head not preserved:\n got %q\nwant %q", withHead, head+got)
	}
}

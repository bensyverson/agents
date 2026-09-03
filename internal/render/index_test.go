package render

import (
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

// wantIndexBody pins the exact bytes of the generated index for the alpha and
// beta fixtures, in that order. It is written out literally rather than built
// from the same pieces as the implementation, so a change to the wording has
// to be a deliberate edit here.
const wantIndexBody = "## Situational instructions\n" +
	"\n" +
	"These files carry instructions for specific situations. When one applies, read the file before acting and follow it.\n" +
	"\n" +
	"| Situation | File |\n" +
	"|---|---|\n" +
	"| When alpha applies | `project/agents/alpha.md` |\n" +
	"| When beta applies | `project/agents/beta.md` |\n"

func TestIndexBody(t *testing.T) {
	got, ok := Index([]module.Module{
		mod("core", "core rules\n"),
		fileMod("alpha", "When alpha applies", "alpha rules\n"),
		fileMod("beta", "When beta applies", "beta rules\n"),
	})

	if !ok {
		t.Fatal("Index reported no index for two when: modules")
	}
	if got.Name != IndexName {
		t.Errorf("Name = %q, want %q", got.Name, IndexName)
	}
	if got.Kind != module.KindInline {
		t.Errorf("Kind = %v, want inline", got.Kind)
	}
	if got.Body != wantIndexBody {
		t.Errorf("Body:\n got %q\nwant %q", got.Body, wantIndexBody)
	}
	if got.Hash != testHash(wantIndexBody) {
		t.Errorf("Hash = %q, want %q", got.Hash, testHash(wantIndexBody))
	}
	if got.Path != "" || len(got.Seeds) != 0 {
		t.Errorf("the index renders inline and seeds nothing: %+v", got)
	}
}

// The table is the manifest's order, not the alphabet's: the manifest is where
// a repo says which situation it wants read first.
func TestIndexRowsFollowManifestOrder(t *testing.T) {
	got, ok := Index([]module.Module{
		fileMod("beta", "When beta applies", "beta rules\n"),
		fileMod("alpha", "When alpha applies", "alpha rules\n"),
	})
	if !ok {
		t.Fatal("Index reported no index for two when: modules")
	}
	first := strings.Index(got.Body, "beta.md")
	second := strings.Index(got.Body, "alpha.md")
	if first < 0 || second < 0 || first > second {
		t.Errorf("rows are not in manifest order:\n%s", got.Body)
	}
}

// A file module without when: is not situational, so it must not appear.
func TestIndexIsAbsentWithoutAWhenModule(t *testing.T) {
	mods := []module.Module{
		mod("core", "core rules\n"),
		fileMod("delegation", "", "delegation rules\n"),
	}
	if got, ok := Index(mods); ok {
		t.Errorf("Index = %+v, want none", got)
	}
	if got, ok := Index(nil); ok {
		t.Errorf("Index(nil) = %+v, want none", got)
	}
}

// A pipe in a when: phrase would otherwise split the row into three cells.
func TestIndexEscapesPipesInASituation(t *testing.T) {
	got, ok := Index([]module.Module{fileMod("alpha", "Before a | b", "alpha rules\n")})
	if !ok {
		t.Fatal("Index reported no index for a when: module")
	}
	if !strings.Contains(got.Body, `| Before a \| b | `) {
		t.Errorf("pipe is not escaped:\n%s", got.Body)
	}
}

// The index is an ordinary module to Render: it fills its slot, is appended
// after the last region when it has none, and is dropped when it stops
// applying — the whole point of synthesising it as a module.
func TestRenderPlacesTheIndexAfterTheLastRegion(t *testing.T) {
	core := mod("core", "core rules\n")
	idx, ok := Index([]module.Module{fileMod("alpha", "When alpha applies", "alpha rules\n")})
	if !ok {
		t.Fatal("Index reported no index for a when: module")
	}

	doc := "# Head\n\n" + wantRegion(core) + "\ntail\n"
	got := mustRender(t, doc, []module.Module{core, idx})
	want := "# Head\n\n" + wantRegion(core) + "\n" + wantRegion(idx) + "\ntail\n"
	if got != want {
		t.Errorf("Render:\n got %q\nwant %q", got, want)
	}
	if again := mustRender(t, got, []module.Module{core, idx}); again != got {
		t.Errorf("not idempotent:\n once %q\ntwice %q", got, again)
	}

	dropped := mustRender(t, got, []module.Module{core})
	if wantDropped := "# Head\n\n" + wantRegion(core) + "\ntail\n"; dropped != wantDropped {
		t.Errorf("dropping the index:\n got %q\nwant %q", dropped, wantDropped)
	}
}

func TestSetRegion(t *testing.T) {
	core := mod("core", "core rules\n")
	other := mod("principles", "principles\n")
	idx, _ := Index([]module.Module{fileMod("alpha", "When alpha applies", "alpha rules\n")})
	staleOther := markedRegion("principles", "000000", "old principles\n")

	tests := []struct {
		name string
		doc  string
		m    module.Module
		want string
	}{
		{
			"replaces the region of that name in place",
			"# H\n\n" + markedRegion(idx.Name, "000000", "old index\n") + "\n" + wantRegion(core) + "\ntail\n",
			idx,
			"# H\n\n" + wantRegion(idx) + "\n" + wantRegion(core) + "\ntail\n",
		},
		{
			"appends after the last region when absent",
			"# H\n\n" + wantRegion(core) + "\ntail\n",
			idx,
			"# H\n\n" + wantRegion(core) + "\n" + wantRegion(idx) + "\ntail\n",
		},
		{
			"leaves every other region exactly as it is",
			"# H\n\n" + wantRegion(core) + "\n" + staleOther,
			idx,
			"# H\n\n" + wantRegion(core) + "\n" + staleOther + "\n" + wantRegion(idx),
		},
		{
			"a document with no regions gets one",
			"# H\n",
			other,
			"# H\n\n" + wantRegion(other),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetRegion(tt.doc, tt.m)
			if err != nil {
				t.Fatalf("SetRegion: %v", err)
			}
			if got != tt.want {
				t.Errorf("SetRegion:\n got %q\nwant %q", got, tt.want)
			}
			again, err := SetRegion(got, tt.m)
			if err != nil {
				t.Fatalf("SetRegion again: %v", err)
			}
			if again != got {
				t.Errorf("not idempotent:\n once %q\ntwice %q", got, again)
			}
		})
	}
}

func TestSetRegionRejectsAnUnparseableDocument(t *testing.T) {
	if _, err := SetRegion(begCore+"a\n", mod("core", "core rules\n")); err == nil {
		t.Fatal("want an error on a document whose region never ends, got nil")
	}
}

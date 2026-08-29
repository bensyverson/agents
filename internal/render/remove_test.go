package render

import (
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

const (
	removeHead = "# Project\n\nThe head is the project's.\n\n"
	between    = "\n## A hand-written section\n\nIt sits between two regions.\n\n"
	tail       = "\n## A tail\n\nBelow the last region.\n"
)

func mustRemove(t *testing.T, doc string, names ...string) (string, []Region) {
	t.Helper()
	got, removed, err := RemoveRegions(doc, names...)
	if err != nil {
		t.Fatalf("RemoveRegions(%v): %v", names, err)
	}
	return got, removed
}

// The whole point: everything the project wrote survives, and only the named
// region leaves.
func TestRemoveRegionsKeepsProjectText(t *testing.T) {
	core, principles := mod("core", "core rules\n"), mod("principles", "principles\n")
	doc := removeHead + wantRegion(core) + between + wantRegion(principles) + tail

	got, removed := mustRemove(t, doc, "principles")

	want := removeHead + wantRegion(core) + between + strings.TrimPrefix(tail, "\n")
	if got != want {
		t.Errorf("document:\n got %q\nwant %q", got, want)
	}
	if len(removed) != 1 || removed[0].Name != "principles" {
		t.Fatalf("removed = %v, want the principles region", removed)
	}
	if removed[0].Body != principles.Body {
		t.Errorf("removed body = %q, want %q", removed[0].Body, principles.Body)
	}
	if removed[0].MarkerHash != principles.Hash {
		t.Errorf("removed marker hash = %q, want %q", removed[0].MarkerHash, principles.Hash)
	}
}

// Removing the first region must leave the head untouched, byte for byte.
func TestRemoveRegionsKeepsHead(t *testing.T) {
	core, principles := mod("core", "core rules\n"), mod("principles", "principles\n")
	doc := removeHead + wantRegion(core) + "\n" + wantRegion(principles)

	got, _ := mustRemove(t, doc, "core")

	if !strings.HasPrefix(got, removeHead) {
		t.Errorf("head changed:\n got %q\nwant prefix %q", got, removeHead)
	}
	if want := removeHead + wantRegion(principles); got != want {
		t.Errorf("document:\n got %q\nwant %q", got, want)
	}
}

// A whole generated file is one region and nothing else; removing it leaves an
// empty document, which is how the caller knows the file can go.
func TestRemoveRegionsEmptiesAWholeFileDocument(t *testing.T) {
	delegation := mod("delegation", "# Delegating\n\nrules\n")

	got, removed := mustRemove(t, wantRegion(delegation), "delegation")

	if got != "" {
		t.Errorf("document = %q, want empty", got)
	}
	if len(removed) != 1 {
		t.Fatalf("removed = %v, want one region", removed)
	}
}

// Removing is a decision, not a sync: a region whose body no longer matches its
// marker still goes, and the caller is handed the body so it can say so.
func TestRemoveRegionsRemovesHandEditedRegion(t *testing.T) {
	core := mod("core", "core rules\n")
	edited := "core rules\nAND MY OWN LINE\n"
	doc := removeHead + markedRegion("core", core.Hash, edited)

	got, removed := mustRemove(t, doc, "core")

	if got != removeHead {
		t.Errorf("document = %q, want just the head %q", got, removeHead)
	}
	if len(removed) != 1 || removed[0].Body != edited {
		t.Fatalf("removed = %v, want the edited body %q", removed, edited)
	}
}

// Several names at once, and a name with no region, are both ordinary.
func TestRemoveRegionsMultipleAndMissing(t *testing.T) {
	core, principles := mod("core", "core rules\n"), mod("principles", "principles\n")
	doc := removeHead + wantRegion(core) + "\n" + wantRegion(principles)

	got, removed := mustRemove(t, doc, "core", "principles", "docs")

	if got != removeHead {
		t.Errorf("document = %q, want just the head", got)
	}
	if len(removed) != 2 {
		t.Fatalf("removed = %v, want two regions", removed)
	}
	if removed[0].Name != "core" || removed[1].Name != "principles" {
		t.Errorf("removed = %v, want document order", removed)
	}
}

func TestRemoveRegionsNoSuchRegionChangesNothing(t *testing.T) {
	core := mod("core", "core rules\n")
	doc := removeHead + wantRegion(core) + tail

	got, removed := mustRemove(t, doc, "docs")

	if got != doc {
		t.Errorf("document changed:\n got %q\nwant %q", got, doc)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want nothing", removed)
	}
}

func TestRemoveRegionsRejectsMalformedDocument(t *testing.T) {
	if _, _, err := RemoveRegions("<!-- agents:begin core@zzz -->\n", "core"); err == nil {
		t.Error("RemoveRegions on a malformed marker returned no error")
	}
}

// What remove leaves behind must be exactly what a later sync would render,
// or `agents check` would report the file as unrendered forever after.
func TestRemoveRegionsLeavesNothingForSyncToDo(t *testing.T) {
	core, principles := mod("core", "core rules\n"), mod("principles", "principles\n")
	doc := removeHead + wantRegion(core) + between + wantRegion(principles) + tail

	got, _ := mustRemove(t, doc, "core")

	rendered := mustRender(t, got, []module.Module{principles})
	if rendered != got {
		t.Errorf("a sync would rewrite the file:\n got %q\nwant %q", rendered, got)
	}
}

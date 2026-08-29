package render

import (
	"reflect"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

func TestInspectClassifies(t *testing.T) {
	core := mod("core", "alpha\n")
	oldHash := testHash("old\n")

	tests := []struct {
		name       string
		doc        string
		mods       []module.Module
		wantRegion RegionReport
		wantFresh  bool
	}{
		{
			name: "fresh",
			doc:  wantRegion(core),
			mods: []module.Module{core},
			wantRegion: RegionReport{
				Name: "core", MarkerHash: core.Hash, BodyHash: core.Hash, ModuleHash: core.Hash,
				Body: "alpha\n", Line: 1,
			},
			wantFresh: true,
		},
		{
			name: "stale: module changed since the render",
			doc:  markedRegion("core", oldHash, "old\n"),
			mods: []module.Module{core},
			wantRegion: RegionReport{
				Name: "core", MarkerHash: oldHash, BodyHash: oldHash, ModuleHash: core.Hash,
				Body: "old\n", Line: 1, Stale: true,
			},
		},
		{
			name: "edited: body no longer matches its marker",
			doc:  markedRegion("core", core.Hash, "hand written\n"),
			mods: []module.Module{core},
			wantRegion: RegionReport{
				Name: "core", MarkerHash: core.Hash, BodyHash: testHash("hand written\n"), ModuleHash: core.Hash,
				Body: "hand written\n", Line: 1, Edited: true,
			},
		},
		{
			name: "stale and edited at once",
			doc:  markedRegion("core", oldHash, "hand written\n"),
			mods: []module.Module{core},
			wantRegion: RegionReport{
				Name: "core", MarkerHash: oldHash, BodyHash: testHash("hand written\n"), ModuleHash: core.Hash,
				Body: "hand written\n", Line: 1, Stale: true, Edited: true,
			},
		},
		{
			name: "orphan: region with no module",
			doc:  markedRegion("gone", oldHash, "old\n"),
			mods: []module.Module{},
			wantRegion: RegionReport{
				Name: "gone", MarkerHash: oldHash, BodyHash: oldHash, ModuleHash: "",
				Body: "old\n", Line: 1, Orphan: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := mustInspect(t, tt.doc, tt.mods)
			if len(rep.Regions) != 1 {
				t.Fatalf("want 1 region, got %d", len(rep.Regions))
			}
			if !reflect.DeepEqual(rep.Regions[0], tt.wantRegion) {
				t.Errorf("region:\n got %+v\nwant %+v", rep.Regions[0], tt.wantRegion)
			}
			if got := rep.Regions[0].Fresh(); got != tt.wantFresh {
				t.Errorf("Fresh() = %v, want %v", got, tt.wantFresh)
			}
		})
	}
}

func TestInspectMissingAndOrder(t *testing.T) {
	core := mod("core", "alpha\n")
	principles := mod("principles", "beta\n")
	goMod := mod("go", "gamma\n")

	doc := "# H\n\n" + wantRegion(goMod) + "\n" + markedRegion("gone", testHash("x\n"), "x\n") + "\n"
	rep := mustInspect(t, doc, []module.Module{core, goMod, principles})

	var names []string
	for _, r := range rep.Regions {
		names = append(names, r.Name)
	}
	if want := []string{"go", "gone"}; !reflect.DeepEqual(names, want) {
		t.Errorf("regions in document order: got %v want %v", names, want)
	}
	if want := []string{"core", "principles"}; !reflect.DeepEqual(rep.Missing, want) {
		t.Errorf("Missing: got %v want %v", rep.Missing, want)
	}
}

func TestInspectAggregates(t *testing.T) {
	core := mod("core", "alpha\n")
	principles := mod("principles", "beta\n")
	oldHash := testHash("old\n")

	clean := mustInspect(t, wantRegion(core)+"\n"+wantRegion(principles), []module.Module{core, principles})
	if clean.AnyStale() || clean.AnyEdited() || clean.AnyOrphan() {
		t.Errorf("clean report: stale=%v edited=%v orphan=%v", clean.AnyStale(), clean.AnyEdited(), clean.AnyOrphan())
	}

	stale := mustInspect(t, markedRegion("core", oldHash, "old\n"), []module.Module{core})
	if !stale.AnyStale() || stale.AnyEdited() || stale.AnyOrphan() {
		t.Errorf("stale report: stale=%v edited=%v orphan=%v", stale.AnyStale(), stale.AnyEdited(), stale.AnyOrphan())
	}

	edited := mustInspect(t, markedRegion("core", core.Hash, "hand\n"), []module.Module{core})
	if edited.AnyStale() || !edited.AnyEdited() || edited.AnyOrphan() {
		t.Errorf("edited report: stale=%v edited=%v orphan=%v", edited.AnyStale(), edited.AnyEdited(), edited.AnyOrphan())
	}

	orphan := mustInspect(t, wantRegion(core), nil)
	if orphan.AnyStale() || orphan.AnyEdited() || !orphan.AnyOrphan() {
		t.Errorf("orphan report: stale=%v edited=%v orphan=%v", orphan.AnyStale(), orphan.AnyEdited(), orphan.AnyOrphan())
	}
}

func TestInspectParseError(t *testing.T) {
	if _, err := Inspect(begCore+"a\n", nil); err == nil {
		t.Fatal("want a parse error, got nil")
	}
}

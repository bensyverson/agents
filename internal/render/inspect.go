package render

import (
	"fmt"
	"slices"

	"github.com/bensyverson/agents/internal/module"
)

// RegionReport is what Inspect knows about one region in a document.
type RegionReport struct {
	Name string
	// MarkerHash is the module hash recorded when the region was rendered.
	MarkerHash string
	// BodyHash is the hash of the body as it stands in the document now.
	BodyHash string
	// ModuleHash is the hash of the current module; empty when Orphan.
	ModuleHash string
	// Body is the current body, for diffing against the module.
	Body string
	// Line is the 1-based line of the begin marker.
	Line int
	// Stale means the module changed since this region was rendered.
	Stale bool
	// Edited means the body was changed by hand after the render. A region can
	// be both stale and edited, and both matter: sync refuses on Edited and
	// rewrites on Stale.
	Edited bool
	// Orphan means no module of this name is in the manifest any more.
	Orphan bool
}

// Fresh reports whether the region is exactly what the current module renders to.
func (r RegionReport) Fresh() bool { return !r.Stale && !r.Edited && !r.Orphan }

// Report is the state of one document against a manifest.
type Report struct {
	// Regions are the document's regions, in document order.
	Regions []RegionReport
	// Missing are the modules with no region in the document, in manifest order.
	Missing []string
}

// AnyStale reports whether any region was rendered from an older module.
func (r Report) AnyStale() bool { return r.any(func(x RegionReport) bool { return x.Stale }) }

// AnyEdited reports whether any region was hand-edited since its render.
func (r Report) AnyEdited() bool { return r.any(func(x RegionReport) bool { return x.Edited }) }

// AnyOrphan reports whether any region has no module in the manifest.
func (r Report) AnyOrphan() bool { return r.any(func(x RegionReport) bool { return x.Orphan }) }

func (r Report) any(pred func(RegionReport) bool) bool {
	return slices.ContainsFunc(r.Regions, pred)
}

// Inspect classifies every region in doc against the modules in mods.
func Inspect(doc string, mods []module.Module) (Report, error) {
	d, err := Parse(doc)
	if err != nil {
		return Report{}, fmt.Errorf("parse document: %w", err)
	}
	byName := make(map[string]module.Module, len(mods))
	for _, m := range mods {
		byName[m.Name] = m
	}

	var rep Report
	present := make(map[string]bool, len(d.Segments))
	for _, r := range d.Regions() {
		present[r.Name] = true
		rr := RegionReport{
			Name:       r.Name,
			MarkerHash: r.MarkerHash,
			BodyHash:   module.Hash(r.Body),
			Body:       r.Body,
			Line:       r.BeginLine,
		}
		if m, ok := byName[r.Name]; ok {
			rr.ModuleHash = m.Hash
			rr.Stale = r.MarkerHash != m.Hash
		} else {
			rr.Orphan = true
		}
		rr.Edited = rr.BodyHash != rr.MarkerHash
		rep.Regions = append(rep.Regions, rr)
	}
	for _, m := range mods {
		if !present[m.Name] {
			rep.Missing = append(rep.Missing, m.Name)
		}
	}
	return rep, nil
}

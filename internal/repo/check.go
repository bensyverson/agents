package repo

import "slices"

// ProblemKind is why one region — or a whole file — fails a check. The value
// is the word printed after the region's name, so the vocabulary a hook's user
// reads lives here and nowhere else.
type ProblemKind string

const (
	// ProblemStale means the module moved since the region was rendered.
	ProblemStale ProblemKind = "stale"
	// ProblemEdited means the region was changed by hand after its render.
	ProblemEdited ProblemKind = "hand-edited"
	// ProblemMissing means a module of the manifest has no region in the file.
	ProblemMissing ProblemKind = "missing"
	// ProblemOrphaned means a region's module has left the manifest, so sync
	// would remove it.
	ProblemOrphaned ProblemKind = "orphaned"
	// ProblemUnrendered is the catch-all: the file is not what the modules
	// render, for a reason no single region explains — regions in an order the
	// manifest no longer gives, or separators the renderer would rewrite.
	ProblemUnrendered ProblemKind = "not what the modules render"
)

// Problem is one line of `agents check`.
type Problem struct {
	// Target is the repo-relative file, with forward slashes.
	Target string
	// Region is empty when the problem is the whole file's.
	Region string
	Kind   ProblemKind
}

// String is the line the CLI prints, without any repo prefix.
func (p Problem) String() string {
	if p.Region == "" {
		return p.Target + ": " + string(p.Kind)
	}
	return p.Target + ": " + p.Region + " " + string(p.Kind)
}

// CheckReport is one repo's verdict: the problems that must be resolved before
// a commit leaves nothing to sync.
type CheckReport struct {
	Dir string
	// Problems are in target order, and within a target in document order.
	Problems []Problem
}

// OK reports whether the repo is clean — the silent, exit-0 case.
func (c CheckReport) OK() bool { return len(c.Problems) == 0 }

// HasEdits reports whether any problem is a hand edit — the kind `agents sync`
// refuses to fix, and `agents diff` exists to review.
func (c CheckReport) HasEdits() bool {
	return slices.ContainsFunc(c.Problems, func(p Problem) bool { return p.Kind == ProblemEdited })
}

// NeedsSync reports whether any problem is one a plain `agents sync` resolves.
func (c CheckReport) NeedsSync() bool {
	return slices.ContainsFunc(c.Problems, func(p Problem) bool { return p.Kind != ProblemEdited })
}

// Check classifies everything that stands between the repo as it is and the
// repo a sync would leave behind. It writes nothing.
//
// The guarantee a pre-commit hook needs is that a clean check means a no-op
// sync, so a target sync would rewrite for a reason no region flag explains
// still yields one ProblemUnrendered line.
func (r *Repo) Check() (CheckReport, error) {
	rep, err := r.Inspect()
	if err != nil {
		return CheckReport{}, err
	}
	out := CheckReport{Dir: r.Dir}
	for _, t := range rep.Targets {
		before := len(out.Problems)
		for _, region := range t.Report.Regions {
			// Stale and edited are separate problems on purpose: sync rewrites
			// for one and refuses for the other, so neither may hide the other.
			if region.Stale {
				out.add(t.Path, region.Name, ProblemStale)
			}
			if region.Edited {
				out.add(t.Path, region.Name, ProblemEdited)
			}
			if region.Orphan {
				out.add(t.Path, region.Name, ProblemOrphaned)
			}
		}
		for _, name := range t.Report.Missing {
			out.add(t.Path, name, ProblemMissing)
		}
		if len(out.Problems) == before && t.NeedsWrite() {
			out.add(t.Path, "", ProblemUnrendered)
		}
	}
	return out, nil
}

func (c *CheckReport) add(target, region string, kind ProblemKind) {
	c.Problems = append(c.Problems, Problem{Target: target, Region: region, Kind: kind})
}

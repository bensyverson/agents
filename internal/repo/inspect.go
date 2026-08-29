package repo

import (
	"slices"
	"strings"
	"time"

	"github.com/bensyverson/agents/internal/gotchas"
	"github.com/bensyverson/agents/internal/render"
)

// Report is every target of a repo, inspected. It is what sync, diff and
// status all read.
type Report struct {
	Dir     string
	Targets []Target
}

// Inspect reads and classifies every target.
func (r *Repo) Inspect() (Report, error) {
	targets, err := r.Targets()
	if err != nil {
		return Report{}, err
	}
	return Report{Dir: r.Dir, Targets: targets}, nil
}

// NeedsWrite reports whether any file would change on the next sync.
func (r Report) NeedsWrite() bool {
	return slices.ContainsFunc(r.Targets, Target.NeedsWrite)
}

// AnyStale reports whether any region was rendered from an older module.
func (r Report) AnyStale() bool { return r.any(render.Report.AnyStale) }

// AnyEdited reports whether any region was changed by hand since its render.
func (r Report) AnyEdited() bool { return r.any(render.Report.AnyEdited) }

// AnyOrphan reports whether any region's module has left the manifest.
func (r Report) AnyOrphan() bool { return r.any(render.Report.AnyOrphan) }

func (r Report) any(pred func(render.Report) bool) bool {
	return slices.ContainsFunc(r.Targets, func(t Target) bool { return pred(t.Report) })
}

// Status is one repo's line in `agents status`.
//
// Stale and Edited are counted distinctly, and a region that is both — the
// module moved and an agent also rewrote the region — is counted in both:
// sync rewrites for one and refuses for the other, so neither may hide the
// other.
type Status struct {
	Dir string
	// Modules are the manifest's module names, in order.
	Modules      []string
	StaleCount   int
	EditedCount  int
	MissingCount int
	OrphanCount  int
	GotchaCount  int
	// GotchaLines is the whole gotchas file's size, preamble included.
	GotchaLines int
	// GotchasOverBudget is true once the file has outgrown gotchas.Budget*.
	// It is advisory: the CLI appends a warning and nothing refuses.
	GotchasOverBudget bool
	// HeadLines is the size of AGENTS.md's project-owned head — the lines
	// above the first generated region. The head is where a repo's own rules
	// live, so this is the balloon gauge: an empty head means the repo has
	// said nothing of its own, and a head that keeps growing is a candidate
	// for a shared module.
	HeadLines int
	// OldestGotcha is valid only when HasOldestGotcha; a repo may have a
	// gotchas file with no entries yet.
	OldestGotcha    time.Time
	HasOldestGotcha bool
	HasGotchasFile  bool
}

// Status summarises the repo for one line of output.
func (r *Repo) Status() (Status, error) {
	rep, err := r.Inspect()
	if err != nil {
		return Status{}, err
	}
	st := Status{Dir: r.Dir, Modules: r.Manifest.Modules}
	for _, t := range rep.Targets {
		if t.Path == AgentsFile {
			if st.HeadLines, err = headLines(t.Current); err != nil {
				return Status{}, err
			}
		}
		st.MissingCount += len(t.Report.Missing)
		for _, region := range t.Report.Regions {
			if region.Stale {
				st.StaleCount++
			}
			if region.Edited {
				st.EditedCount++
			}
			if region.Orphan {
				st.OrphanCount++
			}
		}
	}

	file, exists, err := r.Gotchas()
	if err != nil {
		return Status{}, err
	}
	st.HasGotchasFile = exists
	st.GotchaCount = len(file.Entries)
	st.GotchaLines = file.Lines
	st.GotchasOverBudget = file.OverBudget()
	st.OldestGotcha, st.HasOldestGotcha = file.Entries.Oldest()
	return st, nil
}

// headLines counts the project-owned lines above the first region of doc.
// A document with no regions is all head — the largest possible one, which
// is exactly the not-yet-migrated file the gauge exists to show — so it
// counts in full. Blank lines at the end of the head do not count — the one
// before a region is the renderer's separator, not the project's prose.
func headLines(doc string) (int, error) {
	d, err := render.Parse(doc)
	if err != nil {
		return 0, err
	}
	if len(d.Segments) == 0 || d.Segments[0].Kind != render.SegmentText {
		return 0, nil
	}
	head := strings.TrimRight(d.Segments[0].Text, "\n")
	if head == "" {
		return 0, nil
	}
	return strings.Count(head, "\n") + 1, nil
}

// DiffReport is the review queue for one repo: what agents changed by hand
// inside generated regions, and what they said about the rules themselves.
type DiffReport struct {
	Dir string
	// Edits are the hand-edited regions, in target order.
	Edits []EditedRegion
	// Rules are the `rule:` entries of project/gotchas.md.
	Rules gotchas.Entries
}

// Empty reports whether there is nothing to review.
func (d DiffReport) Empty() bool { return len(d.Edits) == 0 && len(d.Rules) == 0 }

// Diff collects every hand-edited region and every rule: gotcha.
//
// The module is the "a" side of each diff and the file is "b", so a "+" line is
// what the agent added — the opposite of Sync, which shows what a --force would
// undo.
func (r *Repo) Diff() (DiffReport, error) {
	rep, err := r.Inspect()
	if err != nil {
		return DiffReport{}, err
	}
	out := DiffReport{Dir: r.Dir}
	for _, t := range rep.Targets {
		for _, region := range t.Report.Regions {
			if !region.Edited {
				continue
			}
			out.Edits = append(out.Edits, EditedRegion{
				Target: t.Path,
				Region: region.Name,
				Diff:   render.Diff(t.moduleBody(region.Name), region.Body),
			})
		}
	}

	file, _, err := r.Gotchas()
	if err != nil {
		return DiffReport{}, err
	}
	out.Rules = file.Entries.Rules()
	return out, nil
}

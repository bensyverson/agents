package repo

import "fmt"

// ApplyMode says whether a verb that applies the modules' declarations writes
// what it worked out, or only reports it.
type ApplyMode int

const (
	// ApplyWrite writes the change.
	ApplyWrite ApplyMode = iota
	// ApplyDryRun reports what would change and writes nothing.
	ApplyDryRun
)

// applyDeclared brings the .gitignore and the pre-commit hook in line with
// what the modules declare, the step init and update share. It never takes
// over a hook the project wrote or edited: that needs the person's consent,
// which only `agents hooks install --force` carries.
func (r *Repo) applyDeclared() (GitignoreResult, HookResult, error) {
	ignored, err := r.Gitignore(ApplyWrite)
	if err != nil {
		return ignored, HookResult{}, fmt.Errorf("%s: %w", r.Dir, err)
	}
	hook, err := r.InstallHook(ApplyWrite, HookLeaveOthers)
	if err != nil {
		return ignored, hook, fmt.Errorf("%s: %w", r.Dir, err)
	}
	return ignored, hook, nil
}

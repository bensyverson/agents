## Shared rules

**Where these rules come from.** Everything between an `agents:begin` and its `agents:end` marker is generated: it is a shared module, rendered here by a CLI tool named `agents` from the sources listed in `.agents.yaml`. Don't edit inside a region — the next sync either overwrites your edit or refuses to run at all until someone resolves it. Everything outside the markers is this project's, including the head above the first region.

**Feedback on a shared rule goes in this repo's own notes**, outside the markers, not inside them. A rule that was wrong, misread, or cost you time is worth writing down where a re-render won't erase it, and that record is how a shared rule gets changed rather than quietly worked around in one repo.

**`agents check` belongs in pre-commit.** It writes nothing and exits 0 when every region is current and unedited, so `agents check || exit 1` in the hook is what stops a stale or hand-edited region from reaching a commit. When it fails, `agents sync` is usually the whole fix.

**Enabling and disabling rule sets** is `agents add <module>` and `agents remove <module>`, which edit `.agents.yaml` and re-render; `agents list` shows what the sources offer and what this repo has turned on.

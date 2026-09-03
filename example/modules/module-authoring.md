---
kind: file
when: Writing or editing an agents module
---
# Writing a module

A module is one markdown file, `modules/<name>.md`, inside a **source** — a git repository, or a directory, with `modules/` and `templates/` at its root. The file name without `.md` is the module's name: it is what a repo lists in `.agents.yaml`, what the region markers carry, and what its rendered path is derived from. The file may open with a YAML frontmatter block; everything after that block is the body, and the body is rendered verbatim.

## Frontmatter

Frontmatter is recognised only when `---` is the very first line of the file — a `---` further down is content, a rule or a sample, not a header. It is optional: a module with none is an inline module that seeds nothing.

- **`kind: inline`** (the default) renders the body into its own marked region inside `AGENTS.md`, in the order the manifest lists it.
- **`kind: file`** renders the body to `project/agents/<name>.md` instead, as a file holding one whole-file region. **That path is derived from the name and is never declared** — a `path:` key is a load error — so two file modules cannot collide on where they write, and every module can predict where every other one lands.
- **`when:`** is one phrase naming the situation in which that file has to be read: *"Writing or editing an agents module"*, *"Before dispatching any subagent"*. Every enabled module carrying one becomes a row in a generated index table in `AGENTS.md`, pairing the situation with the path, so a reader meets the trigger without paying for the file's contents up front. Valid only with `kind: file`; on an inline module it is a load error. A file module without `when:` renders as usual and simply doesn't appear in the index.
- **`seeds:`** lists repo-relative files that every repo enabling the module should have. Each is copied from `templates/<path>` in the same source when it is missing, and never overwritten again — a seeded file belongs to the repo the moment it exists. That makes seeds right for starting a file the repo then owns, and wrong for anything that must stay in step with the module. A declared seed with no template, or a path that is absolute or climbs out of the repo, is an error.

## References between modules

A module points at a sibling by its rendered path, `project/agents/<name>.md`, and the repo has to have that sibling enabled or the reference points at nothing. Keep such references few and one-directional: each is a coupling a repo can break by enabling one module and not the other.

## Register

- **Every rule carries its why**, in the same breath. A rule with its reason stripped out gets followed literally where it should have been weighed, and can't be argued with on the day it is wrong.
- **Fewer rules, each load-bearing.** The body is read on every task in every repo that enables it. A rule that has never changed an outcome is spending attention it didn't earn — prefer deleting it to hedging it.
- **A file module opens with the two or three lines a reader needs even if they read nothing else**, and puts the detail below. It is opened on a trigger, usually mid-task, and the opening is what survives the skim.
- **Write for a repo you have never seen.** Anything that names a person, a client, a repository or a particular language belongs in that repo's own notes, outside its markers — not in a module every repo renders.

# agents

Renders a shared set of house rules into each repo's `AGENTS.md`, and keeps them in sync.

`AGENTS.md` (with `CLAUDE.md` symlinked to it) is read by every coding harness. Most of its content is the same across repos — working rules, stack rules, workflow modules — and used to be hand-copied, so edits drifted. This tool owns the shared parts as markdown **modules**, embeds them in the binary, and renders them into marked regions of each repo's `AGENTS.md`. Everything outside the markers belongs to the project.

## Layout of a managed repo

- `AGENTS.md` — project-owned head and tail around generated regions, the last of them the generated situational index
- `CLAUDE.md` — a symlink to `AGENTS.md`, created by `init` if absent
- `.agents.yaml` — the manifest: which modules, in what order
- `project/gotchas.md` — project quirks and rule feedback, appended by agents, pruned by humans
- `project/agents/*.md` — the modules too long to inline (`cli-design.md`, `delegation.md`, `design-process.md`, `evidence.md`, `harness.md`, `jobs.md`), one per `kind: file` module the repo enables

## The marker convention

Each rendered module sits between a begin/end marker pair:

```
<!-- agents:begin core@a1b2c3 -->
…rendered module…
<!-- agents:end core -->
```

`a1b2c3` is the module's content hash: the first 6 hex characters of the SHA-256 digest of the module's body, with any leading YAML frontmatter stripped first. A region whose body no longer hashes to its marker has been hand-edited since it was rendered — that mismatch is what `sync` refuses to clobber and `diff` surfaces for review. Anything outside a marker pair — including a module no longer in the manifest, once its region is removed — is project-owned and preserved byte-for-byte.

That holds in *every* rendered file, not just `AGENTS.md`: text above or below the markers of a `project/agents/*.md` file is the project's too, so project-specific additions to a file module go below its `agents:end` marker, where sync leaves them byte-for-byte. In `AGENTS.md` the same space above the first region is the project-owned head, and `init` seeds it from `templates/head.md` when the repo has no `AGENTS.md` at all — a repo that already has one keeps whatever it says as its head.

## Module format

A module is a markdown file in `modules/`, optionally preceded by YAML frontmatter (a `---` block as the very first line):

```
---
kind: file
when: Before dispatching any subagent
---
# Delegating to subagents
…
```

- `kind: inline` (the default) renders the module into its own marked region inside `AGENTS.md`, in manifest order.
- `kind: file` renders the module to `project/agents/<name>.md` instead, as its own file holding a single whole-file region (the project may still write its own head above it, exactly as with `AGENTS.md`). The path is **derived from the name**, never declared: two file modules can therefore never collide on where they write, and a module referencing a sibling writes `project/agents/<name>.md` and knows it is right. A `path:` key is a load error naming the offending module — the key existed in v1 and is gone.
- `when: <one situation phrase>` says when the file must be read — "Before dispatching any subagent". It is valid only with `kind: file` (`when:` on an inline module is a load error) and optional even there: a file module without it renders exactly as before and simply never appears in the index. The phrase is one row of the index table below, so write it as the condition an agent can match against, not as a description of the file.
- `seeds: [project/gotchas.md, …]` lists repo-relative files the module wants every repo that enables it to have. Each is created from `templates/<seed path>` when it is missing, and never overwritten — a seeded file belongs to the repo the moment it exists. Valid on any kind; a path that is absolute or escapes the repo is an error, as is a declared seed with no template. `templates/head.md` is not a seed: the head is `init`'s own special case.

`index` is a **reserved module name**: the tool generates a region of that name itself (below), so a module file called `index.md` is a load error and `index` in `.agents.yaml` is an unknown module.

## The situational index

`AGENTS.md` carries one generated region, named `index`, that lists every enabled module with a `when:` phrase and the file to read for it:

```
<!-- agents:begin index@a1b2c3 -->
## Situational instructions

These files carry instructions for specific situations. When one applies, read the file before acting and follow it.

| Situation | File |
|---|---|
| Before dispatching any subagent | `project/agents/delegation.md` |
<!-- agents:end index -->
```

It is the one region whose body comes from the manifest rather than from a module file, and it is an ordinary region in every other respect: it renders after the last inline region, its marker carries the hash of its own bytes, and `sync`, `check`, `diff`, `status` and `remove` treat it exactly as they treat `core`. Rows are in manifest order. The region exists only while at least one enabled module carries `when:`; enabling the first such module creates it and removing the last one deletes it, which is why `remove` rewrites it rather than leaving it stale.

This replaces the v1 pattern of pairing each file module with a short inline `-brief` module whose only job was to say "read that file". The situation now lives on the module it belongs to, in one line, and the table is rendered.

## Verbs

| Verb | Flags | Does |
|---|---|---|
| `agents init` | `--with a,b,c` (default `core,principles,stage-build`) | Write `.agents.yaml`, render `AGENTS.md` (an existing file becomes the project-owned head; with no file at all the head is seeded from `templates/head.md`), create any file the chosen modules seed (`core` seeds `project/gotchas.md`, `docs` seeds `project/backlog.md`, `background` seeds `project/background.md`), link `CLAUDE.md`, and register the repo. Every step is skipped, not repeated, if already done. Refuses the same way `sync` does if a region has been hand-edited. |
| `agents add <module>…` | — | Append each module to `.agents.yaml` in the order given, then render: an inline module's region lands after every region already in `AGENTS.md` and above whatever tail the project wrote, a `kind: file` module gets its file, and any file the new modules seed is created. A module the manifest already lists prints `<name> is already enabled` and is skipped. Nothing above the first region is touched. A hand-edited region stops the whole thing — nothing is written, the manifest included — exactly as `sync` does. |
| `agents remove <module>…` | — | Drop each module from `.agents.yaml` and delete its region. A `kind: file` module's file goes too when the region was all it held; a file the project has written in keeps every one of its own bytes, loses only the region, and is named in a `kept …` line. Removing is a decision, not a sync: a region edited by hand since its render is removed anyway (and said so), and no other module's region is touched — except the generated `index`, which is rewritten (or deleted, with the last `when:` module) so it never names a file the repo no longer has. |
| `agents list` | — | One line per module the binary knows, sorted, with `*` on the ones `.agents.yaml` lists and the derived `project/agents/<name>.md` path of every `kind: file` module. The one verb that runs outside a managed repo: there it lists what could be enabled and marks nothing. |
| `agents sync` | `--force`, `--all`, `--reseed-gotchas` | Re-render every stale region and generated file. Silent and exits 0 when nothing is stale. A hand-edited region stops that repo's sync — nothing is written — unless `--force`. Also creates any declared seed the repo is missing, printing `seeded <path>` — that is how an existing repo picks up a seed a module gained since `init`. With `--all`, every registered repo is visited; refusals are reported per repo and the exit is non-zero if any repo refused. Prints a budget warning after a repo's rendered lines when its `project/gotchas.md` is over budget. `--reseed-gotchas` replaces that file's preamble with the template's, keeping every entry. |
| `agents diff` | `--all` | Print every hand-edited region as a diff (a `+` line is what the agent added) plus every `rule:` entry in `project/gotchas.md`. Silent when there's nothing to review. |
| `agents status` | `--all` | One line per repo: modules, `head=N` (the project-owned lines above the first region of `AGENTS.md`), and counts of stale, hand-edited, missing and orphaned regions, plus gotcha count and the oldest entry's age. A repo over the gotchas budget gets the warning appended to its line; a repo whose pre-commit hook doesn't run `agents check` gets a one-line hint on stderr naming the hook file and the line to paste. |
| `agents check` | `--all` | Silent and exits 0 when every region is current and unedited and no generated file is missing. Otherwise one line per problem (`AGENTS.md: core stale`, `… hand-edited`, `… missing`, `… orphaned`) and exit 1. It writes nothing, which makes it a pre-commit line: `agents check \|\| exit 1`. With `--all`, every registered repo is checked and each line names its repo. |

A persistent `--modules <dir>` flag (on every verb) reads modules from a directory instead of the ones embedded in the binary, for iterating on a module without reinstalling. Without `--all`, a verb runs on the current directory, which must hold `.agents.yaml` (there's no parent-directory search) — `list` is the exception, and runs anywhere. With `--all`, the repos come from the registry — one absolute path per line at `~/.config/agents/repos`, or wherever `$AGENTS_REGISTRY` points.

## Adding or editing a module

Edit `modules/<name>.md` (add YAML frontmatter only if it needs `kind: file`), then `make sync` — it runs `go install ./cmd/agents` (the installed binary's embedded modules *are* the published standard), re-renders every registered repo with `agents sync --all`, and prints `agents status --all`. Each repo then needs its own commit. `make status` shows what's stale without changing anything; `make diff` prints the review queue. That is the shared text; which modules a given repo *enables* is `agents add <name>` / `agents remove <name>` in that repo, and `agents list` shows both. After cloning, `make hooks` points git at `scripts/git-hooks`. The pre-commit hook runs the Go checks and `agents check`, so a commit that would leave this repo's own `AGENTS.md` stale is refused; the post-commit hook runs `make sync`, so a module commit re-renders every registered repo before their own hooks can complain. A managed repo's hook is never written by the tool — `agents status` only names the file and the line to paste.

## Gotchas

`project/gotchas.md` is a checked-in, agent-appendable list of project-specific traps, seeded from `templates/project/gotchas.md` because `core` declares it (see `seeds:` above). Its format, quoting that template:

> Format: one dated H2 headline, then one paragraph. If an entry needs more than that, it's a finding — write it as a dated doc under `project/` and link it from the paragraph.

Feedback about the shared rules themselves — a rule that was wrong, misread, or cost time — goes in the same file, prefixed `rule:` on the headline (or the entry's first bold span). That prefix is what makes an entry show up in the review loop below, rather than staying a purely local trap.

The file has a budget: **15 entries or 150 lines**. Past either bound, `agents status` appends a warning to the repo's line and `agents sync` prints one after that repo's rendered lines — *"gotchas.md: N entries, M lines (budget 15/150) — retire entries that are fixed or obvious; promote general ones to a module (`agents diff` lists the `rule:` candidates)"*. It is a nag and nothing more: no verb ever refuses over it. Appending has a trigger — something just cost you time — and deleting has none, so the count is put in front of whoever syncs.

The preamble above the file's first `---` belongs to the template, and the entries below it to the project. `agents sync --reseed-gotchas` replaces the preamble with the current template's when the two differ, touching nothing below the rule; a file written before the template had a boundary has no `---` at all, so it gets the preamble and a `---` above everything it holds — including its old header, which a human then deletes once by hand. It prints `reseeded project/gotchas.md` when it writes and nothing when the preamble is already current.

## The review loop

`agents diff --all` is the queue for changing the shared rules: it walks every registered repo and prints every hand-edit an agent or human made inside a generated region, plus every `rule:` gotcha, so you can decide what belongs in `modules/` instead of one repo's `AGENTS.md`. `agents sync` refuses to overwrite a hand-edited region without `--force`, so nothing in that queue is lost to a routine sync.

## Docs

See [project/2026-08-16-plan.md](project/2026-08-16-plan.md) for the design and plan, and [project/2026-08-16-migration.md](project/2026-08-16-migration.md) for the per-repo migration analysis.

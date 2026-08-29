# How `agents` is working in practice

Assessment of the shared rules after ~two weeks across four managed repos: Alpha, Nobedan, Organizize and SleepyHollow. Evidence gathered 2026-08-28 with `agents status` in each repo, `git log` on each `project/gotchas.md`, a harvest of every `rule:` entry and every proposal doc, and a subagent trial of Nobedan's knowledge base. Figures reproduce with `agents status` (run in each repo) and `git log -p --format= -- project/gotchas.md | grep -c '^-## '` (deletions).

## 1. `gotchas.md` grows and is never pruned

| Repo | Lines | Entries | Commits touching | Entries ever deleted | Oldest |
|---|---|---|---|---|---|
| Alpha | 622 | 47 | 33 | **0** | 2026-07-27 |
| Organizize | 258 | 24 | 37 | 2 | 2026-08-26 (all 24 in one day) |
| Nobedan | 103 | 23 | 12 | 1 | 2026-08-16 |
| SleepyHollow | 47 | 17 (one H2) | 12 | 0 | 2026-08-20 |

The template already says *"Delete anything that becomes obvious, gets fixed, or stops recurring. Keep this list short"* — so the missing piece is not the instruction but a **trigger**. Appending has one (something just cost you time); deleting has none. Three further causes:

- **Alpha's file predates the template** and never got the preamble; its header is the old two-line version. No prune instruction has ever been in front of an agent there.
- **Most entries are not project gotchas.** Roughly a third across the four repos are the same harness facts rediscovered per repo: the Bash sandbox blocks `go`/`swift`/`git`/`psql`/`sleepy` (10 entries in 5 repos, including ours), `$TMPDIR` differs between sandboxed and unsandboxed calls (2), `job note` has no `-F` (3), `sleepy` flag shapes (4), `pkill -f` kills the shared server (2), worktrees start behind main (2). Those belong in modules; once they are, the files halve.
- **Entry length is unbounded.** Alpha's entries average 13 lines and the newest is a 40-line essay with a code block; SleepyHollow's format (one bold headline sentence, one paragraph) carries the same information in a quarter of the space.

### Recommendation

1. **Make reading the prune trigger.** Core: *"Read `project/gotchas.md` at session start; while reading, delete any entry that is now fixed, obvious, or a general rule — and if one is general, say so as `rule:`."* Reading is the one moment every entry is in front of an agent.
2. **Cap the shape in the template**: one dated headline plus one paragraph; if it needs more, it is a finding — write the dated project doc and link it.
3. **Let the tool nag.** `agents status` already prints `gotchas=46 (oldest 2026-07-27)`; have `agents sync` warn above a budget (say 15 entries or 150 lines) so the count is seen every time the rules are synced. Cheap, and it is the only mechanism that fires without an agent choosing to.
4. **Re-seed old files' preambles** — `sync` can detect a `gotchas.md` whose preamble does not match the template and offer to replace it above the `---`.

## 2. Alpha's `project/agents/` — what to standardize

Alpha's memory audit (`project/2026-08-21-memory-audit.md`) created four hand-written files beside the two module-rendered ones. Disposition:

**`jobs.md` → promote to a `job` module.** It is almost entirely generic: every leaf gets a parent; leaves are units of *work* grouped by files touched, not one per complaint; a fan-out mints a container leaf plus work leaves *before* dispatch; agents claim/note/release with `--as`, never `done` or commit; `--db` is absolute; work decided against goes to backlog. It would also retire seven scattered gotchas across four repos (`--as` on every write, `note` reads stdin but `done` doesn't, agents can't mark criteria, `orient` errors on an empty focused root). It overlaps `delegation.md` step 4 by design — jobs owns the tracker, delegation owns running agents; each should point at the other. Render it as a file module (`project/agents/jobs.md`) like delegation, since it is read when filing work, not every session.

**`engagement.md` → a seeded `project/background.md`.** It answers *who is this for, who are the people, what constraints come with them, what do they want, what do they call things* — context for weighing a decision, not writing code, as its own preamble says. Every project has a version of that, even if short; only client work has a long one. Seed it from a template like `gotchas.md`, list it in the head, and mention it once in core (*"weigh decisions against `project/background.md` where it exists"*). "Background" over "about/meta" because it names what the file is for; "engagement" only fits consulting. One rule to carry over from the KB trial below: it holds **current** state and is rewritten, not appended — every number, date or name in it appears there once and links to the dated record.

**`preferences.md` → dissolve; it is three things.** (a) *Overrides of shared rules* — the first (commit on `main`) overrides a rule that is in Claude Code's system prompt, not ours; the second (changing an evolved test) is already what core's TDD line permits. (b) *General principles* that belong in modules: one shared struct across a serialization seam (principles, "strongly typed"); a typed constant over a bool that acquires meanings (principles); compare emails case-insensitively (go/web); present options unbiased, weigh operational ownership over build time, think in postures (design-process); and a whole section on **writing prompts for models** — positive direction, capture the why, fewer rules, every added rule competes for attention — which applies to three of the four repos (Alpha, Stack Trace, Organizize's personalization) and has no module today. (c) *Genuinely local rulings* — the maintainer owns `screening.yaml`, screenshots come from `sleepy`, how to estimate a sprint.

So: where does granular feedback fit? **A local ruling goes in the head of `AGENTS.md`**, in the repo's own section — it is read every session, which also forces brevity; the long *why* is a dated project doc. A ruling that overrides a shared rule goes there too and says so; bless that convention in core in one sentence. Anything that generalizes is promoted at review, which is what this document is doing. A 119-line file read "before your first commit" is read by nobody, which is the fate `preferences.md` shares with the KB.

**`rcrm.md`** is measured vendor behavior — a domain reference. Right shape, right place (listed in the head); nothing to standardize.

**`evidence.md`** shows the extension pattern the audit blessed: a generated region followed by a hand-written *"(alpha specifics)"* section. That works and needs no tooling. (SleepyHollow's head does not list `project/agents/delegation.md`, but the `delegation-brief` region names the path, so there is nothing for the tool to check.)

## 3. Should we encourage a knowledge base? The Nobedan trial

A subagent mapped `kb/` (45 files, ~34K words, two-hop hand-maintained indexes) and answered five questions as a fresh agent would.

- **Navigation is good.** Section READMEs with one-liners and "used-in" columns got to the right entry in 2–3 hops every time.
- **Recency is invisible and staleness is everywhere the money is.** Zero deletions or renames in five months. A superseded offer still appears in two entries; the rate card lists a retainer the section index calls superseded; one doc describes a PDF layout and tool API dead since March; a decision is recorded as *open* although a later dated project doc settled it. Corrections accrete as revision-note callouts above bodies that still lie — the exact anti-pattern the docs module's *"edit the premise"* line names.
- **The dated `project/` docs did the memory work.** Two of five questions were answered only there; one the KB actively misled on. Every session pays ~3.3K tokens of preamble plus a standing order to keep the KB current, honored in 12 commits total.
- **What it is good at**: stable ideas — frameworks, reframes, personas. A curriculum, not a memory.

**Verdict: don't encourage a general KB.** The structure we already ship is a lightweight, tool-agnostic project memory with the right gradient: the head (always read, short) → `gotchas.md` (session start, pruned) → dated `project/` docs (point-in-time, corrected in place) → `backlog.md`. What it lacks is exactly one layer: a **current-state file** rewritten rather than appended — the `background.md` above. The head's Documentation list *is* the index; keep the index there rather than growing a second one. If a repo genuinely needs a curriculum (Nobedan does), three rules from the trial make it survivable: `updated:` frontmatter so an index script can flag age; one source per fact; rewrite the body and keep the revision note as a one-line changelog.

## 4. Rules proposed by the projects — review

Harvested from every `rule:` entry, the gotchas that are general rules in disguise, and Alpha's three proposal docs. Grouped by where the fix lands.

### Fix in Jobs (the tool) — done 2026-08-28
`-F` landed on `note`, `done`, `add` and `edit`, and `orient` exits 0 on an empty tree; core's `-m` line now teaches `-F` for both tools and the three `rule:` gotchas below are retired.

- **`-F <file>` on every verb that takes a message** — `note`, `done`, `add`. Three repos filed the same `rule:` because core mentions `job note -m` as a hazard and agents infer `-F` exists. Stdin `-` stays; `-F` is what the git rule already teaches, so one habit covers both. Then reword core.
- **`job done` has no stdin form** (Organizize) — same change.
- **`job orient` errors when the focused root has no available tasks** (Alpha; reproduced here today) — should clear or say "focused root X is empty" rather than fail the rule's first step.
- ~~No way for an agent to mark a criterion~~ (SleepyHollow) — `job edit --set-criterion label=passed` already exists; a doc gap for the jobs module, not a tool gap.

### Fix in modules
- **delegation**: worktrees branch from *local HEAD* at dispatch, not `origin/main` — commit before dispatching, pushing is backup (SleepyHollow verified; Alpha's retro agrees). Add the worktree create/remove commands (Alpha). Replace the `git apply` return trip with *merge the agent's branch* (Alpha :139 supersedes :103). Scratchpad is shared across parallel agents — prefix files with the leaf id (Organizize). Worktree-isolated agents cannot run compound shell commands (Organizize). A fan-out is a decision, not a default; require a *deviations* section in the report (Alpha retro).
- **core / git**: `| tail` on a commit hides its failure and the next `&&` deleted a worktree holding the only copy of an agent's work (Organizize) — never pipe a gating command. Reword the `-m` line once `-F` exists.
- **web** (done 2026-08-28): Nobedan's `rule:` — the second engine after WebKit should be Blink (Chrome + Android), not Firefox — and agents were double-checking trivia in both engines, so `sleepy` is the day-to-day check and Blink is a final-QA step only. `sleepy` facts (absolute path, named flags, invisible in the sandbox, `rAF` never fires, full-page shots race image decode) — point at `sleepy help` rather than copying flag shapes.
- **swift**: the formatter's `redundantType` rewrites to `.init(…)`, so the *"no `.init()` shorthand"* line fights the pre-commit gate (SleepyHollow). Let the formatter win.
- **go**: never hold a SQLite transaction across a model call (Alpha; the audit flagged it as un-hostable); the pre-commit hook re-stages `gofmt` but not `go fix` rewrites (Nobedan); `go test ./...` on a shared database needs `-p 1` and per-agent databases (Organizize).
- **evidence**: a check that ran over zero cases is not green — Postgres tests that skip without `DATABASE_URL`, `swiftformat --lint` in a worktree that excludes `.claude` (Nobedan, SleepyHollow); assert lower time bounds tightly and upper bounds loosely (SleepyHollow); a running `go:embed` server serves the previous build (Organizize); a test asserting placeholder text passes with no real data (Organizize).
- **new `harness` module**: the Claude Code sandbox facts every repo rediscovered — toolchain, git, psql and listening servers need the sandbox off, and the error strings to recognize (`operation not permitted`, `unable to access '~/.gitconfig'`, `package fmt is not in std`, `sandbox_apply`); `$TMPDIR` differs per mode, write scratch by absolute path; the `!` runner and `ssh -t` have no keyboard, so typed-confirmation guards abort — do not fake a TTY past one.
- **new `prompting` module** (from `preferences.md`): writing prompts and template guidance for models.
- **principles**: the shared-struct-across-a-seam corollary; typed fact over multi-meaning bool.

### Already landed or superseded
Alpha's 2026-06-11 delegation retro and 2026-07-31 compression proposal are largely what `delegation.md` and `evidence.md` now are; the compression rule — *keep the statement and the why, lose the diagnosis story* — is the same rule §1 asks gotchas to follow.

## 5. Two open items from the review session

**A plan-writing guide.** It belongs on its own, not inside the jobs module: jobs is the mechanics of the tracker; a plan is the thinking before it — goal, verified facts, options weighed, waves, file ownership, criteria — and its last section hands off to jobs ("turning the plan into leaves"). Write it first as a dated doc here to converge, then promote to a `planning.md` file module beside delegation and jobs, each cross-referencing the others.

**`agents` tooling.** Two repos show `stale` regions (Alpha 2, SleepyHollow 1) — sync is not being run; a pre-commit reminder or `agents status` in `job orient`'s neighbourhood would close that. Add the two warnings above: gotchas over budget, gotchas preamble out of date. The Jobs-side proposals are filed as `Jobs/project/2026-08-28-agents-feedback.md` (importable with `job import`).

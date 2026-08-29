# Plan: acting on the practice assessment

*2026-08-28.* Turns [2026-08-28-practice-assessment.md](2026-08-28-practice-assessment.md) into work, after a review of its recommendations with the maintainer the same day. The Jobs-side fixes (`-F`, non-failing `orient`) landed today and core already teaches them; `design-process` is a WIP and out of scope. Imported 2026-08-28 as umbrella `iZ7pQ`. Reviewed against [2026-08-28-planning.md](2026-08-28-planning.md) after the fact: it lacks a Verified facts section (they live in the assessment), Options weighed, numbered Decisions and per-wave file ownership — the last of which cost a hand-reserved README at integration.

## Decisions

- **`agents add <module>` / `remove <module>` / `list`.** `add` appends to `.agents.yaml`, renders the region and installs the generated file. `remove` drops the entry, deletes the region, and deletes `project/agents/<file>` only if its hash matches the rendered content — a hand-extended file stays, with a warning. Neither touches the project-owned head; the head's Documentation list is the human's.
- **Seeds are declared in module frontmatter**, not in Go: `seeds: [project/gotchas.md]` on `core`, `project/backlog.md` on `docs`, `project/background.md` on the new `background` module. Bodies live in `templates/<path>`. `init` and `sync` create a declared seed that is missing and never overwrite one. The head template stays a special case of `init`.
- **Gotchas budget.** `status` and `sync` warn above 15 entries or 150 lines with an accurate hint: retire fixed/obvious entries; promote general ones to a module (`agents diff` lists the `rule:` candidates). `sync --reseed-gotchas` replaces a pre-template preamble above the first `---`. Warn, never refuse.
- **Stale-sync adoption.** New verb `agents check`: silent and exit 0 when clean, one line and exit 1 when any region is stale or hand-edited — made to be a pre-commit line. `status` prints a one-time hint with the exact line when the repo's hook (`.git/hooks/pre-commit` or a tracked `scripts/git-hooks/pre-commit`) doesn't call it. We never patch a consumer's hook. This repo gets a post-commit hook that runs `make sync`, so module commits reach the repos before their hooks can complain.
- **One `harness` file module** (`project/agents/harness.md`, region `harness-brief`) with a section per harness — Claude Code only today. A second harness is a section, not a module.
- **`jobs` file module.** Main thread uses the database's default identity; subagents always pass `--as <name>` and an absolute `--db`; agents `claim`/`note`/`release`, never `done` or commit. Criteria via `job edit --set-criterion label=passed`. Container leaf plus work leaves are minted before dispatch. Cross-references `delegation` each way.
- **Delegation is a rewrite, reviewed as a draft before it syncs.** Adopted: (a) commit before dispatching — a harness-made worktree branches from local HEAD, a hand-made one from whatever `git worktree add` names (`main`, not `origin/main`); push is backup; (b) the worktree create/remove commands, and the fact that an agent's shell starts in the main checkout; (c) integrate by wip-committing the agent's branch with hooks off and `git merge --no-ff --no-commit wt/<name>`, not `git apply` — **corrected the same evening: `--no-ff` was wrong.** It keeps the hooks-off `wip` snapshot as the merge's second parent, so a day of integrations left eighteen `wip` commits in SleepyHollow's `git log`. The maintainer's ruling: only the integrator commits, and nothing named `wip` reaches `main` — the module now says `git merge --squash`, a real message drafted by the agent and edited by the integrator, and a push as a step in both the dispatch and integration loops; (d) prefix scratchpad files with the leaf id and read back what you pass to `-F`; (e) compound-command refusal is a harness fact → `harness.md`, one line in the briefing template; (f) a *Carving the work* section — fan-out is a decision; map file surfaces, parallelize the disjoint set, pre-carve or reserve a contended file to one writer, serialize the rest; one agent carries several leaves in the same files because the merge cost, not the task count, decides the split; carving occasionally means minting new leaves for a piece of a task, but rarely — and a required *deviations* item in the report, distinct from "what in this brief is wrong". Jobs bookkeeping moves to `jobs`; the "honest caveat" becomes a sentence in step 6.
- **Alpha cleanup happens after the modules land**: delete `project/agents/jobs.md` and `preferences.md` there, and `engagement.md` once `background.md` carries it.
- Every wave ends with `make sync` and a commit per managed repo (only the synced files).

## Waves

1. Core and templates pass — small wording changes with big leverage.
2. Tooling — seeds, add/remove/list, check, budget warning, reseed.
3. New modules — jobs, harness, background; Alpha cleanup.
4. Module edits — delegation rewrite (draft first), go, evidence, principles, swift.
5. Prompting module; the plan-writing doc.

```yaml
tasks:
  - title: Act on the 2026-08-28 practice assessment
    desc: >-
      Umbrella for project/2026-08-28-practice-plan.md. The assessment (project/2026-08-28-practice-assessment.md) found gotchas files that grow and are never pruned, harness and Jobs facts rediscovered per repo, hand-written files in Alpha's project/agents/ that should be shared modules, and a delegation guide that has been overtaken by verified practice. Each wave ends with `make sync` and one commit per managed repo containing only the synced files.
    labels: [practice-2026-08]
    children:
      - title: "Wave 1: core and templates"
        children:
          - title: Core makes reading gotchas the prune trigger
            ref: prune-trigger
            desc: >-
              Change core's "read project/gotchas.md" line so that reading is also the moment to delete: while reading, delete any entry that is now fixed, obvious, or a general rule — and if it is general, say so as a `rule:` entry. Appending has a trigger (something just cost you time); deleting has had none, and no repo has pruned. Update the pinned hash in internal/module/hash_test.go and re-render AGENTS.md.
            criteria:
              - "modules/core.md tells the reader to prune while reading and to mark general entries `rule:`"
              - "TestEmbeddedModuleHashes passes and `agents sync` is a no-op here"
          - title: Core forbids piping a gating command
            ref: no-pipe
            desc: >-
              Add to core's Git section: never pipe a gating command (`git commit … | tail`) — the pipe hides its exit status and the next `&&` runs anyway. In Organizize that deleted a worktree holding the only copy of an agent's work. Also bless the local-rulings convention in one sentence: a repo-local ruling, or an override of a shared rule, lives in the head of AGENTS.md, says that it overrides, and links a dated project doc for the why.
            criteria:
              - "core has a one-line rule against piping gating commands"
              - "core states where a local ruling or an override of a shared rule lives"
          - title: Gotchas template caps the entry shape
            ref: shape-cap
            desc: >-
              In templates/project/gotchas.md, state the shape: one dated H2 headline plus one paragraph. If it needs more than that, it is a finding — write the dated project doc and link it from the paragraph. Alpha's entries average 13 lines; SleepyHollow's one-paragraph form carries the same information in a quarter of the space.
            criteria:
              - "the template preamble states the headline-plus-one-paragraph shape and the dated-doc escape hatch"
          - title: Swift lets the formatter win on `.init()`
            desc: >-
              The swift module says "no `.init()` shorthand", but swiftformat's `redundantType` rule rewrites to `.init(…)`, so the rule fights the pre-commit gate (SleepyHollow). Drop or invert the line so the formatter's output is the rule.
            criteria:
              - "modules/swift.md no longer contradicts swiftformat's redundantType rewrite"
      - title: "Wave 2: tooling"
        children:
          - title: Modules declare their seeds in frontmatter
            ref: seeds
            desc: >-
              Add `seeds: [<repo path>, …]` to module frontmatter; bodies live under templates/<path>. `init` and `sync` create any declared seed that is missing and never overwrite an existing one. Move the gotchas seeding out of internal/repo/init.go into core's header (core seeds project/gotchas.md); docs seeds project/backlog.md. The head template remains a special case of init. TDD: tests for a missing seed, an existing seed, and a module with no seeds, red first.
            criteria:
              - "a module's frontmatter can list seeds and the loader exposes them"
              - "`agents sync` creates a missing declared seed and leaves an existing file untouched"
              - "core seeds gotchas.md and docs seeds backlog.md; no seed path is hardcoded in Go"
              - "README documents the seeds key"
          - title: "`agents add`, `remove` and `list`"
            ref: add-remove
            blockedBy: [seeds]
            desc: >-
              `agents add <module…>` appends to .agents.yaml in the given order, renders the new regions in canonical manifest order, installs generated files and seeds. `agents remove <module…>` drops the manifest entry, deletes the region, and deletes the module's generated file only when its hash matches the rendered content; otherwise it warns and leaves the file. `agents list` prints available modules with a mark on the enabled ones. Neither add nor remove touches the project-owned head. Region-order behavior with hand-written sections between regions must be tested, not assumed.
            criteria:
              - "`agents add jobs` on a managed repo enables the module, renders its region and installs its file"
              - "`agents remove jobs` removes the region and the file when unmodified, and keeps a hand-extended file with a warning"
              - "`agents list` shows available and enabled modules"
              - "the head above the first region is byte-identical before and after add/remove"
              - "README's verb table lists the three verbs"
          - title: "`agents check` for pre-commit hooks, with an adoption hint"
            ref: check
            desc: >-
              New verb `agents check`: silent and exit 0 when every region is current and unedited; otherwise one line per problem and exit 1. `agents status` prints a one-time hint (with the exact line to paste) when the repo's pre-commit hook — .git/hooks/pre-commit or a tracked scripts/git-hooks/pre-commit — does not mention `agents check`. The tool never edits a consumer's hook. In this repo, add a post-commit hook (Makefile `hooks` target) that runs `make sync` so module commits reach the repos before their hooks complain.
            criteria:
              - "`agents check` exits 0 silently on a clean repo and 1 with a message on a stale or hand-edited one"
              - "`agents status` hints when the repo's hook lacks `agents check`, naming the hook file it looked at"
              - "this repo's post-commit hook runs `make sync`"
          - title: Gotchas budget warning and preamble reseed
            desc: >-
              `status` and `sync` warn when project/gotchas.md exceeds 15 entries or 150 lines, with the hint: retire entries that are fixed or obvious; promote general ones to a module (`agents diff` lists the `rule:` candidates). Warn only, never refuse. Add `sync --reseed-gotchas`, which replaces everything above the first `---` with the template preamble when it differs (Alpha's file predates the template and has never shown the prune instruction).
            criteria:
              - "status and sync print the budget warning with the hint above 15 entries or 150 lines"
              - "`sync --reseed-gotchas` replaces a pre-template preamble and is a no-op on a current one"
      - title: "Wave 3: new modules"
        children:
          - title: "`jobs` file module"
            ref: jobs-module
            blockedBy: [add-remove]
            desc: >-
              Generalize Alpha's project/agents/jobs.md into a file module (project/agents/jobs.md, region `jobs-brief`). Content: every leaf gets a parent; leaves are units of work grouped by files touched, not one per complaint; a fan-out mints a container leaf plus work leaves before dispatch; the main thread uses the database's default identity while subagents always pass `--as <name>` and an absolute `--db`; agents claim/note/release, never done or commit; criteria are marked with `job edit --set-criterion label=passed`; `-F <file>` on note/done/add/edit; work decided against goes to backlog. Point at delegation for running agents. Retires the `--as`, stdin, criterion and empty-orient gotchas across the repos — delete them when syncing.
            criteria:
              - "modules/jobs.md exists as a file module and renders in this repo"
              - "the module states the identity rule, the never-done rule, the criterion command and the before-dispatch bookkeeping"
              - "the retired gotchas are deleted in the managed repos on sync"
          - title: "`harness` file module"
            ref: harness-module
            blockedBy: [add-remove]
            desc: >-
              One file module (project/agents/harness.md, region `harness-brief`) with a section per harness; only Claude Code today. Content from the assessment: toolchain, git, psql and listening servers need the sandbox off, and the error strings to recognize (`operation not permitted`, `unable to access '~/.gitconfig'`, `package fmt is not in std`, `sandbox_apply`); $TMPDIR differs per mode, so write scratch by absolute path; the `!` runner and `ssh -t` have no keyboard, so typed-confirmation guards abort — never fake a TTY past one; worktree-isolated agents cannot run compound commands (`&&`, `;`, loops, heredoc-plus-command) — split them and use the Write tool for file bodies. Retires the sandbox gotchas in every repo, including this one's.
            criteria:
              - "modules/harness.md exists with a Claude Code section covering the sandbox, $TMPDIR, no-TTY and compound-command facts"
              - "this repo's own sandbox gotcha is deleted"
          - title: "`background` module seeds project/background.md"
            blockedBy: [seeds]
            desc: >-
              A module whose region is one line in core's neighborhood — weigh decisions against project/background.md where it exists — and which seeds templates/project/background.md: who this is for, the people, their constraints, what they want, what they call things. It holds current state and is rewritten, not appended; every number, date or name appears once and links the dated record. Modelled on Alpha's engagement.md.
            criteria:
              - "the seed template exists and states the rewrite-not-append rule"
              - "core or the module tells agents when to read it"
          - title: Alpha cleanup
            blockedBy: [jobs-module, harness-module]
            desc: >-
              After the modules land and sync: delete Alpha's hand-written project/agents/jobs.md and preferences.md (its local rulings move to the head; its general principles are carried by the wave 4 module edits and the prompting module), and move engagement.md's content into project/background.md. Done in the Alpha repo, one commit.
            criteria:
              - "Alpha has no hand-written jobs.md or preferences.md"
              - "Alpha's head carries its local rulings"
      - title: "Wave 4: module edits"
        children:
          - title: Delegation rewrite, reviewed as a draft
            ref: delegation
            blockedBy: [jobs-module, harness-module]
            desc: >-
              Rewrite modules/delegation.md per the Decisions section of the plan: commit before dispatching (harness worktrees branch from local HEAD; hand-made ones from what you name — `main`, not `origin/main`; push is backup); the worktree create/remove commands and the fact that an agent's shell starts in the main checkout; integrate by wip-committing the agent's branch with hooks off and `git merge --no-ff --no-commit wt/<name>`, retiring the `git apply` return trip; scratchpad files prefixed with the leaf id; a Carving the work section (fan-out is a decision; map file surfaces, parallelize the disjoint set, pre-carve or reserve a contended file to one writer, serialize the rest; one agent carries several leaves in the same files because merge cost decides the split; carving occasionally mints a new leaf for a piece of a task, but rarely); a required deviations item in the report distinct from "what in this brief is wrong"; the compound-command line in the briefing template pointing at harness.md; Jobs bookkeeping moved to jobs.md with cross-references each way; the honest caveat folded into step 6. Deliverable is a draft for the maintainer's review before it syncs; steps 2, 3 and 5 must not contradict each other afterwards.
            criteria:
              - "a draft delegation.md covering every adopted item is reviewed by the maintainer before sync"
              - "the merge-based integration replaces git apply throughout, including the traps"
              - "Alpha's and SleepyHollow's delegation `rule:` gotchas are deleted on sync"
          - title: Go module additions
            desc: >-
              Add: never hold a SQLite transaction across a model call (Alpha; un-hostable); the pre-commit hook re-stages gofmt output but not go fix rewrites — run go fix before staging (Nobedan); `go test ./...` against a shared database needs `-p 1` and a database per agent (Organizize).
            criteria:
              - "the three rules are in modules/go.md and the source gotchas are deleted on sync"
          - title: Evidence module additions
            desc: >-
              Add: a check that ran over zero cases is not green — tests that skip without DATABASE_URL, a lint whose config excludes the directory it ran in (Nobedan, SleepyHollow); assert lower time bounds tightly and upper bounds loosely (SleepyHollow); a running go:embed server serves the previous build until restarted (Organizize); a test that asserts placeholder text passes with no real data (Organizize). Keep the statement and the why; lose the diagnosis story.
            criteria:
              - "the four rules are in modules/evidence.md in the module's compressed style and the source gotchas are deleted on sync"
          - title: Principles additions
            desc: >-
              Two corollaries of "strongly typed" from Alpha's preferences.md: one shared struct across a serialization seam, never a hand-written twin on each side; a typed fact over a bool that has acquired several meanings.
            criteria:
              - "both corollaries are in modules/principles.md under Strongly typed"
      - title: "Wave 5: prompting and planning"
        children:
          - title: "`prompting` module"
            blockedBy: [delegation]
            desc: >-
              From Alpha's preferences.md section on writing prompts for models: positive direction over prohibitions, capture the why, fewer rules — every added rule competes for attention — and template guidance. Applies to Alpha, Stack Trace and Organizize's personalization; opt-in.
            criteria:
              - "modules/prompting.md exists and the three repos that need it enable it"
          - title: Plan-writing guide as a dated doc
            blockedBy: [delegation]
            desc: >-
              Write project/<date>-planning.md here first: goal, verified facts, options weighed, waves, file ownership, criteria, and a last section on turning the plan into leaves. Converge on it in use before promoting it to a `planning` file module beside delegation and jobs. Promotion is a later leaf.
            criteria:
              - "the dated doc exists and this plan doc is checked against it"
```

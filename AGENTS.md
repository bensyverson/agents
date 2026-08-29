## Overview

You are working on `agents`, a small Go CLI that renders shared house rules (the markdown modules in `modules/`) into marked regions of each repo's `AGENTS.md` and keeps them in sync. The modules are the product; the tool should stay small.

## Documentation

- [README.md](README.md) — what the tool is and how to start; [DOCS.md](DOCS.md) — the marker convention, module format and every verb
- [project/](project/) — dated design documents; [project/2026-08-28-practice-plan.md](project/2026-08-28-practice-plan.md) is the current plan (umbrella `iZ7pQ`), [project/2026-08-28-practice-assessment.md](project/2026-08-28-practice-assessment.md) the assessment behind it, [project/2026-08-28-planning.md](project/2026-08-28-planning.md) how plans are written, [project/2026-08-16-plan.md](project/2026-08-16-plan.md) the v1 design, [project/2026-08-29-public-repo.md](project/2026-08-29-public-repo.md) the confidentiality ruling
- [modules/](modules/) — the shared rules themselves; [templates/](templates/) — files seeded into managed repos

## Confidentiality

**This repo is public. Never name a managed repo, its client, or anything lifted from its content** — figures, filenames, quoted rulings — in plans, assessments, gotchas, tests or commit messages; refer to repos by pseudonym or by shape. Per-repo working notes go in `local/` (gitignored). Why: [project/2026-08-29-public-repo.md](project/2026-08-29-public-repo.md).

This file is itself rendered from those modules by the tool (`agents sync` here must always be a no-op) — the regions below are what every managed repo receives.

<!-- agents:begin core@3a7a5e -->
## Working rules

**Understand the why.** If the goal behind a request isn't clear, ask before solving — beware the XY problem.

**Diverge, then converge.** First brainstorm options (create choices), weigh them against the user's goals, recommend one (make choices), confirm, then execute.

**Ambiguity.** If the *code* could go several ways, choose the idiomatic one for the language. If the *requirement* is ambiguous or the question is architectural, stop and ask — don't decide.

**Dependencies.** Avoid them unless re-implementing would be unreasonable; ask before adding one; each is security and maintenance surface.

**TDD, strictly red/green.** Write tests for every case and every new method first, watch them *all* fail, then implement. A test that is green during red tests nothing — remove or rewrite it. If an existing test must change to pass because the behavior or expectation has changed, explain why clearly. Every bug fix starts with a regression test.

**Plans and tasks live in `job`.** Open every session with `job orient` (no arguments), then read `project/gotchas.md` — while reading, prune it: delete any entry that's now fixed, obvious, or a general rule, marking it `rule:` first if it's general. Don't use Plan Mode or ad-hoc todo lists.

**Don't tour the codebase.** Start from the README and the docs (an Explore agent is fine); dig only where the task leads — once you have a specific need, read as much as that need requires.

**Scripts.** Analysis tooling goes in `scripts/` so it can be re-run — check there before writing one.

**Critique before declaring done.** Re-read the original request: is the need actually met? Do lint and tests pass? Are docs updated? What would an expert flag? Fix serious flaws before reporting.

**Tidiness.** No stray files in the repo root; delete transients, and file valuable artifacts (reports, scripts) where they belong.

**Documentation.** Keep the project docs current as you build. Touch the README only when a doc file is added or new users must know.

**Gotchas.** When a project quirk costs you time and no rule predicts it, append it to `project/gotchas.md`. If a rule in this file was wrong or misled you, record that there too, prefixed `rule:`.

**Where these rules come from.** The marked regions are generated and shared across repos via a CLI tool named `agents`; don't edit inside them. If a rule here is wrong or cost you time, say so in `project/gotchas.md` prefixed `rule:`; that is how shared rules get reviewed.

**Local rulings.** A repo-local ruling, or an override of a shared rule, lives in the project-owned head of `AGENTS.md`, above the generated regions — say plainly that it overrides, and link a dated project doc for the why.

## Git

- Offer to commit when a unit of work is complete and accepted. Rebase onto upstream; ask on real conflicts, explaining the conflict in plain terms first.
- Commit all uncommitted files together — later changes usually depend on earlier ones, and a half-working state helps nobody. Never amend.
- The subject completes "This commit…": present-tense verb first — "Adds…", "Fixes…", "Retires…". Detail goes in the body.
- Pass the message with `-F <file>`, not inline `-m`; the shell interprets `-m` first. Same for `job`: `note`, `done`, `add` and `edit` all take `-F <file>` (`-F -` reads stdin).
- Pre-commit hooks run the formatter and tests. Run them yourself first (see the stack rules).
- Never pipe a gating command (`git commit … | tail`) — the pipe swallows its exit status, so a following `&&` runs even after a failure.
<!-- agents:end core -->

<!-- agents:begin principles@7a5b19 -->
## Principles

Defaults, not laws. When we break one, we do it consciously and say so in the report and the docs.

- **Pragmatism.** Builders, not purists. Practical choices that serve the near-term goal and protect the long-term one.
- **Eat the frog.** No band-aids. Given an easy-but-compromised path and a correct one, take the correct one; fix problems at the source. Keep YAGNI in mind, but when a need is obvious, don't underdeliver.
- **Composability.** Simple, strong components composed into systems — never a monolith.
- **Library + thin executable.** Core logic in a library; the app or CLI is a light consumer, so the core can be reused elsewhere. An adapter that holds a decision rather than wiring one is a bug.
- **Decoupling.** Tight coupling makes testing, debugging and refactoring hard — separate concerns. Separating a model, its storage and its UI is the everyday case: databases and UI frameworks change; today's web app may grow a CLI or mobile app.
- **Just enough abstraction.** One layer around an LLM provider is prudent; a `TextGenerationProvider` above it is not.
- **Readable file sizes.** Aim for files a reader can hold in their head (a few hundred lines; ~400 is the comfortable ceiling). Past ~2k lines, navigation degrades and errors accumulate; splitting also makes functionality discoverable by filename.
- **Comments say why, not what.** Doc comments state *what* concisely; other comments only explain the non-obvious. No change history in comments. Most code needs none.
- **Strongly typed.** Prefer enums, named constants and config over magic strings and numbers; prefer typed structs over dictionaries, even for wire types. Two packages exchanging data across a serialization seam share **one** struct that both import, never a hand-written twin on each side — the type checker cannot see across encode/decode, and two definitions drift. Given a bool and a typed constant, take the typed constant: a bool named for one consequence gets reused to gate the others until it means several things, so name the underlying *fact* as a type and let the behaviors follow.
- **Previews.** Give each UI component a way to render in its various states — a SwiftUI `#Preview`, a demo page, a story — the foundation for tests and for human review.
- **Async by default.** Keep the app interactive during heavy work; surface loading and error states. On the web, prefer progressive enhancement over full reloads.
- **Event streams where they fit.** Append-only logs are auditable, undoable, and time-travelable.
<!-- agents:end principles -->

<!-- agents:begin stage-build@3d5d83 -->
## Stage: BUILD

Pre-launch, zero users, no existing data. Never spend effort on backward compatibility — assume every use is green-field — but flag breaking changes and update the affected tests. Be ambitious: if a feature is important, build it fully now rather than an MVP; balance that against over-engineering and future-proofing.
<!-- agents:end stage-build -->

<!-- agents:begin go@91ab6a -->
## Go

- Before committing: `go fix ./...`, `gofmt -w .`, `go vet ./...` and `go mod tidy`, then the tests you touched. `go fix` converges over several passes — "re-run to apply more" is progress, not failure; re-run until clean before editing code.
- **Run `go fix ./...` before staging, not just before committing.** A pre-commit hook that re-stages `gofmt` rewrites will not re-stage `go fix` rewrites: a file `go fix` changed that is already gofmt-clean commits unfixed, and your working tree quietly diverges from what you committed.
- **Tests that share a database need `-p 1` and a database per agent.** `go test ./...` runs packages in parallel, so packages that seed the same fixtures and truncate the same tables produce a wall of unrelated-looking failures that survives a re-run and reads as a real regression.
- **Schema changes are numbered migrations** in the project's migrations directory (the head names it). Never run one by hand — the binary migrates when it starts or opens the database and records the version; the next run applies it. Read the full note history on the task (`job show <id>`) before writing schema; it is the most expensive thing to change.
- **On SQLite:** **`CHECK` passes on NULL.** `CHECK (a = b)` admits any row where either side is NULL — guard every comparison with `IS NOT NULL`, or it enforces nothing.
- **On SQLite:** **NULLs are distinct in a `UNIQUE` index.** A nullable column in a dedup key admits duplicates forever; wrap it in `COALESCE(col, '')` in the index expression.
- **On SQLite:** **Never hold a transaction open across a model or network call.** `BeginTx` is deferred — it pins a read snapshot at the first read, so the write at the end fails with `SQLITE_BUSY_SNAPSHOT` if any other connection committed meanwhile, and `busy_timeout` cannot rescue it because waiting cannot refresh a stale snapshot. Split into a step that reads and calls but writes nothing, and a short transaction that persists the result.
- Wire types are structs, not `map[string]any`, unless the shape is genuinely dynamic.
- **`r.ParseForm()` reads a body only when it is urlencoded**; for multipart it leaves it empty without erroring. Keep one wire format per route — a handler that accepts two body shapes needs two sets of checks where the design wanted one.
<!-- agents:end go -->

<!-- agents:begin jobs-brief@42b137 -->
## Jobs

`job` is the tracker for plans and tasks. **Read `project/agents/jobs.md` before filing or claiming work** — it carries the shape of the tree, criteria and blockers, the identity rules for agents, and how big a leaf should be.

- Subagents pass a unique `--as <name>` and an absolute `--db` on every call; they `claim`, `note` and `release`, never `done`.
<!-- agents:end jobs-brief -->

<!-- agents:begin harness-brief@a03f30 -->
## Harness

The harness an agent runs inside has facts of its own — the Bash sandbox, `$TMPDIR`, no TTY, worktree isolation, background processes. **`project/agents/harness.md` carries them.** Read it the first time a tool call fails with a permission error or a "too complex to verify" refusal, and before writing a brief for a subagent.
<!-- agents:end harness-brief -->

<!-- agents:begin background@882e19 -->
## Background

**Weigh decisions against `project/background.md` where it exists** — who the work is for, the people involved, the constraints that come with them, what they want, and what they call things. It is context for judging a decision rather than for writing code: read it when a choice turns on who is on the other end, and use its vocabulary in what you write.

**It holds *current* state, so it is rewritten, not appended.** Every number, date and name appears there once and links the dated `project/` document it came from; when a fact changes, edit the sentence that holds it and let the dated record keep the history.
<!-- agents:end background -->


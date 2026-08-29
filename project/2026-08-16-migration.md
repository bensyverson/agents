# Migrating the eight repos — proposals and decisions

*2026-08-16 — analysis, ahead of the migration leaf (OzJNh)*

Eight read-only analyses, one per repo, compared each repo's `AGENTS.md` (and hooks, Makefiles, design docs where the file was stale) against the drafted modules. The per-repo proposals (not kept in this repo) each had the proposed `.agents.yaml`, what stays in the head, what moves to `project/gotchas.md`, candidate module edits, and contradictions. This document is the cross-repo synthesis and the list of decisions the maintainer must make; it is input to the module review (TYlMB) as much as to the migration.

**Status (2026-08-16, end of day):** all eight repos are migrated and `agents status --all` is clean; timetattle's commit is held for the maintainer (its confidentiality gate flags "LLM" in the principles module). Per-repo honest assessments were relayed in the session; the module edits they forced (web no-JS as a default with named exceptions, one-element-per-file scoped to new components, two evidence rules restored, Go migration wording for CLIs) are in.

## Per-repo summary

| Repo | Proposed modules | Gotchas to move | Module edits | Contradictions | Notes |
|---|---|---|---|---|---|
| alpha | core principles stage-build go web docs delegation-brief delegation evidence | 14 | 6 | 2 | Largest file (142 lines); dated briefing template must be retired |
| timetattle | core principles stage-build go docs delegation-brief delegation evidence | 5 | 4 | 2 (global file only) | Costliest trap lives in the Makefile, not AGENTS.md (FDA revoked by `go install`) |
| delta | core principles stage-build python docs delegation-brief delegation evidence | 4 | 6 | 5 | Also has a 69-file Go+SQLite service under `web/server` — `go`/`web` excluded pending a ruling |
| nobedan | core principles stage-build go web docs | 2 (both `rule:`) | 4 | 3 | Monorepo; `go`/`web` govern only `stacktrace/`; Postgres, not SQLite; may be live |
| OperativeKit | core principles stage-build swift web docs delegation-brief delegation | 0 (+2 seeds) | 5 | 4 | **`project/` is gitignored** — blocks init; no `.jobs.db`; no hooks installed |
| Penumbra | core principles stage-build swift swiftui docs (+ delegation-brief delegation evidence, opt-in) | 2 | 5 | 4 | Xcode project, no `Package.swift`: `swift` module's test command and Linux mandate are wrong here; no `.jobs.db` |
| kura | core principles stage-build go web docs | 0 (+2 seeds) | 4 | 3 | Cleanest file; `web`'s progressive-enhancement wording conflicts with its DESIGN.md |
| Jobs | core principles stage-build go web docs delegation-brief delegation evidence | 0 | 5 | 2 | `project/principles.md` becomes a rival source of truth |

Every repo already has `CLAUDE.md -> AGENTS.md`; nothing to flip.

## Decisions before any repo is migrated

1. **Stack modules stack.** Four repos are genuinely two stacks (kura, alpha, Jobs, nobedan: Go + web; delta: Python + Go). **Ruled (2026-08-16):** a manifest lists every stack present; no subdirectory scoping — agents run from the repo root regardless of sub-project.
2. **Stage.** ~~nobedan and delta look post-launch; there is no `stage-live` module.~~ **Ruled (2026-08-16):** a `stage-pilot` module — real users testing, real data with possible monetary liability, so sweeping change is still fine but nothing destructive to existing data — for alpha and delta; nobedan stays on `stage-build`.
3. **`job` adoption.** `core.md` opens every session with `job orient`; OperativeKit and Penumbra have no `.jobs.db`. Proposed: `job init` in each as part of migration (one commit each) rather than a per-repo exception in the head.
4. **OperativeKit's `.gitignore` ignores `project/`.** `gotchas.md`, `backlog.md` and `project/agents/*` would be invisible to git and to any worktree. Must be un-ignored (`!project/gotchas.md` etc., or narrow the pattern to the design docs) before `init`.
5. **Rival sources of truth to retire or redirect** (per `docs.md`, a marked block quote at the top pointing at the rendered region): Jobs `project/principles.md`; alpha `project/2026-07-27-subagent-briefing-template.md`; nobedan's line-1 `@kb/README.md` import (harness-specific — must be rewritten as a plain pointer); kura's head calling `project/` "historical".
6. **TDD scope.** `core.md` is unconditional; delta scopes strict TDD to known-output code and untested refactors (and contradicts itself with a test-after line). Migration silently tightens delta. Confirm.
7. **Head vs gotchas boundary** (asked by five agents). Proposed rule: *standing project facts and rules* (confidentiality gates, `@MainActor`, sibling-library workflow, migrations path) go in the **head**; *incidents and traps that cost time* go in **gotchas**; *feedback about the shared rules* goes in gotchas prefixed `rule:`. Traps found outside AGENTS.md (Makefile comments, hook scripts) are in scope for seeding gotchas — three repos' best entries came from there.
8. **Global `~/.claude/CLAUDE.md`.** ~~Delete the Planning line and resolve the full-suite line.~~ **Resolved (2026-08-16):** the file is now blank and stays blank.

## Candidate module edits, grouped for the module review

Counted by how many repos raised the same point. Full wording and quotes are in the per-repo files.

**`go.md`**
- Drop the literal `internal/migrations/` (nobedan, delta, Jobs; also flagged in the handoff). *"…numbered migrations in the project's migrations directory (named in the head)."*
- "the server migrates on startup" is wrong for CLIs (timetattle) and never says what to do (Jobs): *"the binary migrates on startup and records the version — to apply one, restart it."*
- Prefix the two SQLite bullets *"On SQLite:"* (nobedan is Postgres).
- Pre-commit list should include `go vet ./...` and `go mod tidy` (kura's hook already gates on them).
- Add `r.ParseForm()`/one-wire-format-per-route (alpha) — general to any Go HTTP service.

**`core.md`**
- `job orient` *(no arguments)* (kura, Jobs).
- Git: "ask on real conflicts, **explaining the conflict clearly**" (delta, OperativeKit); restore the atomic-commit rationale "later changes depend on earlier ones" (OperativeKit); Dependencies — "they are security and maintenance surface" (Jobs); the `-m` hazard applies to `job note -m` too (timetattle).
- "Don't tour" lost its permission clause: "…once you have a specific need, read as much as that need requires" (Penumbra).

**`swift.md`**
- `FooBarCore`/`FooBarCommand` rule must be scoped to *new* combined packages — as written OperativeKit itself violates it.
- Cross-platform/Linux mandate is false for app targets; scope to library packages (Penumbra).
- Don't hardcode `swift test --quiet`; "or the project's `xcodebuild` invocation named in the head" (Penumbra).
- File-size (~200–300) vs `principles` (~2k) read as an order-of-magnitude disagreement 40 lines apart; make the tighter number explicit (Penumbra). DocC bullet was demoted from line 1 to last (Penumbra).

**`web.md`**
- "Sub-page changes shouldn't need a full reload" reads as *client routing*, which kura's DESIGN.md forbids as non-negotiable — reword to "enhance, don't SPA" or drop. **Blocks `web` for kura until resolved.**
- Server-side tests can't see the browser; keep a JS/browser runner and run it by hand for anything the browser can observe (alpha — its strongest lesson).
- Styling comes from the project's design tokens/doc (kura); one custom element per file with a shadow root (OperativeKit — new rule, not a restoration).

**`evidence.md`**
- Restore the reason anecdotes are stripped ("a rule you accept on the strength of its anecdote is a rule you have not understood"), "a harness that diverges from production fails in the direction of looking fine", and "query your own mirror before the vendor" (alpha).
- Validate constants/figures against a source and record provenance; render a generated artifact and look at it, don't just read its source (delta).

**`delegation.md` / `delegation-brief.md`**
- Placeholder-migration bullet lost the precondition that makes it safe ("the runner records one row per version") — a reader on a high-water-mark runner will corrupt state (alpha, delta).
- Restore the dot-directory precedent clause (delta); add a short "when to delegate / when not to" section (delta); say that pushback, not typing, is the value of delegating (timetattle).

**`principles.md`**
- "Separate model, storage, and UI" narrowed *Decoupling* to its example; restore the rule and its rationale (Jobs). Optional 400-line anchor (Jobs).
- *Previews* is Apple-shaped; name a web equivalent (OperativeKit). *Library + thin executable*: "an adapter that holds a decision rather than wiring it is a bug" (kura).

## Sequencing proposal

1. The maintainer makes the decisions above and reviews the modules (TYlMB) with the edits list in hand.
2. Re-render this repo (`agents sync`) and commit the module changes.
3. Migrate in this order, one commit per repo, verifying with `agents status --all` after each: kura (cleanest) → Jobs → OperativeKit (after the `.gitignore` and `job init` fixes) → timetattle → nobedan → Penumbra → delta → alpha (largest).

---
seeds: [project/gotchas.md]
---
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

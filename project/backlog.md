# Backlog

Work decided against, kept so the next person doesn't rediscover it. Nothing here is scheduled or blocking — active work lives in `job`. One dated H2 per item: what it is, why it's parked, and what would un-park it.

---

## 2026-08-16 — Model docs in the manifest / generated docs TOC in AGENTS.md

**What.** A `docs:` key in `.agents.yaml` listing each repo's project documents, and a generated region in `AGENTS.md` holding a table of contents rendered from it — so every managed repo's `AGENTS.md` points at its own README, plan and gotchas without anyone hand-writing the links.

**Why it's parked.** The `docs` module plus the project-owned head covers the need today — the head is exactly where a repo lists its own documents, and `status`'s `head=N` shows whether a repo has bothered. Nobody has to hand-model a repo's docs to get that.

**Superseded in part: a generated region now exists.** The original objection was that a region whose body depends on the manifest breaks "a marker hash names a shared standard". The generated `index` region (2026-09-03) does exactly that and is fine, because the objection does not survive contact with it: the index is deterministic from the manifest plus the modules, so any two repos with the same inputs render the same bytes; its marker hash covers *its own bytes*, which is all `check` and `diff` ever needed to tell current from stale from hand-edited; and "identical across repos" was always a property of *modules*, not of every region — the project-owned head has never been identical anywhere. What is left parked is the per-repo docs list: that body depends on facts the manifest does not hold and a human must maintain, which is a different and much weaker bargain than a table assembled from `when:` phrases the modules already carry.

**What would un-park it.** Repos' doc lists visibly drifting (dead links, plans nobody linked), or `status` growing a warning about documents in `project/` that nothing links to — at which point the tool needs to know what the docs *are*, and the manifest is the place to say so.

## 2026-08-28 — Module candidates surfaced by the Alpha cleanup

Alpha's dissolved `preferences.md` held four rules no module carries, now parked in its head: compare emails case-insensitively (go/web); and three on presenting client-facing options — unbiased and parallel, weigh operational ownership over build time, give each option a coherent posture (the assessment assigned these to `design-process`, which turns out to be an IDEO facilitation script with no such content). Un-parks when a second repo needs any of them.

## 2026-08-28 — `sync` appends a region below a hand-written file of the same path

Enabling `jobs` in Alpha rendered the region *under* the old hand-written `project/agents/jobs.md` (the existing text became the file's head — by design). Correct, but an `agents add` of a file module onto a hand-written file should probably say so in its output. Un-parks if it bites again.

## 2026-08-28 — The hook hint doesn't recognise the `pre-commit` framework

delta runs `agents check` through `.pre-commit-config.yaml`, but `agents status` still hints because the framework's generated `.git/hooks/pre-commit` never contains the string. Teach `repo.HookHint` to look for an `agents check` entry in `.pre-commit-config.yaml` when that file exists. Un-parks when a second repo adopts the framework, or the false hint annoys.

## 2026-09-03 — `as:` aliasing for colliding module names

**What.** An `as:` key on a manifest entry, so a repo that enables two modules with the same name from different sources can rename one of them rather than being refused.

**Why it's parked.** Modules reference each other by rendered path (`project/agents/<name>.md`), and a module cannot know what alias its consumer chose — so aliasing the module breaks every sibling reference into it, and the file it renders no longer matches the path the other modules point at. Refusing the collision by name, with both source-qualified names in the message, is honest and costs nothing until it happens. See [2026-09-03-sources-and-situations.md](2026-09-03-sources-and-situations.md), "Name collisions across sources".

**What would un-park it.** A real collision between two sources someone actually wants both of. The likelier fix is sources declaring their own names so rendered paths can be qualified consistently, which keeps sibling references working; aliasing is the fallback if that is too heavy.

## 2026-09-03 — Module folders with assets

**What.** A module as a directory rather than a single markdown file, so it can carry images, scripts or example files alongside its prose and have them rendered into the repo.

**Why it's parked.** No module has needed one yet, and the mechanism is not the copy — it is staleness. A rendered asset has no marker to hash, so the tool would need a second way to tell "current" from "hand-edited" from "the repo's own", which is a real design and not a small one. `seeds:` already covers the one case that exists (a file the repo owns from the moment it exists, never overwritten).

**What would un-park it.** The first module that genuinely needs a non-markdown file to be *kept in sync* rather than seeded once.

## 2026-09-03 — Placing the index anywhere but last

**What.** A way to choose where the generated `index` region lands in `AGENTS.md` — a manifest key, or an `index` entry in the module list marking the slot.

**Why it's parked.** Last is the only placement that needs no configuration and no migration: the index renders after the last inline region, so every existing repo grew one without its head or its other regions moving a byte. A configurable slot means a second ordering rule, an interaction with `add` (which appends), and a repo whose index quietly stops being where the docs say it is.

**What would un-park it.** Evidence that agents miss the table at the bottom of a long `AGENTS.md` — for instance a repeated pattern of dispatching without reading `delegation.md`, which is the same signal that would revisit the `-brief` retirement.

## 2026-09-03 — Emitting `SKILL.md`

**What.** Rendering each `kind: file` module with a `when:` phrase as a harness-native skill file (`SKILL.md` plus frontmatter) in addition to `project/agents/<name>.md`.

**Why it's parked.** The harness-agnostic ruling stands: `AGENTS.md` and its files are read by every coding harness, and a skill format is one vendor's. It is also nearly free to add later — a file module with `when:` already has the skill shape, a situation and a body, so this is a rendering option rather than a format change.

**What would un-park it.** A harness whose skill mechanism demonstrably beats the index table in practice, or a second harness converging on the same format so it stops being vendor-specific.

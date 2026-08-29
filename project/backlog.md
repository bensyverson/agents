# Backlog

Work decided against, kept so the next person doesn't rediscover it. Nothing here is scheduled or blocking — active work lives in `job`. One dated H2 per item: what it is, why it's parked, and what would un-park it.

---

## 2026-08-16 — Model docs in the manifest / generated docs TOC in AGENTS.md

**What.** A `docs:` key in `.agents.yaml` listing each repo's project documents, and a generated region in `AGENTS.md` holding a table of contents rendered from it — so every managed repo's `AGENTS.md` points at its own README, plan and gotchas without anyone hand-writing the links.

**Why it's parked.** There is no templating in v1: a module renders to fixed bytes, identical in every repo, and the marker hash is the hash of those bytes. A per-repo TOC would need a module whose body depends on the manifest, which means a second rendering mode, a second staleness rule, and a hash that no longer identifies a shared standard. The `docs` module plus the project-owned head covers the need today — the head is exactly where a repo lists its own documents, and `status`'s `head=N` shows whether a repo has bothered.

**What would un-park it.** Repos' doc lists visibly drifting (dead links, plans nobody linked), or `status` growing a warning about documents in `project/` that nothing links to — at which point the tool needs to know what the docs *are*, and the manifest is the place to say so.

## 2026-08-28 — Module candidates surfaced by the Alpha cleanup

Alpha's dissolved `preferences.md` held four rules no module carries, now parked in its head: compare emails case-insensitively (go/web); and three on presenting client-facing options — unbiased and parallel, weigh operational ownership over build time, give each option a coherent posture (the assessment assigned these to `design-process`, which turns out to be an IDEO facilitation script with no such content). Un-parks when a second repo needs any of them.

## 2026-08-28 — `sync` appends a region below a hand-written file of the same path

Enabling `jobs` in Alpha rendered the region *under* the old hand-written `project/agents/jobs.md` (the existing text became the file's head — by design). Correct, but an `agents add` of a file module onto a hand-written file should probably say so in its output. Un-parks if it bites again.

## 2026-08-28 — The hook hint doesn't recognise the `pre-commit` framework

delta runs `agents check` through `.pre-commit-config.yaml`, but `agents status` still hints because the framework's generated `.git/hooks/pre-commit` never contains the string. Teach `repo.HookHint` to look for an `agents check` entry in `.pre-commit-config.yaml` when that file exists. Un-parks when a second repo adopts the framework, or the false hint annoys.

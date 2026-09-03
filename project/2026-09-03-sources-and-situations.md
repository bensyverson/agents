# Plan: module sources and situational modules

*2026-09-03.* Two changes to how `agents` works, settled in a design conversation with the maintainer the same day; this doc is the record of what was decided and the work it implies. Imported 2026-09-03 as umbrella `hZ0UQe`; every wave landed the same day, and the fleet is pinned to `github.com/bensyverson/agents-md`.

## Goal

`agents` becomes a package manager for house rules rather than a renderer for one person's. A repo's `.agents.yaml` names one or more **sources** (a git URL or a local path, pinned to a ref) and picks modules from them; the binary embeds only two example modules, and the maintainer's own rules move to a sister repo that this repo consumes like any other. A file module declares the **situation** in which it must be read, and `AGENTS.md` gains one generated index table in place of the five `-brief` modules that exist today only to point at a file. Done means: every registered repo renders from the sister repo through the real fetch path, no `-brief` module exists, `agents check` runs offline from the cache, and `agents update` moves a pin and prints what changed in the rules before rendering.

## Verified facts

Measured 2026-09-03 in this repo unless stated.

| Fact | Command |
|---|---|
| 24 modules, 5 of them `-brief` pointers, 6 `kind: file` | `ls modules \| wc -l`, `ls modules/*-brief.md \| wc -l`, `grep -l 'kind: file' modules/*.md \| wc -l` |
| 11 modules reference a sibling by rendered path (`project/agents/<name>.md`); six distinct targets | `grep -l 'project/agents/' modules/*.md \| wc -l`; `grep -ho 'project/agents/[a-z-]*\.md' modules/*.md \| sort \| uniq -c` |
| Every `kind: file` module's `path:` is already `project/agents/<name>.md` | `grep -A1 'kind: file' modules/*.md` |
| 14 repos in the registry | `wc -l < ~/.config/agents/repos` |
| Go source is 9,229 lines across `internal/{cli,gotchas,manifest,module,registry,render,repo}`; the largest file is 349 lines | `find . -name '*.go' \| xargs wc -l \| sort -n` |
| `Manifest` is `Modules []string`; names match `^[a-z0-9][a-z0-9-]*$` | `internal/manifest/manifest.go` |
| Modules and templates are embedded via `embed.go`; `make install` is the publish step and the post-commit hook runs `make sync` | `embed.go`, `Makefile`, `scripts/git-hooks/post-commit` |
| `go test ./...` fails today: `TestEmbeddedModuleHashes` wants `f7acd9` for `evidence`, gets `2393e6` — the module was edited without re-pinning | `go test ./internal/module/` |
| Seeds come from `templates/` in the binary; three seed templates plus `head.md` | `find templates -type f` |

Assumptions, labelled: the `-brief` tl;dr bullets are not needed in `AGENTS.md` so long as the index row fires before the situation (maintainer's call, 2026-09-03; revisit if `agents diff --all` shows agents dispatching without reading `delegation.md`). Git is present on every machine that runs `agents` (true of every registered repo's machine today).

## Options weighed

**Situational modules: a `-brief` pair per file module (today) vs. a `when:` line and a generated index.** The pair is honest but doubles the manifest entry, and the index is a per-repo rendering, which `project/backlog.md` (2026-08-16) parked on the grounds that a region whose body depends on the manifest breaks "a marker hash names a shared standard". That objection holds for a repo's own docs list and not here: the index is deterministic from manifest plus modules, its hash covers its bytes, which is all `check` and `diff` need, and "identical across repos" was a property of modules, not of every region. The index wins; the backlog entry is revised, not contradicted.

**Where the brief's rules go.** Three choices for the four load-bearing bullets in `delegation-brief` (commit first, agents never commit, integrator commits, verify what comes back): fold into `core`, an optional inline summary under each index row, or the top of the file with the row's situation phrased so it fires first. The third wins: it keeps the pattern pure if-this-then-that and costs nothing to reverse.

**Repo per module vs. a source holding many modules.** Modules cross-reference by rendered path and are coherent as a set, so one ref should pin the set. A source is a repo (or directory) with `modules/` and `templates/` at its root; the manifest picks modules from it. Repo-per-module lost on overhead alone.

**Pinning: Swift's exact/range/branch/revision with a lockfile vs. pre-commit's `rev` in the config.** Modules are prose, so "compatible versions" has no meaning and a range solver buys nothing. pre-commit's shape wins: `ref` in `.agents.yaml` is the pin, no lockfile, `agents update` rewrites it. A human reads one file.

**Fetch: `git` on `$PATH` vs. a Go git library vs. raw URLs.** A library is a large dependency; raw URLs give no listing, no coherent snapshot and no pinning except by hand-editing a sha into a URL. Shelling out wins: private repos work with the user's credentials, local directory URLs work without a push, and a shallow clone into a cache makes `check` offline.

**`sync` fetches vs. a separate `update`.** A fetching `sync` makes a fresh clone non-reproducible, makes `check` (offline, in pre-commit) disagree with `sync` about "current", and moves the model's instructions without a diff. Separate verbs win. Remote prose is a prompt-injection surface, and "pin by default, diff on update" is the mitigation.

**Name collisions across sources: rendered names qualified by source, `as:` aliasing, or an error.** A module cannot know the alias its consumer gave its source, so qualified rendered paths break every sibling reference unless sources declare their own names (a second file format). Aliasing breaks the same references for the aliased module. An error naming both modules wins; source-declared names are the upgrade if it bites.

**What the binary embeds.** Nothing (init needs `--source`), the maintainer's modules as a special `builtin` source, or two generic examples as an ordinary embedded source. Examples win: an offline demo, the format's documentation and the test fixtures in one, and the source abstraction stays honest with three loaders and no special case above them.

**Decided against, parked in `project/backlog.md`:** `as:` aliasing; module folders with assets (wait for the first module that needs one; the asset staleness mechanism is a real design); placing the index anywhere but last; emitting `SKILL.md` (the harness-agnostic ruling stands; a file module with `when:` already has the skill shape, so this stays a rendering option).

## Decisions

1. **`when:` on a file module.** Frontmatter key holding one situation phrase ("Before dispatching any subagent"). Valid only with `kind: file`; a file module without it renders as today and is absent from the index.
2. **The index is a generated region named `index`** in `AGENTS.md`, rendered after the last inline region, only when at least one enabled module carries `when:`. Body: a short preamble then a two-column table, *Situation* and *File*, one row per such module in manifest order. Hashed and checked like any region; never listed in the manifest.
3. **Rendered paths are derived: `project/agents/<name>.md`.** The `path:` key is removed; a module that sets it is a load error. A module references a sibling by that path, and the sibling must be enabled.
4. **The five `-brief` modules are retired.** Each brief's content moves to the top of its file module as the opening lines, and the file gains `when:`. `core`'s own pointers to `project/agents/*.md` stay, since `core` is inline.
5. **`.agents.yaml` gains `sources:`**, a list of `{name, git | path, ref}`; exactly one of `git` and `path`; `ref` optional for `path`. A manifest with no `sources:` uses the embedded `example` source. Module entries are `name` (from the first listed source) or `source/name`. Two enabled modules with the same rendered name are an error naming both.
6. **A source is a directory with `modules/` and `templates/` at its root.** Seeds resolve against the source that holds the module.
7. **`ref` is the pin and the tool writes shas.** `agents add`/`agents update` write the resolved commit sha; a human may write a branch or tag by hand, in which case `sync` renders whatever the cache holds and `update` replaces it with a sha.
8. **Cache and fetch.** Sources are fetched by shelling out to `git` into `$XDG_CACHE_HOME/agents/sources/<hash of URL>` (`$AGENTS_CACHE` overrides): a full mirror clone, not a shallow one, because fetching by sha needs a server option a plain bare repo lacks; each fetched commit's tree is extracted beside it so loading runs no git at all. The user's git config is left alone, since credential helpers and URL rewrites are how a private source is reachable. `sync` and `check` never move a ref; `sync` fetches a source only when the cache lacks it, and `check` fails with a one-line "run `agents sync`" when it does — with `--all` too: an unfetched source is reported like a skipped registry entry but fails the run, since a pre-commit check on a fresh machine must not warn and pass.
9. **`agents update [source…]`** fetches, rewrites `ref` to the resolved sha of `--ref <x>` (default: the remote's HEAD), prints a unified diff of every enabled module's body that changed, then renders. With `--all`, every registered repo. Never interactive.
10. **The binary embeds one source, `example`**, holding two generic modules: an inline `agents` module (house rules for a repo that uses the tool) and a file module `module-authoring` with `when:` (the module format, `when:`, derived paths, sibling references). `init` with no `--source` uses it and prints where the maintainer's rules live. `--modules <dir>` is retired in favour of a `path:` source. Until wave 5 the binary embeds both the first-party `modules/` and `example/`; the wire leaf treats "no `sources:`" as *the embedded source*, which is first-party until the sister repo exists and `example` after — so the fleet never renders from the examples by accident.
11. **The maintainer's modules and templates move to a sister repo** (`github.com/bensyverson/agents-md`) with this repo's post-commit hook and the `sync`/`status`/`diff` Make targets. Dogfood in two steps: first this repo and the fleet consume it as a `path:` source from the local checkout, exercising the source layer before any network is involved; once it is pushed, `update` moves every manifest to the `git:` URL pinned to a sha. `agents sync` here stays a no-op throughout. The confidentiality ruling applies to the sister repo.
12. **Migration is a script**, `scripts/migrate-sources.sh`, that rewrites each registered manifest: adds or replaces the `sources:` block (a `path:` entry on the first pass, the `git:` URL and sha on the second) and drops `-brief` entries. Every registered repo re-renders once per pass and commits.
13. **Every wave ends with** `go fix`, `gofmt`, `go vet`, `go mod tidy`, the tests, a DOCS.md update, and a commit; waves 1 and 3 also end with a fleet sync and a commit per registered repo.

## Waves

Dependency order, not theme. No two leaves in a wave share a file.

**Wave 0: green suite.** Re-pin the `evidence` hash (`internal/module/hash_test.go`) so the suite passes; say in the commit that the module changed without a re-pin.

**Wave 1: formats.** Three disjoint packages, parallel.

| Leaf | Owns |
|---|---|
| `when:` and derived paths | `internal/module/*` |
| Source package: loaders and cache | new `internal/source/*` |
| Manifest: sources and qualified names | `internal/manifest/*` |

**Wave 2: index and examples.** Both need `when:`.

| Leaf | Owns |
|---|---|
| Index region | `internal/render/*`, `internal/repo/sync.go`, `internal/repo/inspect.go`, `internal/repo/addremove.go` |
| Example source | new `example/modules/*`, `example/templates/*`, `embed.go` |

**Wave 3: briefs and wiring.** The briefs need the index; the wiring needs all three formats and the examples.

| Leaf | Owns |
|---|---|
| Retire the briefs | `modules/*.md`, `templates/head.md`, `DOCS.md`, `project/backlog.md` |
| Wire sources into the repo layer | `internal/repo/*` except `update.go`, `internal/cli/root.go`, `internal/cli/addremove.go` |

Ends with `make sync` and a commit per registered repo. `modules/` still renders from the binary until wave 5.

**Wave 4: `agents update`.** Owns new `internal/repo/update.go`, new `internal/cli/update.go`, `DOCS.md`.

**Wave 5: sister repo, consumed by path.** Owns the sister repo and, here, `modules/`, `templates/`, `Makefile`, `scripts/git-hooks/post-commit`, `embed.go`, new `scripts/migrate-sources.sh`, and every registered `.agents.yaml`. Ends with the fleet rendering from the local checkout through a `path:` source, one commit per registered repo.

**Wave 6: switch to git.** After the sister repo is pushed. Owns `scripts/migrate-sources.sh`, every registered `.agents.yaml`, `README.md`, `DOCS.md`, `project/backlog.md`. Ends with `agents update --all`, a commit per registered repo, and this doc's header naming the umbrella.

```yaml
tasks:
  - title: Module sources and situational modules
    desc: >-
      Umbrella for project/2026-09-03-sources-and-situations.md. Makes agents a package manager for house rules: a manifest names sources (git URL or path, pinned to a sha) and picks modules from them; a file module declares the situation in which it must be read and AGENTS.md renders one index table in place of the -brief modules; the binary embeds only two example modules and the maintainer's rules move to a sister repo this repo consumes like any other. Every wave ends with the Go checks, tests, a DOCS.md update and a commit; waves 1 and 3 also end with a fleet sync and a commit per registered repo.
    labels: [sources]
    children:
      - title: "Wave 0: green suite"
        children:
          - title: Re-pin the evidence hash
            ref: repin
            desc: >-
              TestEmbeddedModuleHashes wants f7acd9 for evidence and gets 2393e6: the module was edited without re-pinning. Update the pin in internal/module/hash_test.go and say so in the commit. Owns internal/module/hash_test.go only.
            criteria:
              - "go test ./... passes"
      - title: "Wave 1: module, source and manifest formats"
        children:
          - title: "`when:` frontmatter and derived paths"
            ref: when
            blockedBy: [repin]
            desc: >-
              Decisions 1 and 3. Module gains When string, valid only with kind: file; Path is derived as project/agents/<name>.md and a path: key is a load error. Strict red/green: tests for when on an inline module (error), when on a file module, derived path, and path: rejected. Owns internal/module/*.
            criteria:
              - "A file module with when: loads with the phrase; an inline module with when: is a load error naming the module"
              - "Module.Path is project/agents/<name>.md for every kind: file module and a path: key is a load error"
              - "Existing modules load unchanged once their path: lines are removed"
          - title: Source package
            ref: source
            blockedBy: [repin]
            desc: >-
              Decisions 6, 7, 8. New internal/source: a Source has a name, a git URL or a path, and a ref; three loaders (git, path, embedded) each yield an fs.FS of modules/ and templates/. Git fetches by shelling out into $XDG_CACHE_HOME/agents/sources/<hash of URL> ($AGENTS_CACHE overrides), shallow, and resolves a ref to a sha; an absent cache is a typed error. Tests use a local bare repo made in the test. Owns internal/source/*.
            criteria:
              - "A git source fetched from a local bare repo yields its modules and templates at the pinned sha, and a second load with the cache present makes no git call"
              - "Resolving a branch, a tag and a sha each return the commit sha; an unknown ref is an error naming it"
              - "Loading a git source with no cache and fetch disabled returns the typed not-fetched error"
              - "A path source loads from the directory directly and never touches the cache"
          - title: Manifest sources and qualified names
            ref: manifest
            blockedBy: [repin]
            desc: >-
              Decision 5. .agents.yaml gains sources: [{name, git | path, ref}], exactly one of git and path; module entries are name or source/name with the first source as default; no sources: means the embedded example source. Marshal keeps the two-space canonical form. Two enabled modules with the same rendered name is an error naming both and their sources. Owns internal/manifest/*.
            criteria:
              - "A manifest with two sources and mixed bare and qualified names parses to the right (source, module) pairs and round-trips through Marshal byte-for-byte"
              - "A source with both git and path, or neither, is a parse error"
              - "Two modules rendering to the same name are rejected with both qualified names in the message"
      - title: "Wave 2: index and examples"
        children:
          - title: Index region
            ref: index
            blockedBy: [when]
            desc: >-
              Decision 2. A generated region named index, after the last inline region in AGENTS.md, rendered only when at least one enabled module has when:. Body is a short preamble and a two-column Situation | File table in manifest order. Hashed, checked, diffed and reported like any region; add/remove/sync keep it current; it is never a manifest entry. Owns internal/render/*, internal/repo/sync.go, internal/repo/inspect.go, internal/repo/addremove.go.
            criteria:
              - "sync on a repo with two when: modules renders one index region whose table has two rows in manifest order, and sync again is a no-op"
              - "Removing the last when: module removes the index region; adding one back re-creates it"
              - "check reports the index region as stale when a when: phrase changes and hand-edited when its body is edited"
              - "index in .agents.yaml is an unknown-module error"
          - title: Example source
            ref: example
            blockedBy: [when]
            desc: >-
              Decision 10. example/modules holds two generic modules: agents (inline house rules for a repo that uses the tool: run agents check in pre-commit, never edit inside markers, where the rules come from) and module-authoring (kind: file, when: "Writing or editing an agents module", the module format, when:, derived paths, sibling references by rendered path, templates). embed.go embeds example/ instead of modules/ and templates/. Both modules must be generic enough that nobody mistakes them for a standard. Owns example/*, embed.go.
            criteria:
              - "The binary embeds only example/ and the two modules load through the embedded loader"
              - "module-authoring carries when: and renders to project/agents/module-authoring.md"
              - "Neither example module names a person, a repo or a stack"
      - title: "Wave 3: retire the briefs; wire sources"
        children:
          - title: Retire the -brief modules
            ref: briefs
            blockedBy: [index]
            desc: >-
              Decision 4. Delete the five -brief modules; move each brief's content to the opening lines of its file module and give that module a when: phrase that fires before the situation (delegation: before dispatching any subagent; jobs: before filing or claiming work, and when running as a subagent; harness: the first time a tool call is refused, and before briefing a subagent; design-process: before a meaningful change to user-facing functionality; cli-design: before building or extending a CLI). Remove path: from every file module. Update DOCS.md (module format, layout, index) and revise the parked 2026-08-16 backlog entry on generated TOCs to say why the index is allowed. Then make sync and commit each registered repo. Owns modules/*.md, templates/head.md, DOCS.md, project/backlog.md.
            criteria:
              - "No modules/*-brief.md exists and no module carries path:"
              - "agents sync here is a no-op after the change and every registered repo renders the index with no stale or hand-edited regions"
              - "DOCS.md documents when:, the index region and derived paths, and the backlog entry is revised"
          - title: Wire sources into the repo layer
            ref: wire
            blockedBy: [source, manifest, example]
            desc: >-
              Decisions 5, 6, 8, 10. The repo layer loads modules and seed templates through sources; list shows source-qualified names; add resolves a bare name against the default source; sync fetches a source only when the cache lacks it and check fails with "run agents sync" when it does; --modules is retired. Owns internal/repo/* except update.go, internal/cli/root.go, internal/cli/addremove.go.
            criteria:
              - "init with no --source writes a manifest using the example source and renders both example modules"
              - "A repo with a git source and a path source renders modules from both and seeds from the right source's templates"
              - "check on a repo whose source is not cached exits 1 with a one-line message naming agents sync, and makes no network call"
              - "--modules is gone from every verb"
      - title: "Wave 4: update"
        children:
          - title: "`agents update`"
            ref: update
            blockedBy: [wire]
            desc: >-
              Decision 9. update [source...] fetches each named source (default: all), rewrites ref to the resolved sha of --ref <x> (default: the remote's HEAD), prints a unified diff of every enabled module body that changed, then renders. --all visits every registered repo. Never prompts. Document every verb change in DOCS.md. Owns internal/repo/update.go, internal/cli/update.go, DOCS.md.
            criteria:
              - "After a new commit in the source, update rewrites ref to that sha, prints the diff of the changed module and re-renders its region"
              - "update on an already-current source prints nothing and exits 0"
              - "update --all reports per repo and exits non-zero if any repo refused a hand-edited region"
      - title: "Wave 5: sister repo, consumed by path"
        children:
          - title: Sister repo, consumed by path
            ref: sister
            blockedBy: [update, briefs]
            desc: >-
              Decisions 11 and 12, first pass. Create the sister repo bensyverson/agents-md as a local git checkout with modules/, templates/, the post-commit hook and the sync/status/diff Make targets moved from here; its post-commit runs agents update --all. Remove those from this repo; the Makefile keeps build/test/check/hooks. Add the confidentiality ruling to its head. Write scripts/migrate-sources.sh, which rewrites a registered manifest's sources block and drops -brief entries, and run it with a path: source pointing at the checkout, so this repo and the fleet render from the sister directory through the source layer before any network is involved. Commit each registered repo. Owns the sister repo and, here, modules/, templates/, Makefile, scripts/git-hooks/post-commit, embed.go, scripts/migrate-sources.sh, and every registered .agents.yaml.
            criteria:
              - "The sister repo holds every module and template, and this repo holds none"
              - "Every registered manifest names the sister checkout as a path: source, agents check --all is silent, and agents sync here is a no-op"
              - "agents list in a repo whose source is the sister repo shows every first-party module qualified by source"
      - title: "Wave 6: switch the fleet to git"
        children:
          - title: Switch the fleet to git
            ref: migrate
            blockedBy: [sister]
            desc: >-
              Decisions 11 and 12, second pass, after the sister repo is pushed to github.com/bensyverson/agents-md. Extend scripts/migrate-sources.sh to replace the path: entry with the git: URL, run it across the registry, then agents update --all pins each manifest to a sha through the real fetch path, then a commit per registered repo. agents sync here stays a no-op. README points at the sister repo prominently; DOCS.md describes sources, path versus git, the cache and update; backlog records aliasing, folders, index placement and SKILL.md as parked. Owns scripts/migrate-sources.sh, every registered .agents.yaml, README.md, DOCS.md, project/backlog.md.
            criteria:
              - "Every registered manifest names the sister repo as a git: source pinned to a sha, and agents check --all is silent and exits 0 with the cache cold-fetched from GitHub"
              - "agents sync here is a no-op"
              - "README links the sister repo and DOCS.md covers sources, ref, the cache and update"
```

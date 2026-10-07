# Plan: adding a source from the command line

*2026-10-07.* Acts on [2026-09-24-feedback-adding-a-source.md](2026-09-24-feedback-adding-a-source.md), a session's account of failing to add `bensyverson/agents-md` to a repo made with plain `agents init`, and on the maintainer's report that there is no good way to write `agents init github.com/bensyverson/agents-md` or add a module from that source to an existing repo. Decisions 1–4 were proposed and accepted in conversation the same day; the rest are the smaller calls the feedback note left open.

## Goal

A source is something you add and remove with a verb, never by hand-editing YAML. `agents init github.com/bensyverson/agents-md` makes a working repo with no other arguments; `agents source add github.com/bensyverson/agents-md` followed by `agents add web` adds one to a repo that already exists, including one still on the embedded example modules. Every trap the feedback note hit is either gone or explained by the error it hits. Done means every scenario in the Verified facts table behaves as its decision says, under test, and DOCS.md describes the new verbs.

## Verified facts

Measured 2026-10-07 with a binary built from `ed2322b`, in scratch repos with `AGENTS_CACHE`, `AGENTS_REGISTRY` and `GIT_CONFIG_GLOBAL=/dev/null` pointed away from the real ones.

| Fact | How |
|---|---|
| `init --source github.com/bensyverson/agents-md` fails in `git clone` ("make sure you have the correct access rights"): a scheme-less spec is passed to git verbatim | `agents init --source github.com/bensyverson/agents-md --with core` |
| `init --source https://github.com/bensyverson/agents-md --with core,principles` works and pins a sha; `agents add web` then works and `check` exits 0 | same scratch repo |
| No verb writes `sources:` after `init`; `ParseSource` lives in `internal/repo/init.go` | `agents --help`, `grep -rn ParseSource internal` |
| `init`'s default module list is hard-coded: `repo.DefaultModules()` returns `agents, module-authoring` | `internal/repo/init.go:21` |
| Listing a source in a manifest that names `agents` fails `unknown modules in .agents.yaml: agents-md/agents` — the embedded source is dropped and the error qualifies the name with the new source | hand-edited `.agents.yaml`, `agents sync` |
| `agents add agents` on `modules: []` fails `no modules listed` | `agents add` |
| After swapping an embedded repo to `core, delegation, docs, evidence, jobs, principles, stage-build` by hand, the first `sync` leaves `check` failing (`AGENTS.md: not what the modules render`) while `status` reports `stale=0`; a second sync passes. The only difference is the `index` region's position — the first sync leaves it where the old regions were, the second moves it to the end | `diff` of AGENTS.md across the two syncs |
| The same module set through `init` from scratch converges in one pass | `init --source … --with …; check` |
| `project/agents/module-authoring.md` survives the hand-edit swap and `status` reports `orphans=0` | `ls project/agents; agents status` |

Assumptions, labeled: the feedback's cache-permission failure (#3) is a property of the Claude Code sandbox, not of the tool, and is not reproduced here.

## Options weighed

**Adding a module from a source not yet declared: `agents add github.com/bensyverson/agents-md/web` vs. `agents source add` then `agents add agents-md/web`.** The one-liner is what you'd type, but a general git URL can't be split into repo and module — `host/a/b/c` may be repo `a/b` with module `c` or repo `a/b/c` — so it works only for hosts with a fixed `owner/repo` shape and fails confusingly elsewhere. Two verbs compose and every git URL works. Two verbs win.

**The embedded modules when a real source is added: drop them vs. keep the embedded source under a reserved name.** Keeping it means every resolver, `list` and the manifest writer carry a permanent second namespace for two demo modules. A repo that adds a real source has outgrown the demo. Drop them, and say so.

**Where a source declares its defaults: a `default: true` line in module frontmatter vs. a list at the source root.** Frontmatter keeps the flag beside the module but loses order, and render order is manifest order. A list keeps the order. List wins.

## Decisions

1. **`agents source add <spec> [--name N] [--ref R]`** appends a source to `.agents.yaml`, resolving its pin on the remote exactly as `init` does, and adds no modules. A name already in use is an error. `agents source remove <name>` drops a source, and refuses while an enabled module comes from it, naming those modules — removing rules is `agents remove`'s decision, not a side effect.
2. **Adding the first source to a manifest with no `sources:` block removes the embedded example modules** — their regions and files go as `agents remove` would take them — and prints one line naming what was removed and why.
3. **A scheme-less `host/path` spec is HTTPS.** A spec that is not an existing directory, has no scheme and no scp-style `user@host:`, and whose first segment contains a dot and does not start with one, becomes `https://<spec>`. One parser, used by `init` and `source add`; it moves out of `internal/repo/init.go` to sit beside `manifest.Source`.
4. **A source may declare its default modules** in `source.yaml` at its root: `defaults: [core, principles, …]`, in render order. `init` with no `--with` uses the default source's list; a source with no list and no `--with` is refused as today. The embedded example declares `defaults: [agents, module-authoring]`, and `repo.DefaultModules` is deleted.
5. **`init` takes the source as a positional argument**: `agents init [spec]`. `--source` is removed (BUILD stage; breaking, and the tests change with it).
6. **`agents add` works on an empty `modules:` list.** `ErrNoModules` stays for the verbs that render, not for the one that fills the list.
7. **An unknown module that the embedded source supplies says so**: `agents and module-authoring are example modules built into the binary, which a manifest with sources: does not use; run agents remove agents module-authoring`. The name is not qualified with a source it was never looked up in.
8. **A cache that can't be written names `AGENTS_CACHE`** in the error. No silent `$TMPDIR` fallback: a per-session cache is cold every time, and the offline verbs would fail with a misleading "run `agents sync`".
9. **One `sync` always converges**: after any sync, `check` passes with no further write. The index-placement bug gets a regression test first.
10. **A generated file under `project/agents/` whose region names a module the manifest no longer lists is orphaned**, counted by `status` and reported by `check`, and `sync` removes it the way `agents remove` would.

Out of scope: aliasing, `agents add <url>/<module>` (Options), any `$TMPDIR` cache fallback (Decision 8).

## Waves

One wave, three leaves with disjoint file surfaces, then integration. DOCS.md, README.md and the sister repo's `source.yaml` belong to the integrator: each agent returns the DOCS.md text its change needs in its report. The wave ends with `go fix`, `gofmt`, `go vet`, `go test ./...`, `agents sync` here as a no-op, a commit here, and a commit in agents-md adding `source.yaml` followed by `agents update` here.

- **Source verbs and init** (Decisions 1–5): `internal/repo/init.go`, `internal/repo/sources.go`, new `internal/repo/sourceverbs.go`, `internal/manifest/sources.go`, `internal/module/load.go` and the source loader for `source.yaml`, `internal/cli/init.go`, new `internal/cli/source.go`, `internal/cli/root.go`, `example/source.yaml`, their tests. It may call `repo.Remove`; it does not edit `internal/repo/addremove.go`.
- **Add and errors** (Decisions 6–8): `internal/manifest/manifest.go`, `internal/repo/addremove.go`, `internal/cli/addremove.go`, `internal/source/git.go` (error text only), their tests.
- **Sync convergence and orphan files** (Decisions 9–10): `internal/render/*`, `internal/repo/sync.go`, `internal/repo/inspect.go`, `internal/repo/repo.go`, `internal/cli/check.go`, `internal/cli/status.go`, their tests.

```yaml
tasks:
  - title: Add sources from the command line
    desc: >-
      Umbrella for project/2026-10-07-adding-sources.md. A source is added and removed with a verb, init takes one positionally and uses its declared defaults, and every trap in project/2026-09-24-feedback-adding-a-source.md is fixed or explained. Ends with the suite green, agents sync here a no-op, DOCS.md updated, and source.yaml committed in agents-md.
    labels: [sources]
    children:
      - title: Source verbs and init
        ref: verbs
        desc: >-
          Decisions 1-5. source add/remove, the embedded-module swap, scheme-less host/path specs, source.yaml defaults, positional init with --source removed. Owns internal/repo/{init,sources,sourceverbs}.go, internal/manifest/sources.go, the module/source loader for source.yaml, internal/cli/{init,source,root}.go, example/source.yaml.
        criteria:
          - "agents init github.com/bensyverson/agents-md with no other argument writes a pinned https source and the source's defaults"
          - "agents source add on an embedded-only repo removes the example modules, says so, and agents add web then renders"
          - "agents source remove refuses while an enabled module comes from the source, naming it"
      - title: Add on an empty manifest, and clearer errors
        ref: errors
        desc: >-
          Decisions 6-8. add works on modules: []; an unknown module the embedded source supplies is explained, unqualified; an unwritable cache names AGENTS_CACHE. Owns internal/manifest/manifest.go, internal/repo/addremove.go, internal/cli/addremove.go, error text in internal/source/git.go.
        criteria:
          - "agents add agents on modules: [] enables and renders it"
          - "A sources manifest listing agents fails with a message naming the embedded example modules and agents remove"
          - "An unwritable cache directory produces an error that names AGENTS_CACHE"
      - title: Sync convergence and orphaned generated files
        ref: converge
        desc: >-
          Decisions 9-10. Regression test for the index-placement non-convergence first; then orphaned project/agents/ files are counted, reported and removed by sync. Owns internal/render/*, internal/repo/{sync,inspect,repo}.go, internal/cli/{check,status}.go.
        criteria:
          - "Swapping an embedded repo's manifest to a set with when: modules by hand, one sync leaves check passing"
          - "A project/agents/ file for a module no longer listed is counted by status, reported by check, and removed by sync"
```

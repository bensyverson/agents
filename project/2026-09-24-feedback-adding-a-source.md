# Feedback: adding a source to an already-initialised repo

*2026-09-24.* Written by Claude Code at the maintainer's request, after a failed attempt in `~/git/c3po`. The task was to add `core delegation docs evidence jobs principles stage-build` from `bensyverson/agents-md` to a repo made with plain `agents init` (so it had no sources and used the embedded `agents` and `module-authoring`). It took about six tool calls and ended without adding anything. Below is each place I got stuck, in the order I hit them.

## 1. There's no CLI path to add a source after `init`

`agents add --help` only talks about module names and never mentions sources. `--source` exists only on `init`. The one hint is a sentence in `init --help` ("more sources can be added to .agents.yaml by hand at any time"), which doesn't give the format. I only found the `sources:` schema (`name` / `git` / `ref`, or `path`) by grepping other repos' `.agents.yaml` in `~/git`. An agent without those repos on disk would have been stuck.

**Suggest:** an `agents source add <git-url|dir> [--ref R] [--name N]` verb, or `agents add --source …`. At the very least, `agents help manifest` should document the schema, and `add --help` should point to it.

## 2. Declaring a source makes the embedded modules unresolvable, and the error doesn't say why

After I added the `sources:` block, `agents add` failed with:

```
Error: /Users/ben/git/c3po: unknown modules in .agents.yaml: agents-md/agents, agents-md/module-authoring
```

Those are the two modules `init` turns on by default. They came from the embedded example source, which is apparently dropped once any source is listed. The error makes this confusing:

- It qualifies them as `agents-md/...`, which suggests they were looked up in the new source rather than having lost their old one.
- It doesn't say that the embedded source goes away when sources are declared.
- It doesn't say what to do about it.

So the default route (plain `init`, add a real source later) runs straight into this, with no explanation.

**Suggest:** one of these:

- Keep the embedded source available under a reserved name (e.g. `builtin/agents`).
- Have the error say: "`agents` and `module-authoring` are embedded example modules, which are not available once `.agents.yaml` lists sources. Remove them (`agents remove …`) or list them by path."
- Have the future `source add` verb offer to make the swap.

## 3. The default cache isn't writable in the Claude Code sandbox

The first attempt failed with `preparing the cache: mkdir /Users/ben/Library/Caches/agents/sources/…/clone-…: operation not permitted`. Setting `AGENTS_CACHE=$TMPDIR/agents-cache` got past it. That variable is documented only as a single phrase in the top-level help.

**Suggest:** when the cache can't be written, name `AGENTS_CACHE` in the error. Also consider falling back to `$TMPDIR` when the default location is denied.

**Not verified:** I also set `GIT_CONFIG_GLOBAL=/dev/null` ahead of time, because the sandbox denies reading `~/.gitconfig` and a plain `git log` in `~/git/agents-md` failed on it. I don't know whether the clone would have failed without it.

## State left behind

I restored c3po's `.agents.yaml` to its original contents. `AGENTS.md` was never changed, and `agents check` exits 0. I haven't picked between the fixes in #2; that's up to the maintainer.

## Follow-up: finishing the swap (same day)

The maintainer approved dropping the two embedded modules. Finishing the swap turned up three more problems:

4. **`agents add` refuses an empty manifest.** With `modules: []` (the obvious way to "start fresh on the new source"), `add` fails with `no modules listed`. That's the very state `add` exists to fix. I had to write the module list into `.agents.yaml` by hand and run `sync`.
5. **The first `sync` didn't converge.** Right after `agents sync` printed `rendered AGENTS.md`, `agents check` reported `AGENTS.md: not what the modules render` / `out of date`. A second `sync` fixed it. A single sync should be enough: my guess is that the `index` region is rendered before the file modules it indexes exist. This is unverified.
6. **Dropping a module from the manifest by hand leaves its file behind.** `project/agents/module-authoring.md` (a `kind: file` region) survived the sync, and `check` still passed with it there. `agents remove` presumably cleans this up, but #2 made `remove` unusable, because the manifest wouldn't load. `sync`, or at least `status`, could flag files under `project/agents/` whose module is no longer enabled.

Also: `ref: main` was accepted, and `agents update` then pinned it to `bb0fa63`. That worked well.

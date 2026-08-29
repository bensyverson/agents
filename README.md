# agents

> Keeps the instructions you give coding agents the same across all your repos.

Every repo has an `AGENTS.md` (or `CLAUDE.md`) telling coding agents how to work there. Most of it is the same house rules repeated in every project, so it drifts as you edit one copy and forget the others. `agents` treats those rules like packages: each rule set is a markdown **module**, each repo declares which modules it wants, and `agents sync` renders them into marked regions of `AGENTS.md`. Everything outside the markers stays yours.

## Install

```
go install github.com/bensyverson/agents/cmd/agents@latest
```

## Quick start

In a repo you want to manage:

```
agents init                 # writes .agents.yaml, renders AGENTS.md, links CLAUDE.md
agents add go               # enable another module
agents list                 # what's available, * on what's enabled
```

After changing a module in this repo, roll it out everywhere:

```
make sync                   # reinstall, then agents sync --all
agents diff --all           # review hand-edits and rule feedback from every repo
```

## Workflow

The modules in this repo embody my house style. This is what is working for me at the moment, and this repo will always represent my current recommendations and best practices. To make changes or extensions, clone or fork the repo. In the future I may add support for remote modules.

## Learn more

- [DOCS.md](DOCS.md): the marker convention, module format, every verb and flag, gotchas and the review loop
- [modules/](modules/): the shared rules themselves
- [project/](project/): design docs and plans

---

By [Ben Syverson](https://github.com/bensyverson/). Licensed under the [MIT License](LICENSE).

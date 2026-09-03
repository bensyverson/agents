# agents

> Keeps the instructions you give coding agents the same across all your repos.

Every repo has an `AGENTS.md` (or `CLAUDE.md`) telling coding agents how to work there. Most of it is the same house rules repeated in every project, so it drifts as you edit one copy and forget the others. `agents` treats those rules like packages: each rule set is a markdown **module**, modules live in **sources** (a git repository or a directory), each repo declares which sources and modules it wants, and `agents sync` renders them into marked regions of `AGENTS.md`. Everything outside the markers stays yours.

## Install

```
go install github.com/bensyverson/agents/cmd/agents@latest
```

## Quick start

In a repo you want to manage:

```
agents init                 # writes .agents.yaml, renders AGENTS.md, links CLAUDE.md
agents list                 # what your sources offer, * on what's enabled
agents add <module>         # enable another one
```

With no `--source`, `init` uses the two example modules the binary embeds — enough to see the shape of it, offline. To render somebody's real rules, name the source they live in:

```
agents init --source https://github.com/bensyverson/agents-md --with core,principles,stage-build
agents update               # move the pin and see what the rules now say
```

## Workflow

My own house style lives in [bensyverson/agents-md](https://github.com/bensyverson/agents-md), the source above: it is what is working for me at the moment, and it will always represent my current recommendations and best practices. Fork it, or write a source of your own — a git repository, or a directory, with `modules/` and `templates/` at its root — and point `agents init --source` at it. `.agents.yaml` pins each git source to a commit, so nobody's edit changes what your agents read until you run `agents update`.

## Learn more

- [DOCS.md](DOCS.md): sources and pinning, the marker convention, module format, every verb and flag, gotchas and the review loop
- [bensyverson/agents-md](https://github.com/bensyverson/agents-md): the first-party modules and templates
- [example/](example/): the example source the binary embeds, and the format documenting itself
- [project/](project/): design docs and plans

---

By [Ben Syverson](https://github.com/bensyverson/). Licensed under the [MIT License](LICENSE).

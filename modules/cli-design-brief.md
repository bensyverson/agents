## CLI design

A CLI built for agents is judged by how little an agent must hold in its head to use it correctly, and how hard it is to misuse. **Read `project/agents/cli-design.md` before adding a verb, a flag, an output format or an error message** — it carries the grammar, the house exit-code table, the rules for output, state, batches and errors that teach.

- stdout is the API, `--json` is always one flag away, every read carries a revision, and a write answers with what it made.
- Errors name the thing by path, say what happened, and say the next command; nothing silently does nothing.

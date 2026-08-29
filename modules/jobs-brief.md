## Jobs

`job` is the tracker for plans and tasks. **Read `project/agents/jobs.md` before filing or claiming work** — it carries the shape of the tree, criteria and blockers, the identity rules for agents, and how big a leaf should be.

- Subagents pass a unique `--as <name>` and an absolute `--db` on every call; they `claim`, `note` and `release`, never `done`.

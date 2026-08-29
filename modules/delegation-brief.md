## Delegating to subagents

Design on the main thread; dispatch execution to agents for anything larger than a small change. **Read `project/agents/delegation.md` before dispatching** — it carries what to delegate, how to carve the work, the worktree workflow, the traps, and the briefing template.

- Fanning out is a decision, not a default: map each leaf's file surface first, parallelize only the disjoint set, pre-carve or reserve a contended file to one writer, and serialize the rest.
- Commit before dispatching — a worktree branches from local HEAD, so uncommitted work is invisible to the agent.
- Agents never commit; the integrator makes every commit on `main`: snapshot the agent's branch with hooks off, `git merge --squash` it, read the diff (that is the code review), commit through the hooks with a real message from the agent's proposed one, push, then close the leaves.
- Choose the model deliberately, require **deviations from the brief** and **"what in this brief is wrong?"** in every report, and verify what comes back — the pushback, not the typing, is usually the value.

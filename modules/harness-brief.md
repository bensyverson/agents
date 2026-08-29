## Harness

The harness an agent runs inside has facts of its own — the Bash sandbox, `$TMPDIR`, no TTY, worktree isolation, background processes. **`project/agents/harness.md` carries them.** Read it the first time a tool call fails with a permission error or a "too complex to verify" refusal, and before writing a brief for a subagent.

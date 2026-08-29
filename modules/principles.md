## Principles

Defaults, not laws. When we break one, we do it consciously and say so in the report and the docs.

- **Pragmatism.** Builders, not purists. Practical choices that serve the near-term goal and protect the long-term one.
- **Eat the frog.** No band-aids. Given an easy-but-compromised path and a correct one, take the correct one; fix problems at the source. Keep YAGNI in mind, but when a need is obvious, don't underdeliver.
- **Composability.** Simple, strong components composed into systems — never a monolith.
- **Library + thin executable.** Core logic in a library; the app or CLI is a light consumer, so the core can be reused elsewhere. An adapter that holds a decision rather than wiring one is a bug.
- **Decoupling.** Tight coupling makes testing, debugging and refactoring hard — separate concerns. Separating a model, its storage and its UI is the everyday case: databases and UI frameworks change; today's web app may grow a CLI or mobile app.
- **Just enough abstraction.** One layer around an LLM provider is prudent; a `TextGenerationProvider` above it is not.
- **Readable file sizes.** Aim for files a reader can hold in their head (a few hundred lines; ~400 is the comfortable ceiling). Past ~2k lines, navigation degrades and errors accumulate; splitting also makes functionality discoverable by filename.
- **Comments say why, not what.** Doc comments state *what* concisely; other comments only explain the non-obvious. No change history in comments. Most code needs none.
- **Strongly typed.** Prefer enums, named constants and config over magic strings and numbers; prefer typed structs over dictionaries, even for wire types. Two packages exchanging data across a serialization seam share **one** struct that both import, never a hand-written twin on each side — the type checker cannot see across encode/decode, and two definitions drift. Given a bool and a typed constant, take the typed constant: a bool named for one consequence gets reused to gate the others until it means several things, so name the underlying *fact* as a type and let the behaviors follow.
- **Previews.** Give each UI component a way to render in its various states — a SwiftUI `#Preview`, a demo page, a story — the foundation for tests and for human review.
- **Async by default.** Keep the app interactive during heavy work; surface loading and error states. On the web, prefer progressive enhancement over full reloads.
- **Event streams where they fit.** Append-only logs are auditable, undoable, and time-travelable.

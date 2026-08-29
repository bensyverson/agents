## Swift

- Keep DocC coverage at 100% for any code you add or change.
- Before committing: `swiftformat . --lint` (then `swiftformat .` if needed) and the full suite (`swift test --quiet`, or the project's `xcodebuild` invocation named in the head). Both should be pre-commit hooks; if the repo has none, run them yourself.
- Swift 6 strict concurrency; resolve warnings as you go. No `nonisolated(unsafe)` without permission. Prefer async/await.
- Prefer `struct` for data; `final class` for durable shared-reference objects; `actor` for shared mutable state or a single access point (DB connection, queue).
- New types conform to `Friendly` (`Codable & Hashable & Equatable & Sendable`) even without current plans to serialize or compare.
- **Library packages are cross-platform by default**: stay in Foundation so they build on Linux; wrap Apple-only APIs in `@available` and cover at least macOS and iOS. App targets state their platforms in the head.
- Modern Swift Regex, not the legacy APIs.
- Help the type checker: annotate the type when an initializer's expression is generic, chained, or overloaded (`let output: String = …`), and spell out `Type(...)` rather than `.init(...)` where the type would otherwise be inferred — but where swiftformat's `redundantType` rule disagrees, the formatter's output is the rule.
- **One type per file.** Nest small enums/structs inside their owner. Extensions go in `BaseType+Purpose.swift` (third-party types too). A file over ~200–300 lines wants splitting (tighter than the general guidance, on purpose).
- Sources and Tests organized in folders, at most one level deep. New test suites (new struct) get their own file.
- **A new combined library + CLI package** `FooBar` = library target `FooBarCore` + CLI target `FooBarCommand`.

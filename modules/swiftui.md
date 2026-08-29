## SwiftUI

- **SwiftUI app targets default every declaration to `@MainActor`** (Xcode's default actor isolation for new projects; SwiftPM library targets don't, and the head says if this project differs) — never add `@MainActor` to new types; mark something `nonisolated` only when a protocol requires it (`fileWrapper`, `readableContentTypes`) or a method does blocking I/O or processing that would stall the UI.
- Use `Theme.swift` (the lightweight design system) for all styling.
- One View per file, every View has a working `#Preview`, and previews cover the interesting states.
- Accessibility and localization are first-class from the start; a component isn't done until it has both.
- ViewModels only when needed; name them descriptively and give the model and parameter a descriptive name rather than `viewModel`.
- Folder-based Xcode project: new source and test files are auto-discovered — never touch `project.pbxproj` for them.
- `xcodebuild test` is slow: run it once, pipe to a temp file, grep that. When running inside Xcode, use the Xcode tools.

## Python

- No environment manager: use `python` / `python3` directly.
- Pre-commit runs `ruff`, `ruff-format`, `mypy` and `pytest`. Run `ruff` and `ruff-format` and the tests you touched before committing — the full suite is slow.
- Write type-safe code (annotations, pydantic models) so `mypy` passes first time.
- Test fixtures live in `tests/fixtures/`; copy inputs there rather than depending on transient directories.
- If you're writing a lot of disposable code, PAUSE and check the CLI tools and skills available; if you must, mention it to the user as a candidate enhancement.

BINARY := agents
PKG    := ./cmd/agents

.PHONY: build install run test fmt fix vet tidy check hooks sync status diff clean help

build:
	go build -o $(BINARY) $(PKG)

# The installed binary's embedded modules ARE the published standard:
# `make install` is the publish step for a module change.
install:
	go install $(PKG)

# Usage: make run ARGS="status --all"
run:
	go run $(PKG) $(ARGS)

test:
	go test ./...

fmt:
	gofmt -w .

fix:
	go fix ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# Everything the pre-commit hook checks, runnable by hand first.
check:
	./scripts/git-hooks/pre-commit

# Point git at the checked-in hooks (one-time per clone): pre-commit gates
# the commit, post-commit re-renders every registered repo.
hooks:
	git config core.hooksPath scripts/git-hooks

# The rollout loop after editing a module: publish, re-render every
# registered repo, then show the fleet. Each repo still needs its own commit.
sync: install
	agents sync --all
	agents status --all

status: install
	agents status --all

# The review queue: hand-edits inside regions and rule: gotchas, fleet-wide.
diff: install
	agents diff --all

clean:
	rm -f $(BINARY)

help:
	@echo "Targets:"
	@echo "  build    - compile ./$(BINARY) from $(PKG)"
	@echo "  install  - go install (publishes the embedded modules)"
	@echo "  run      - go run (pass args via ARGS=\"...\")"
	@echo "  test     - go test ./..."
	@echo "  fmt      - gofmt -w ."
	@echo "  fix      - go fix ./..."
	@echo "  vet      - go vet ./..."
	@echo "  tidy     - go mod tidy"
	@echo "  check    - run the pre-commit checks by hand"
	@echo "  hooks    - install the checked-in git hooks (core.hooksPath)"
	@echo "  sync     - install, then agents sync --all and status --all"
	@echo "  status   - install, then agents status --all"
	@echo "  diff     - install, then agents diff --all (the review queue)"
	@echo "  clean    - remove the local binary"

BINARY := agents
PKG    := ./cmd/agents

.PHONY: build install run test fmt fix vet tidy check hooks clean help

build:
	go build -o $(BINARY) $(PKG)

# The binary embeds only the example source; the modules a repo actually
# renders come from the sources its .agents.yaml names.
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

# Point git at the checked-in hooks (one-time per clone): pre-commit runs the
# Go checks and `agents check`, so this repo can never commit a stale region.
hooks:
	git config core.hooksPath scripts/git-hooks

clean:
	rm -f $(BINARY)

help:
	@echo "Targets:"
	@echo "  build    - compile ./$(BINARY) from $(PKG)"
	@echo "  install  - go install"
	@echo "  run      - go run (pass args via ARGS=\"...\")"
	@echo "  test     - go test ./..."
	@echo "  fmt      - gofmt -w ."
	@echo "  fix      - go fix ./..."
	@echo "  vet      - go vet ./..."
	@echo "  tidy     - go mod tidy"
	@echo "  check    - run the pre-commit checks by hand"
	@echo "  hooks    - install the checked-in git hooks (core.hooksPath)"
	@echo "  clean    - remove the local binary"

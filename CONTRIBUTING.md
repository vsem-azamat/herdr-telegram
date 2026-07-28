# Contributing

The project is currently design-first and has no production implementation.

## Before contributing

Read `AGENTS.md` and the documents it links. Open a discussion before changing scope, identity semantics, security boundaries, persistence, process lifecycle, or technology choices.

## Clean-slate policy

Do not copy implementation or prose from existing Telegram/Herdr bridges. Contributions must be independently implementable from authoritative public APIs and this repository's specification.

## Change shape

Prefer one implementation-plan phase or a smaller coherent slice. Avoid mixed refactors, speculative abstractions, and drive-by tooling changes.

## Tests

Behavioral changes follow TDD and include the relevant failure boundaries. No production Telegram credentials or active user Herdr sessions are allowed in tests.

Planned checks:

```bash
test -z "$(gofmt -l .)"
go mod tidy -diff
go vet ./...
go test ./...
go test -race ./...
go run ./tools/validate-docs
```

Until product code exists, validate required documentation, local links, Go module consistency, formatting, repository cleanliness, and absence of likely Telegram tokens.

## Reviews

A phase is not complete until:

1. specification-compliance review passes;
2. quality/security review passes;
3. required commands are executed with real output;
4. documentation matches behavior.

Agents and contributors must not merge or publish without explicit repository-owner approval.

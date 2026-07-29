# Contributing

The project currently ships an initial public Herdr Go SDK. The Telegram bridge remains specification-only and has no runnable product implementation.

## Before contributing

Read `AGENTS.md` and the documents it links. Open a discussion before changing scope, identity semantics, security boundaries, persistence, process lifecycle, or technology choices.

## Clean-slate policy

Do not copy implementation or prose from existing Telegram/Herdr bridges. Contributions must be independently implementable from authoritative public APIs and this repository's specification.

## Change shape

Prefer one implementation-plan phase or a smaller coherent slice. Avoid mixed refactors, speculative abstractions, and drive-by tooling changes.

## Tests

Behavioral changes follow TDD and include the relevant failure boundaries. No production Telegram credentials or active user Herdr sessions are allowed in tests.

## Local setup and checks

Install the tracked hooks once per clone:

```bash
make hooks
```

Run the same gate as CI:

```bash
make check
```

The gate checks required files and local links, Go module consistency, formatting and vetting, normal and race-enabled tests, hook syntax and modes, the temporary no-bridge-product-code boundary, and likely Telegram tokens.

See `docs/development.md` for individual commands and validation conventions.

## Reviews

A phase is not complete until:

1. specification-compliance review passes;
2. quality/security review passes;
3. required commands are executed with real output;
4. documentation matches behavior.

Agents and contributors must not merge or publish without explicit repository-owner approval.

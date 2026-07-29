# Development

## Setup

Use the Go version declared in `go.mod`, then install the repository hooks:

```text
make hooks
make check
```

Hooks are convenience checks. Pre-commit validates an isolated copy of the staged index, pre-push validates the commit tips being pushed, and CI remains authoritative. All three use the same Make targets.

## Python-to-Go tooling map

| Python habit | Go equivalent here |
|---|---|
| `uv` | Go toolchain plus `go.mod` / `go.sum` |
| Ruff formatting | `gofmt` |
| Ruff linting | compiler diagnostics plus `go vet` |
| `ty` | Go's compiler/type checker plus `go vet` |
| pytest | standard `testing` package and `go test` |
| pre-commit | tracked `.githooks/pre-commit` |
| pre-push | tracked `.githooks/pre-push` |
| Pydantic models | typed structs, constructors, and explicit `Validate() error` methods |

`go.sum` authenticates downloaded module content; it is not a fully pinned environment lockfile. Toolchain compatibility is declared by `go.mod` and verified in CI.

## Commands

```text
make format       # rewrite Go files with gofmt
make lint         # formatting plus compiler-aware static checks
make vet          # go vet only
make test         # normal test suite
make test-race    # race-enabled test suite
make docs         # repository/documentation policy
make pre-commit   # fast local commit gate
make pre-push     # full local gate
make check        # complete CI-equivalent validation
```

## Testing style

- Prefer the standard library over assertion frameworks.
- Use table-driven tests with named `t.Run` cases for input matrices.
- Mark shared helpers with `t.Helper()` and isolate files with `t.TempDir()`.
- Compare error identity with `errors.Is` or `errors.As`, not complete messages.
- Use `httptest` for HTTP boundaries and focused fuzz tests for protocol parsers.
- Keep race tests in the full gate; publish coverage for visibility before setting percentage gates.

## Validation style

Do not add a general validation framework by default.

- Local configuration uses typed structs, strict decoding, and explicit semantic validation.
- Domain identifiers use constructors or parsing functions that reject invalid values.
- Telegram and Herdr payloads use narrow transport structs followed by explicit envelope validation.
- External payload decoders remain forward-compatible with unknown upstream fields unless the protocol requires strict rejection.
- Validation errors identify the field and invariant without exposing tokens or native session identifiers.

Add helper libraries only when repeated validation code proves they reduce complexity.

## Later gates

Introduce broader lint aggregation, vulnerability scanning, and coverage thresholds when product packages and external dependencies exist. Pin those tools outside the application dependency graph and keep local and CI versions identical.

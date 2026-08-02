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

## Local Telegram development credentials

Use `.env.example` only as a key-name template. Keep the real bot token outside the worktree in a same-user mode-`0600` regular file under a mode-`0700` directory, and set `BOT_TOKEN_FILE` to its absolute path. `.env` may contain the non-secret development `ADMIN_ID` and private `CHAT_ID`, remains ignored, and should also use mode `0600`.

Never source or print `.env` in CI, place the token in a Bot API command-line argument, or use an intended production bot in automated tests. Live mutation probes require explicit disposable-resource approval.

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

## Opt-in disposable Herdr contract probe

The normal suite skips the live expected-session test. Run it only against a disposable capability-advertising binary; the test creates its own config, socket, workspaces, fake agent process, and session reports:

```text
HERDR_EXPECTED_SESSION_BIN="$HOME/.local/bin/herdr-expected-session" \
  go test ./herdr -run '^TestExpectedSessionForkLive$' -v
```

Do not point this probe at an active user Herdr process or production agent session. The test first requires the capability and uses only its temporary socket endpoint.

The Linux plugin/systemd lifecycle probe is also opt-in. It links a generated plugin only in a temporary Herdr registry and launches a uniquely named transient user unit:

```text
HERDR_PLUGIN_LIFECYCLE_BIN="$HOME/.local/bin/herdr-expected-session" \
  go test -race ./spikes -run '^TestPluginSystemdLifecycleLive$' -v
```

It performs no terminal-agent or Telegram mutation. Its accepted side effect is a marker in `testing.T.TempDir`; cleanup stops the transient unit and disposable Herdr process.

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

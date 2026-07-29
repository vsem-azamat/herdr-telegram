# Technology

## Runtime

The daemon is implemented in Go. Herdr integration uses the executable/plugin boundary, CLI, and newline-delimited JSON socket protocol.

Core standard-library packages:

- `net/http` for direct Telegram Bot API calls;
- `encoding/json` for Telegram and Herdr payloads;
- `context` for deadlines, cancellation, and shutdown;
- `log/slog` for structured redacted logs;
- `database/sql` for explicit persistence transactions;
- `net` for Unix sockets;
- `embed` for migrations and service assets.

The daemon remains one process supervised by `systemd --user`.

## Herdr SDK

The public `herdr` Go package is a small standard-library client for Herdr's newline-delimited JSON Unix-socket API. Its first checkpoint includes bounded unary requests for:

- `ping`;
- `session.snapshot`;
- `agent.list`;
- `agent.get`;
- `agent.prompt` with the protocol's optional wait object.

Each unary call owns one socket connection. The decoder tolerates unknown response fields for forward compatibility, checks request correlation and result discriminators, surfaces typed server and transport-stage errors, bounds responses, and honors context cancellation. Event subscriptions require a separate long-lived connection and are later work.

The SDK connects only to the explicit path supplied by its caller; it does not discover or authenticate a Herdr endpoint. The future bridge adapter remains responsible for the threat model's owner, mode, symlink, descriptor, and peer checks before using the client.

For `agent.prompt`, a write- or read-stage transport failure can be ambiguous: the server may already have accepted the text. The SDK preserves the transport stage but does not retry automatically.

`PromptWait` observes Herdr's agent lifecycle rather than correlating a completion to the submitted text. An empty `until` uses Herdr's `idle`, `done`, or `blocked` default. If a just-prompted non-working agent does not advance its lifecycle sequence, Herdr may return `agent_prompt_stalled`; this remains a typed `APIError`, not proof that the text was not accepted.

Protocol 17 `agent.prompt` accepts `target`, `text`, and optional `wait`; it does not accept an expected native session. The SDK exposes that fact rather than adding a false safety abstraction. Automatic Telegram routing remains blocked by the stronger bridge invariant.

## SQLite

The planned driver is `modernc.org/sqlite`, introduced in the persistence phase rather than the documentation baseline.

Required properties:

- no CGO dependency;
- one deployable binary;
- standard `database/sql` transactions;
- WAL and foreign-key support;
- explicit busy timeout and cancellation behavior;
- restrictive database-file creation;
- embedded, versioned migrations.

Phase 4 must verify these properties and record build-size and operational impact before accepting the dependency. Persistence code must not be hidden behind an ORM.

## Telegram

Use a small direct adapter over `net/http`. The implementation must expose and test:

- `update_id` and durable offset advancement;
- forum topic identity;
- `409` competing-poller failures;
- `429.retry_after`;
- malformed and `ok:false` responses;
- known failures versus ambiguous mutation outcomes;
- request deadlines and response-size limits.

No Telegram bot framework is part of the MVP.

## Testing and tooling

The canonical local and CI gate is:

```text
make check
```

`docs/development.md` defines its individual targets, hook behavior, and validation conventions.

Use:

- standard `testing` package;
- `httptest` for Telegram contract fixtures;
- temporary Unix sockets and SQLite databases for integration tests;
- fuzz tests for protocol and envelope parsers where useful;
- the race detector for every concurrency-bearing phase.

Additional linters must remain scoped and must not replace delivery work.

## Packaging

`go.mod` currently uses the intended public module path:

```text
github.com/vsem-azamat/herdr-telegram
```

No remote is implied by this path. Reconsider it before the first public tag if publication moves elsewhere.

The `go` directive names the oldest supported toolchain line. CI reads the version from `go.mod` and resolves a compatible patch release. There is no patch-pinning `toolchain` directive.

Release requirements:

- build `./cmd/herdr-telegram` without CGO;
- embed migrations and systemd assets;
- publish checksummed binaries for supported targets;
- keep `herdr-plugin.toml` as a repository artifact;
- install the daemon independently from Herdr's managed plugin checkout;
- use the installed absolute binary path in the systemd unit;
- verify `--version`, offline doctor, migrations, service installation, and rollback from the built artifact.

Do not add release automation until the manual release procedure is stable and repetitive.

## Concurrency

Runtime ownership is explicit:

```text
one Telegram poller
one Herdr subscription reader
one serialized topology reconciler
one per-topic turn coordinator
one supervised process
```

Every goroutine requires:

- an owning component;
- a cancellation path;
- bounded channels or documented backpressure;
- shutdown tests;
- no authority to bypass daemon/session fencing.

## Excluded from MVP

- bot frameworks;
- ORMs and query builders;
- CGO by default;
- brokers, distributed locks, and background-job systems;
- generic chat or multiplexer interfaces;
- dependency-injection frameworks;
- unbounded goroutine-per-update concurrency;
- multiple implementations of the daemon.

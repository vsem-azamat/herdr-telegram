# AGENTS.md

## Purpose

This repository is a clean-slate Herdr-native Telegram bridge. Read these before changing anything:

1. `README.md`
2. `docs/architecture.md`
3. `docs/decisions.md`
4. `docs/technology.md`
5. `docs/threat-model.md`
6. `docs/operations.md`
7. `docs/implementation-plan.md`

## Non-negotiable invariants

- Telegram topic identity binds to stable Herdr `agent_session`, never durable `pane_id`.
- Focus is never a routing input or fallback.
- Do not ship prompting unless Herdr provides or proves an atomic expected-session precondition. A local read-then-prompt check has a replacement race.
- Snapshot is authoritative; events are replayable hints.
- Missing/ambiguous/conflicting routes fail closed.
- Never automatically retry an ambiguous `agent.prompt`, `createForumTopic`, or Telegram send.
- One in-flight turn per topic in MVP.
- No automatic support for agents without native `agent_session`.
- No arbitrary shell, pane close, agent termination, topic deletion, or automatic session start/resume in MVP.
- Plugin disable/unlink must prevent remote mutations.

## Clean-slate boundary

Do not copy source, tests, schemas, prose, or Git history from existing bridges. Public APIs, observed behavior, and general patterns may inform design. Implement from this repository's specifications and authoritative Herdr/Telegram documentation.

If you consult comparative repositories, record the conceptual lesson in `docs/references.md`; do not import structure by habit.

## Delivery workflow

- Work one phase from `docs/implementation-plan.md` at a time.
- Use TDD: failing test → verify failure → minimal implementation → verify pass → refactor.
- Keep changes focused; do not pull later-phase scope forward.
- Run the narrow test first, then the phase suite, then full checks.
- Request specification-compliance review before quality/security review.
- Never merge, publish, push a release, configure production credentials, or enable production routing without explicit human approval.

## Technology constraints

MVP stack:

```text
Go (version declared in go.mod)
stdlib net/http + encoding/json + context + log/slog
database/sql + reviewed pure-Go SQLite driver
go:embed for migrations and service assets
standard testing + httptest
gofmt + go vet + go test -race
systemd --user on Linux
```

Do not add bot frameworks, ORMs, CGO by default, brokers, generic provider interfaces, or distributed infrastructure without a new documented decision and concrete need. Every goroutine needs an owner, cancellation path, and bounded backpressure behavior.

## Testing rules

- No automated test may use production Telegram or active user Herdr sessions.
- Contract tests use redacted fixtures.
- Live mutation probes use disposable panes/sessions and pre-provisioned disposable Telegram resources.
- Test crash boundaries and ambiguous outcomes explicitly.
- Never assert exactly-once behavior the upstream APIs cannot guarantee.
- Focus-change tests are mandatory for routing changes.
- Plugin disable/unlink tests are mandatory for new mutation paths.

## Security rules

- Never log, commit, echo, or place the bot token in process arguments.
- Redact Bot API URLs and native session identifiers.
- Validate complete Telegram envelope, not only user/chat IDs.
- Reject unsafe local file/socket ownership or permissions.
- Treat Telegram as remote control over local agents.
- Destructive behavior requires a new threat-model review and explicit scope approval.

## Documentation

When behavior changes, update the relevant architecture/decision/operations/threat document in the same change. Do not let implementation silently become the specification.

## Stop conditions

Stop and surface the issue instead of guessing when:

- explicit pane target behavior differs from protocol fixtures;
- no stable native session identity exists;
- two panes claim the same stable session;
- Telegram or Herdr mutation outcome is ambiguous;
- plugin enabled state cannot be verified;
- runtime instance identity changes;
- tests require production credentials;
- requested scope contradicts a recorded invariant.

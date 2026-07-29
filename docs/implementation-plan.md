# Implementation plan

This document is the execution handoff. It assumes the implementer has read `architecture.md`, `decisions.md`, `technology.md`, `threat-model.md`, and `operations.md`.

This is the sole normative execution plan.

## Delivery rules

- TDD for every behavior-bearing change.
- One phase per focused branch/PR.
- Separate specification-compliance and quality/security reviews.
- Do not combine product scope from later phases.
- Never use production Telegram credentials in automated tests.
- Never merge, publish, or enable production routing without explicit human approval.
- Verify every external side effect from real tool output.

## Phase 0 — Contract spikes and lifecycle proof

**Goal:** Resolve all three implementation-blocker families before Telegram bridge product code.

A focused transport-only SDK checkpoint may precede these spikes. It may model and test Herdr's existing protocol, including low-level `agent.prompt`, but must not implement stable-session routing or claim an expected-session guarantee the server does not expose.

### Herdr explicit target spike

**Current checkpoint:** A disposable live probe verified that protocol 17 can prompt an explicit pane independently of focus. Tagged v0.7.5 schema/source and upstream development source at `73d92004f50d3f5fafe64e0f9b7fddbcf4d99965` (protocol 18) confirm that `agent.prompt` has no atomic expected-session precondition, so steps 5–8 below remain blocked on an upstream API change. Do not repeat the same client-side read-then-prompt experiment as if it could close that gap.

[`spikes/herdr-expected-session.md`](spikes/herdr-expected-session.md) records the minimal backward-compatible proposal and required server-side race tests. Herdr server code and tests belong upstream and require explicit user authorization before opening an issue, fork, or PR. Until that implementation exists, this spike remains an explicit upstream blocker rather than evidence that the gate passed.

Prove on a disposable Herdr workspace that the server can protect the expected stable session atomically:

1. create or choose two disposable agent panes;
2. change focus away from the target;
3. prompt the explicit pane;
4. verify only the intended agent received it;
5. test stale and mismatched targets;
6. force/coordinate occupant replacement between daemon observation and prompt dispatch;
7. verify the server rejects dispatch when the pane no longer hosts the expected `AgentSessionKey`;
8. verify completion evidence remains tied to the same resolved occupant/session;
9. save redacted request/response fixtures.

Do not use a user's active agent session for the mutation probe.

If current Herdr has no atomic expected-session precondition, stop normal-prompt implementation, document the protocol gap, and propose the smallest backward-compatible upstream API change. A local read-then-prompt check is not an acceptable substitute.

### Plugin/systemd lifecycle spike

Prove:

- one-shot startup can atomically register socket/config/state paths;
- the separately installed daemon reads the descriptor;
- systemd restart behavior is bounded;
- plugin disable/unlink prevents a mutation;
- Herdr restart/handoff refreshes the descriptor;
- stale descriptors fail closed.

### Telegram prerequisites spike

Against a disposable bot and pre-provisioned forum:

- validate `getMe`, `getChat`, `getChatMember`, `getWebhookInfo`;
- prove privacy/admin requirements;
- capture `409`, `429`, malformed response, and ambiguous timeout fixtures where practical;
- document manual setup and cleanup.

**Exit evidence:**

- redacted Herdr request/response and race-test fixtures proving atomic expected-session dispatch, or an explicit upstream blocker with prompting disabled;
- systemd/plugin lifecycle transcript covering disable/unlink and stale descriptors;
- disposable Telegram prerequisite/recovery transcript with no production credentials;
- updated normative documents. Telegram bridge product-code phases cannot start while any family remains unresolved.

## Phase 1 — Domain model

**Goal:** Pure typed state and invariants with no I/O.

Create:

```text
internal/domain/identity.go
internal/domain/bindings.go
internal/domain/turns.go
internal/domain/errors.go
internal/domain/*_test.go
```

At the first bridge product-code commit, replace the temporary `cmd/herdr-telegram` / `internal` guard in `tools/validate-docs` with checks for the expected package/test structure. A standalone `herdr` SDK package does not trigger that transition.

Test:

- stable session identity independent of pane;
- new session in same pane is different;
- missing identity is ineligible;
- duplicate route is conflict;
- binding lifecycle separate from route availability;
- route and daemon generation fencing;
- inbox/turn transitions, including `ambiguous`;
- Telegram inbox acknowledgment independent from downstream turn resolution;
- `queued → dispatching → submitted/waiting → done|blocked` and hold/recovery states;
- outbox known-retryable versus ambiguous delivery transitions;
- one in-flight turn per topic;
- no focus-derived route type exists.

## Phase 2 — Herdr bridge adapter

**Goal:** Reuse the public `herdr` SDK behind a narrow bridge port and add only protocol surfaces still required by the daemon.

Extend the public SDK instead of creating a second client:

```text
herdr/subscription.go
herdr/controls.go
herdr/*_test.go
```

Create bridge-specific policy and validation:

```text
internal/herdr/adapter.go
internal/herdr/validation.go
internal/herdr/*_test.go
internal/herdr/testdata/
```

The internal adapter must compose `herdr.Client`; it must not duplicate socket framing, unary envelopes, snapshot structs, prompt encoding, or typed errors.

Test:

- live protocol/version compatibility through the public SDK;
- bridge-required semantic validation for Claude, Codex, and no-session snapshots;
- server-side expected-session prompt behavior once the Phase 0 protocol gate is resolved, including stale-target rejection; no adapter-side read-then-prompt substitute;
- bounded reads and allowlisted controls added to the public SDK;
- disconnect and malformed-response propagation through the adapter;
- event subscription acknowledgment and retained replay on the deferred long-lived SDK connection;
- unknown transport fields tolerated while missing bridge-required semantics fail closed.

## Phase 3 — Reconciler

**Goal:** Deterministically project snapshots to stable session routes.

Create:

```text
internal/app/reconcile.go
internal/app/reconcile_test.go
```

Test:

- idempotent snapshot;
- pane move updates route only;
- session disappears/returns;
- session replacement in same pane;
- duplicate claim quarantined;
- server epoch/revision reset;
- subscribe-buffer-snapshot interleavings;
- route generation changes only when required;
- creation eligibility and circuit breaker.

## Phase 4 — SQLite state and fencing

**Goal:** Durable recovery without persisting live topology.

Create:

```text
internal/store/sqlite.go
internal/store/sqlite_test.go
internal/assets/assets.go
internal/assets/migrations/0001_initial.sql
```

Tables:

```text
schema_migrations
topic_bindings
telegram_inbox
turns
telegram_outbox
checkpoints
topic_creation_attempts
daemon_fence
```

Test:

- migrations from empty DB;
- pure-Go SQLite driver behavior with `database/sql`;
- WAL/foreign keys;
- restrictive file creation;
- uniqueness constraints;
- contiguous next-offset advancement;
- crash/cancellation at transaction boundaries;
- SQLite busy/locked behavior;
- stale daemon fence rejection;
- instance-ID mismatch;
- known-safe outbox retry attempt counters and bounded exhaustion;
- ambiguous outbox resolution as delivered/dropped/retry-as-new;
- `retry-as-new` preserves the original row and links a new row with `retry_of_outbox_id`;
- backup/checkpoint procedure.

## Phase 5 — Telegram adapter and admission

**Goal:** Make Telegram semantics explicit without a framework.

Create:

```text
internal/telegram/client.go
internal/telegram/updates.go
internal/telegram/testdata/
internal/telegram/*_test.go
internal/app/admission.go
internal/app/admission_test.go
```

Methods:

```text
getMe
getChat
getChatMember
getWebhookInfo
getUpdates
sendMessage
createForumTopic
editForumTopic
```

Test:

- envelope allow/deny matrix;
- topic normalization;
- `allowed_updates`;
- sequential batches and next offset;
- 409 terminal competing poller;
- 429 retry-after;
- 5xx backoff;
- invalid JSON and `ok:false`;
- ambiguous network timeout;
- token redaction from every error path.

## Phase 6 — Topic lifecycle

**Goal:** Map eligible sessions to Telegram topics safely.

Create:

```text
internal/app/topic_lifecycle.go
internal/app/topic_lifecycle_test.go
internal/app/topic_creation_integration_test.go
```

Test:

- one known-success creation;
- repeated reconciliation creates none;
- ten-topic cap and circuit breaker;
- ambiguous creation never retries;
- `/bind-pending` binds the current thread;
- confirmed `/retry-create`;
- session move preserves topic;
- new session in old pane creates new binding;
- offline binding remains active;
- manual close/delete becomes `externally_missing`;
- raw session ID never appears in title.

## Phase 7 — Prompt routing

**Goal:** Deliver one admitted topic update to one exact live agent session.

Create:

```text
internal/app/route_update.go
internal/app/turn_coordinator.go
internal/app/route_update_test.go
internal/app/prompt_boundaries_integration_test.go
```

Test every boundary:

1. crash before dispatch;
2. Herdr known rejection;
3. known acceptance and local commit;
4. timeout after possible acceptance;
5. commit failure after acceptance;
6. stale route between resolution and send;
7. plugin disabled between resolution and send;
8. stale daemon generation;
9. duplicate Telegram update;
10. second message queued behind active turn.
11. Telegram offset advances after durable admission even when the associated turn is ambiguous.
12. `waiting_timed_out` and `waiting_offline` retain the per-topic turn lock.
13. `done`, `blocked`, known rejection, and explicit local resolution release the next turn.

Acceptance is durable admission plus fail-closed ambiguity—not exactly-once execution.

## Phase 8 — Daemon and plugin packaging

**Goal:** Assemble one supervised process.

Create:

```text
cmd/herdr-telegram/main.go
internal/daemon/daemon.go
internal/config/config.go
internal/health/health.go
internal/logging/logging.go
internal/assets/systemd/herdr-telegram.service
herdr-plugin.toml
internal/daemon/startup_integration_test.go
```

Build and installation deliverables:

- build `./cmd/herdr-telegram` with the pinned module/toolchain contract;
- embed SQL migrations and systemd assets with `go:embed`;
- keep the Herdr plugin manifest as a separately installable/linkable repository artifact;
- produce checksummed Linux release binaries without CGO;
- inspect executable metadata and embedded assets;
- install the binary into a disposable prefix;
- run CLI `--version`, `doctor --offline`, service-unit render/install/remove, and migration smoke tests from the installed binary;
- verify graceful shutdown and cancellation under `go test -race`.

Test the lifecycle matrix from `operations.md`, including plugin disable/unlink and Herdr restart. The phase is incomplete if the daemon only works through `go run` or from a source checkout.

## Phase 9 — Telegram commands and bounded output

**Goal:** Add usable, limited remote UX.

Commands:

```text
/status
/recent [lines]
/esc
/enter
/key <allowlisted-key>
/doctor
/reconcile
/bind-pending <attempt-id>
/retry-create <attempt-id>
/help
```

No interrupt, pane close, shell, arbitrary argv, or raw socket command in MVP.

Local-only administrative recovery commands:

```text
herdr-telegram admin turn resolve <turn-id> --as delivered|dropped|requeue
herdr-telegram admin outbox resolve <item-id> --as delivered|retry-as-new|dropped
herdr-telegram admin topic-attempt show <attempt-id>
```

`requeue`/`retry-as-new` must create a new auditable row rather than mutating historical inbox or delivery evidence. These commands are not exposed through Telegram in MVP.

Test every outbox transition: known pre-acceptance failure back to `pending`, bounded exhaustion to `failed_permanent`, and each operator resolution from `ambiguous`. A retry-as-new test must prove the ambiguous row remains immutable and the new row links back to it.

Test authentication, revalidation, rate/length limits, route replacement during control dispatch, and output redaction/bounds.

## Phase 10 — Disposable live E2E

Use a pre-provisioned disposable forum and disposable Herdr sessions.

Required scenarios:

- two independent agents in one tab → two topics;
- focus changes do not affect routing;
- simultaneous topic messages remain isolated and serialized;
- pane move preserves topic;
- native session replacement creates a separate topic;
- daemon and Herdr restart recovery;
- retained/replayed events;
- ambiguous prompt dispatch;
- ambiguous topic creation and operator recovery;
- manual topic close/delete;
- competing poller 409;
- plugin disable/unlink;
- token rotation;
- no production secret leakage.

## Phase 11 — Release readiness

Gates:

```text
test -z "$(gofmt -l .)"
go mod tidy -diff
go vet ./...
go test ./...
go test -race ./...
go run ./tools/validate-docs
```

Also require:

- dependency audit;
- secret scan;
- specification review;
- code quality/security review;
- 24-hour dev-stage soak;
- operations rollback test;
- explicit human approval before publishing or production enablement.

## MVP completion

MVP is done only when real output proves:

- exact session routing independent of focus;
- topic continuity across pane relocation;
- session replacement isolation;
- known-outcome restart recovery without duplicate topic creation;
- ambiguous cross-system outcomes are visible and never blindly retried;
- unauthorized identities cannot mutate Herdr;
- plugin disable/unlink prevents mutations;
- no production credentials appear in repository, tests, or normal logs.

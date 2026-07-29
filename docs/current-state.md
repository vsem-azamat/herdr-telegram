# Current state and agent handoff

> Last updated: 2026-07-29. This is an orientation checkpoint, not a replacement for the normative architecture, decisions, threat model, operations guide, or implementation plan. Update it whenever a checkpoint changes what is shipped, blocked, or next.

## Read in this order

1. [`AGENTS.md`](../AGENTS.md) — non-negotiable rules and delivery workflow.
2. [`README.md`](../README.md) — product model, scope, and repository map.
3. This file — what is complete now, what remains blocked, and the immediate next task.
4. [`architecture.md`](architecture.md), [`decisions.md`](decisions.md), and [`threat-model.md`](threat-model.md) — normative identities, state machines, and security boundaries.
5. [`technology.md`](technology.md), [`development.md`](development.md), and [`operations.md`](operations.md) — implementation and runtime constraints.
6. [`implementation-plan.md`](implementation-plan.md) — ordered delivery phases and acceptance evidence.
7. [`references.md`](references.md) — authoritative upstream material; revalidate it when protocol or platform versions change.
8. [`CONTRIBUTING.md`](../CONTRIBUTING.md) and [`SECURITY.md`](../SECURITY.md) before proposing external changes or reporting vulnerabilities.

When this summary conflicts with a normative document, stop and resolve the inconsistency rather than choosing the convenient interpretation.

## Shipped checkpoint

[PR #4](https://github.com/vsem-azamat/herdr-telegram/pull/4) delivered the initial public Go SDK for Herdr v0.7.5 / protocol 17. The repository currently contains no runnable Telegram bridge, daemon, SQLite store, polling loop, or production service.

Public package:

```text
github.com/vsem-azamat/herdr-telegram/herdr
```

Implemented unary methods:

```text
Ping
Snapshot
ListAgents
GetAgent
Prompt
```

The SDK provides one-request-per-connection Unix-socket NDJSON transport, context cancellation, bounded responses, request-ID and result-discriminator validation, protocol-derived types, typed API/transport/protocol errors, and fail-closed `AmbiguousPromptError` handling. Event subscription, reconnect, and replay support are not implemented.

`NewClient` trusts the explicit socket endpoint. The future bridge adapter—not this low-level package—must perform the threat model's path, owner, mode, symlink, descriptor, and peer checks before construction.

## Verified facts and unresolved gates

During the 2026-07-29 handoff audit, `make check` passed. The audit did not run a fresh live Herdr, Telegram, or systemd probe.

The SDK checkpoint was validated against Herdr v0.7.5 / protocol 17. Tagged upstream source remains the protocol reference; re-check current installed and upstream versions before relying on old observations.

Verified in a disposable live probe:

- an explicit pane target receives `agent.prompt` independently of the focused pane;
- Claude and Codex expose native `agent_session` identity in snapshots;
- a detected Pi process did not expose stable native session identity.

No redacted Phase 0 fixture or transcript from that probe is tracked. Treat the observations as orientation only and reproduce evidence needed for an acceptance gate.

Critical unresolved gate:

- protocol 17 `agent.prompt` accepts `target`, `text`, and optional `wait` only;
- it has no atomic `expected_session`, generation, revision, compare-and-send, or idempotency precondition;
- therefore a local `Snapshot → verify occupant → Prompt(pane)` sequence remains TOCTOU-vulnerable;
- automatic Telegram-to-agent prompt routing must remain disabled until a server-side primitive is implemented and race-tested.

Prompt waiting observes Herdr lifecycle state, not completion correlated to the submitted turn. Any non-dial prompt failure is conservatively ambiguous and must not be retried automatically.

The other Phase 0 gate families are also not complete:

- plugin registration plus `systemd --user` lifecycle, including disable/unlink fencing;
- disposable Telegram forum prerequisites and recovery behavior.

Do not start bridge product phases merely because the SDK exists. Phase 0 remains the gate.

## Next task

The default next repository task is the **Phase 0 expected-session contract specification** described in `implementation-plan.md`. Create `docs/spikes/herdr-expected-session.md` on one focused branch and:

1. re-check the latest authoritative Herdr schema and server implementation in a separate read-only upstream checkout;
2. record whether a newer protocol already provides atomic expected-session dispatch;
3. if it does not, specify the smallest backward-compatible request field, mismatch error, atomic server behavior, and occupant-replacement tests;
4. record the upstream issue/proposal URL if the user authorizes publication.

Herdr server implementation belongs in the upstream Herdr repository, not this repository. Do not edit, fork, publish to, or open a PR against upstream without explicit user authorization. This repository owns the bridge specification and should record the resulting protocol status and links.

Do not start SDK event subscriptions, `internal/` bridge packages, plugin/systemd mutation probes, or Telegram prerequisite probes as the default continuation. They are separate later tasks with different acceptance evidence and prerequisites. If the user chooses a different Phase 0 family, update this handoff and keep it to one focused branch/PR.

## First-session checklist

From a clean checkout:

```text
git status --short --branch
git fetch --prune origin
git switch main
git merge --ff-only origin/main
make hooks
make check
go doc ./herdr
```

Then:

1. inspect open PRs/issues and recent `main` history instead of assuming this checkpoint is still current;
2. confirm the next task and its acceptance evidence with the user;
3. create one short-lived branch from current `main`;
4. use TDD: failing test, observed failure, minimal implementation, passing narrow test, then full checks;
5. request specification-compliance review followed by quality/security review;
6. open a PR and wait for CI;
7. never merge, release, configure production credentials, or enable production routing without explicit human approval.

The canonical repository gate is:

```text
make check
```

## Do not depend on prior local artifacts

Previous research used disposable sessions and temporary files outside the repository. They are not part of the handoff and may no longer exist. Do not treat `/tmp` captures, a currently running Herdr session, local credentials, or conversation history as source material. Reproduce required evidence from authoritative documentation and disposable fixtures, then commit only redacted durable artifacts that belong in the repository.

Do not rely on a recorded count of open PRs or issues; inspect GitHub at the start of every session.

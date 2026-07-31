# Current state and agent handoff

> Last updated: 2026-07-31. This is an orientation checkpoint, not a replacement for the normative architecture, decisions, threat model, operations guide, or implementation plan. Update it whenever a checkpoint changes what is shipped, blocked, or next.

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

The SDK provides one-request-per-connection Unix-socket NDJSON transport, context cancellation, bounded responses, request-ID and result-discriminator validation, protocol-derived types, typed API/transport/protocol errors, and fail-closed `AmbiguousPromptError` handling. It also models the optional expected-session prompt field and capability implemented by the temporary personal Herdr fork. Event subscription, reconnect, and replay support are not implemented.

`NewClient` trusts the explicit socket endpoint. The future bridge adapter—not this low-level package—must perform the threat model's path, owner, mode, symlink, descriptor, and peer checks before construction.

## Verified facts and unresolved gates

During the 2026-07-31 lifecycle checkpoint, `make check` and the opt-in race-enabled plugin/systemd probe passed. No Telegram probe or production credential was used.

The SDK checkpoint was validated against Herdr v0.7.5 / protocol 17. The expected-session spike subsequently reviewed upstream `master` at `73d92004f50d3f5fafe64e0f9b7fddbcf4d99965`, which reports protocol 18 but still exposes no atomic expected-session prompt contract. Tagged source remains the SDK protocol reference; re-check current installed and upstream versions before relying on old observations.

Verified in a disposable live probe:

- an explicit pane target receives `agent.prompt` independently of the focused pane;
- Claude and Codex expose native `agent_session` identity in snapshots;
- a detected Pi process did not expose stable native session identity.

No redacted Phase 0 fixture or transcript from that probe is tracked. Treat the observations as orientation only and reproduce evidence needed for an acceptance gate.

Expected-session gate status:

- protocol 17 and the upstream protocol 18 source audited at `73d92004f50d3f5fafe64e0f9b7fddbcf4d99965` expose `agent.prompt` with `target`, `text`, and optional `wait` only;
- a local `Snapshot → verify occupant → Prompt(pane)` sequence remains TOCTOU-vulnerable, including when the native ID is scraped directly from Codex, Claude, or Pi state;
- with explicit approval, personal-fork PR [`vsem-azamat/herdr#1`](https://github.com/vsem-azamat/herdr/pull/1) was independently reviewed, rebased onto upstream `02fe7d76`, validated, and squash-merged to fork `master` at `a8758ff3`;
- the fork passed its full test suite and a side-by-side disposable capability smoke test; `~/.local/bin/herdr-expected-session` now points to the merged build while `/usr/bin/herdr` remains unchanged;
- this repository's SDK can encode the field, but must require the affirmative capability because ordinary/older Herdr may ignore unknown request fields;
- [`spikes/herdr-expected-session-live-probe.md`](spikes/herdr-expected-session-live-probe.md) records the redacted disposable Go-client probe: matching explicit-pane dispatch ignored focus, replacement failed with no rejected input, and a replacement session could not satisfy the accepted session's wait;
- the expected-session compare-and-submit gate is proven for the temporary fork, but ordinary upstream Herdr has not adopted it and lifecycle status remains uncorrelated to a particular submitted turn;
- automatic Telegram-to-agent prompt routing remains disabled pending the plugin/systemd and Telegram Phase 0 gate families and a decision on turn-completion correlation.

Prompt waiting observes Herdr lifecycle state, not completion correlated to the submitted turn. A correlated `agent_session_mismatch` response to an expected-session request is a known rejection; other non-dial prompt failures are conservatively ambiguous and must not be retried automatically.

Plugin/systemd lifecycle status:

- [`spikes/plugin-systemd-lifecycle.md`](spikes/plugin-systemd-lifecycle.md) records a disposable Linux probe of atomic one-shot descriptor publication, a separately installed transient systemd companion, bounded restart policy, sequential disable/unlink denial, Herdr restart refresh, stale descriptor replay rejection, and unsafe-mode rejection;
- the coordinated race also proves that `plugin.list(enabled)` followed by a separate mutation is not a revocation fence: an operation already past the check can commit after `plugin disable` returns;
- the lifecycle family therefore remains blocked pending an accepted atomic enabled/installed precondition or equivalent lifecycle authority.

The disposable Telegram forum prerequisites and recovery family is also incomplete.

Do not start bridge product phases merely because the SDK exists. Phase 0 remains the gate.

## Next decision

The current focused task is the Phase 0 plugin/systemd lifecycle spike. Its feasible descriptor/restart behavior is now recorded, but the deterministic disable race is a release blocker.

The expected-session contract gap is published as [Herdr Discussion #2016](https://github.com/herdrdev/herdr/discussions/2016), but no upstream maintainer has accepted it. The personal fork remains temporary development infrastructure. Do not open another issue or PR against `herdrdev/herdr` without separate explicit owner approval and maintainer alignment.

The next decision is whether to seek Herdr direction on a server-owned atomic plugin-enabled mutation precondition, or explicitly revise the plugin lifecycle authority through a new reviewed decision. Do not invent a registry-lock workaround. The Telegram prerequisite spike is a separate remaining Phase 0 task, and `PromptWait` still does not correlate completion to the submitted turn. Product routing stays disabled.

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

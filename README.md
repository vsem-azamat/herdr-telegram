# herdr-telegram

A clean-slate, Telegram-oriented remote interface for [Herdr](https://herdr.dev/).

> **Status:** architecture and planning only. There is no runnable bridge yet.

## Product model

One Telegram forum topic represents one stable Herdr agent session:

```text
Telegram forum topic
        │ durable binding
        ▼
Herdr native agent session
        │ live reconciliation
        ▼
current pane_id
```

`pane_id` is a runtime address, not conversation identity. Moving or recreating a pane must not create a new topic when the native agent session remains the same. A new native session in the same pane must not inherit the previous topic.

## Why this project exists

Herdr supports multiple independent agents in neighboring panes. A tab-focused bridge cannot know which independent agent should receive a message when focus changes. This project addresses the narrower problem directly:

```text
Telegram topic → exact agent session → current pane
```

It is intentionally not a fork or rewrite of an existing bridge. The repository imports no third-party bridge source or Git history. Public APIs, independently observed behavior, and general reliability patterns are references; this repository's documents and tests are its specification.

## Technology

The daemon is implemented in Go and integrates with Herdr through its process/socket protocol. The planned stack uses standard HTTP, JSON, context, and structured logging packages; `database/sql` with a pure-Go SQLite driver; embedded migrations/assets; and `systemd --user` supervision.

See [Technology](docs/technology.md) for the concrete stack and operating constraints.

## Core invariants

1. A Telegram topic maps to at most one stable Herdr agent session.
2. A stable agent session maps to at most one active topic in the configured forum.
3. Input is sent only after the session resolves to exactly one live pane.
4. The focused pane is never used as a fallback.
5. Release is blocked until Herdr can atomically submit to a pane only when it still hosts the expected native session; a daemon-side precheck alone is not sufficient.
6. Snapshot state is authoritative; Herdr events are replayable hints.
7. Missing, stale, ambiguous, or conflicting routes fail closed.
8. A cross-system timeout can be `ambiguous`; the bridge never promises impossible exactly-once execution.
9. Agents without native `agent_session` identity are diagnostic-only in MVP.
10. Telegram is treated as a remote agent-control boundary, not a casual notification channel.

## MVP scope

Included:

- one Telegram bot;
- one allowlisted forum supergroup;
- one Herdr server/socket;
- Claude and Codex sessions with native session identity;
- automatic topic creation with a circuit breaker;
- explicit session-to-pane reconciliation;
- direct text prompting;
- bounded `/status`, `/recent`, `/esc`, `/enter`, `/key`, `/doctor`, and `/reconcile` commands;
- SQLite state, Telegram update admission, ambiguous-state handling, and operator recovery;
- Linux `systemd --user` lifecycle;
- disposable live integration tests before production use.

Not included:

- tmux or generic multiplexer backends;
- Slack, Discord, Matrix, or transport plugins;
- arbitrary shell execution;
- agents without native session identity;
- automatic agent start/resume when a session is missing;
- terminal-byte mirroring;
- automatic destructive topic or pane lifecycle;
- multiple bots, forums, or Herdr servers;
- web dashboards or Telegram Mini Apps.

## Repository map

| Document | Purpose |
|---|---|
| [Architecture](docs/architecture.md) | Runtime model, identities, state, failure semantics |
| [Decisions](docs/decisions.md) | Why the design takes this shape |
| [Technology](docs/technology.md) | Go runtime, dependencies, tooling, and packaging |
| [Threat model](docs/threat-model.md) | Trust boundaries and required controls |
| [Operations](docs/operations.md) | Plugin/service lifecycle and recovery |
| [Implementation plan](docs/implementation-plan.md) | Ordered delivery phases and acceptance gates |
| [References](docs/references.md) | Authoritative API and design references |
| [Agent handoff](AGENTS.md) | Rules for future coding agents |

## Verified foundation

The design was checked against a live Herdr v0.7.5 / protocol 17 instance. Its snapshot exposes native session records for Claude and Codex:

```json
{
  "pane_id": "w3:p2",
  "agent": "codex",
  "agent_session": {
    "agent": "codex",
    "kind": "id",
    "source": "herdr:codex",
    "value": "..."
  }
}
```

A detected Pi agent currently has no `agent_session`, confirming that process detection and stable conversation identity are separate capabilities.

## Implementation posture

No product code should be added until all three Phase 0 contract families pass:

- atomic expected-session behavior for explicit `agent.prompt.target`, including occupant replacement races;
- exact `systemd --user` and Herdr plugin disable/unlink lifecycle;
- Telegram forum prerequisites and operator recovery;

Implementation should proceed TDD-first, one phase at a time, with separate specification and quality reviews. Agents must not publish, merge, or enable the bridge against production Telegram without explicit human approval.

## License

MIT. Because the repository is still local and unpublished, the owner can change this before public publication if a different policy is preferred.

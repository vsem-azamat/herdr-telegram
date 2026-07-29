# Design decisions

This is a compact decision log. It records current choices, reasons, rejected alternatives, and conditions for revision.

## D-001 — Telegram-oriented, not provider-neutral

**Decision:** Support Telegram forum topics only.

**Reason:** Provider-neutral abstractions would force the domain to model the least common denominator before the first real workflow is proven. Telegram-specific behavior—forum topics, update offsets, bot privacy, `message_thread_id`, `429.retry_after`, and ambiguous topic creation—is central rather than incidental.

**Rejected:** A generic `ChatBackend` for Telegram, Slack, Discord, and Matrix.

**Revisit when:** A second transport has a committed user and a concrete end-to-end design.

## D-002 — Native agent session is canonical

**Decision:** Durable identity is:

```text
herdr_instance_id
+ agent_session.source
+ agent_session.agent
+ agent_session.kind
+ agent_session.value
```

**Reason:** `pane_id` changes when a running pane moves across workspaces and may change on recreation. A Telegram topic represents a conversation, not a screen rectangle.

**Rejected:** topic→tab, topic→focused pane, and durable topic→pane mappings.

## D-003 — `pane_id` is an ephemeral live route

**Decision:** Resolve stable session identity to the current explicit pane, but ship prompting only when Herdr can atomically condition dispatch on that expected session.

**Reason:** Focus is mutable global UI state. Persisted pane routing can become stale and leak one topic's prompt to another session.

**Consequence:** Missing, duplicate, or mismatched live routes fail closed. A daemon-side read followed by an unconditional pane prompt is insufficient because occupant replacement can occur between the two calls.

## D-004 — Go for MVP

**Decision:** Use Go with standard HTTP/JSON/context/logging packages, `database/sql`, a reviewed pure-Go SQLite driver, and embedded migrations/assets.

**Reason:** The long-lived daemon ships as one executable with direct systemd deployment, typed state machines, explicit cancellation, race testing, and simple release/rollback semantics.

See `technology.md`.

## D-005 — Snapshot authoritative, events advisory

**Decision:** `session.snapshot` defines current topology. Event subscriptions reduce latency but never independently establish durable truth.

**Reason:** Herdr currently replays retained events to new subscribers, event ordering is not a durable cross-restart oracle, and revisions are scoped to resource/server lifetime.

**Bootstrap:** subscribe and buffer → snapshot → install state → apply buffered hints → reconcile if uncertain.

## D-006 — SQLite without ORM

**Decision:** Store bindings, inbox states, turns, outbox items, checkpoints, creation attempts, and daemon fencing in SQLite WAL mode.

**Reason:** One local process needs transactional recovery, uniqueness, inspectability, and no additional service.

**Rejected:** JSON files, Redis, PostgreSQL, SQLAlchemy, and a generic repository framework.

**Consequence:** SQL migrations and transaction boundaries are explicit and tested.

## D-007 — No exactly-once claim

**Decision:** Promise durable admission and fail-closed ambiguous recovery, not exactly-once Herdr execution or Telegram delivery.

**Reason:** No transaction spans SQLite, Herdr, and Telegram. Neither `agent.prompt` nor `sendMessage`/`createForumTopic` accepts a downstream idempotency key.

**Consequence:** A timeout after possible acceptance becomes `ambiguous`; automatic retry is forbidden.

## D-008 — One turn per topic

**Decision:** Serialize Telegram updates per topic and permit one in-flight agent turn in MVP.

**Reason:** Concurrent prompt injection makes output boundaries and recovery much harder and is not required for the personal workflow.

## D-009 — Existing stable sessions only

**Decision:** Automatically map only live supported agents with a native `agent_session`.

**Reason:** Agent process detection alone is not stable conversation identity. Auto-start/resume would add a second product—agent orchestration—before routing is trusted.

**Rejected:** Pane-scoped fallback and silently creating replacement sessions.

## D-010 — Automatic topic creation is bounded

**Decision:** Eligible new sessions may create topics, capped at ten creations per reconciliation/startup.

**Reason:** Identity drift or malformed snapshots must not create a topic storm.

**Consequence:** Exceeding the cap opens a circuit breaker and requires operator attention.

## D-011 — Ambiguous topic creation is manual recovery

**Decision:** If `createForumTopic` may have succeeded but no response was durably recorded, do not retry.

**Reason:** Telegram provides no caller idempotency key and no reliable API to list/search all forum topics.

**Recovery:** `/bind-pending <attempt-id>` from the visible topic, or confirmed `/retry-create <attempt-id>` if no topic exists.

## D-012 — Raw Telegram Bot API adapter

**Decision:** Implement the small required method set directly with Go's `net/http` and `encoding/json`.

**Reason:** Poll offset, forum identity, malformed responses, webhooks, competing pollers, and ambiguous timeouts are core semantics. A bot framework would hide the boundaries we need to test.

## D-013 — Linux systemd user service

**Decision:** Linux MVP uses `systemd --user`. Herdr's one-shot startup hook only refreshes runtime registration.

**Reason:** Plugin startup hooks are explicitly not daemon supervision. Detached self-managed processes complicate crash recovery and disable/uninstall behavior.

**Consequence:** The daemon package must be installed independently of Herdr's managed plugin checkout. Before every mutation, it confirms the plugin remains installed and enabled.

## D-014 — No automatic destructive lifecycle

**Decision:** MVP does not close panes, terminate agents, delete topics, or execute arbitrary shell commands.

**Reason:** Telegram credentials provide remote agent control. Destructive scope should be added only after routing and recovery have survived live soak testing.

## D-015 — Clean-slate implementation

**Decision:** Import no third-party bridge source or history.

**Reason:** We want an architecture governed by this project's constraints and under our control. Existing projects are evidence and inspiration, not a codebase to rename.

**Required practice:** Future contributors must be able to explain each implementation from this repository's spec and authoritative API documentation.

## D-016 — MIT license for the clean-slate repository

**Decision:** Use MIT for the initial repository.

**Reason:** The project is a small integration intended for broad reuse, carries no imported implementation, and benefits from a simple permissive license. This can still be changed before the repository is published.

## D-017 — Stable names

**Decision:** Use repository/package names `herdr-telegram` / `herdr_telegram`, executable `herdr-telegram`, and Herdr plugin ID `io.github.vsem-azamat.herdr-telegram`.

**Reason:** These names are explicit, searchable, and avoid a globally ambiguous short plugin ID. The reverse-GitHub namespace matches the current owner.

## D-018 — Build the protocol SDK before bridge routing

**Decision:** Deliver a small public Go SDK for Herdr protocol 17 as an intermediate checkpoint before Telegram bridge product phases.

**Reason:** Socket transport, envelopes, snapshots, agent records, and typed errors are independently useful and testable. They do not depend on proving the bridge's stable-session routing policy.

**Boundary:** The SDK mirrors low-level `agent.prompt(target, text, wait?)` exactly and documents that protocol 17 has no atomic expected-session precondition. It must not expose a method whose name or contract implies session-safe dispatch. Telegram routing remains blocked until the stronger server-side guarantee exists.

# Design decisions

This is a compact decision log. It records current choices, reasons, rejected alternatives, and conditions for revision.

## D-001 — Telegram-oriented, not provider-neutral

**Decision:** Support Telegram forum topics inside one allowlisted private bot chat only.

**Reason:** Current Bot API supports bot topic mode in private chats, matching the intended one-user UX without a supergroup. Provider-neutral abstractions would force the domain to model the least common denominator before the first real workflow is proven. Telegram-specific behavior—private topic mode, update offsets, `message_thread_id`, `429.retry_after`, and ambiguous topic creation—is central rather than incidental.

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

**Reason:** The project is a small integration intended for broad reuse, carries no imported implementation, and benefits from a simple permissive license. The repository is now public; changing the license requires an explicit new decision.

## D-017 — Stable names

**Decision:** Use repository/package names `herdr-telegram` / `herdr_telegram`, executable `herdr-telegram`, and Herdr plugin ID `io.github.vsem-azamat.herdr-telegram`.

**Reason:** These names are explicit, searchable, and avoid a globally ambiguous short plugin ID. The reverse-GitHub namespace matches the current owner.

## D-018 — Build the protocol SDK before bridge routing

**Decision:** Deliver a small public Go SDK for Herdr protocol 17 as an intermediate checkpoint before Telegram bridge product phases.

**Reason:** Socket transport, envelopes, snapshots, agent records, and typed errors are independently useful and testable. They do not depend on proving the bridge's stable-session routing policy.

**Boundary:** The SDK mirrors low-level `agent.prompt` fields exactly. For legacy protocol 17 this means `target`, `text`, and optional `wait`; it may also encode the capability-gated `expected_session` extension implemented by a Herdr server. The SDK must not claim session-safe dispatch unless the caller first observes the affirmative capability. Telegram routing remains blocked until the stronger server-side guarantee is proven and the remaining Phase 0 gates pass.

## D-019 — Temporary capability-gated Herdr fork compatibility

**Decision:** During development, support the optional `agent.prompt.expected_session` contract implemented in the personal `vsem-azamat/herdr` fork. Treat the fork as a temporary compatibility dependency, not as a permanent product backend or evidence of upstream support.

**Reason:** Reading a native Codex, Claude, or Pi session ID from a pane does not make dispatch atomic. The occupant can change between the read and an unconditional pane prompt. Provider session IDs normally identify resumable history, not a provider-supported input endpoint. Scraping provider files or process state would add fragile provider-specific code without closing the race.

**Safety boundary:** Automatic prompting must require `ping.capabilities.agent_prompt_expected_session == true`, pass all four native session fields, and fail closed otherwise. A protocol number, local precheck, or recognized request shape is insufficient because older servers may ignore unknown fields.

**Migration:** When ordinary upstream Herdr ships an equivalent accepted contract, remove the fork installation requirement and validate its advertised wire behavior. Keep the stable-session routing model; adapt only the capability/wire compatibility layer if upstream chooses a different shape.

**Rejected:** Pane read followed by unconditional prompt, direct provider transcript/session scraping, and silently assuming every protocol-18 server implements the fork extension.

## D-020 — Plugin enabled-state checks are not a revocation fence

**Decision:** Keep plugin disable/unlink as a mandatory mutation fence, but do not treat a daemon-side `plugin.list` check, registry-file read, filesystem watch, descriptor refresh, or service stop signal as proof of atomic revocation.

**Reason:** The disposable lifecycle spike deterministically paused a companion after it observed the plugin enabled, disabled the plugin to completion, and then allowed the already-authorized modeled mutation to commit. This is the same read-then-mutate TOCTOU shape rejected for session routing.

**Required contract:** Before product mutation routing, one authority must linearize plugin enabled/installed state with mutation acceptance. A possible server-owned precondition needs separate Herdr direction and protocol review; it is not approved merely by naming it here. Holding Herdr's private plugin-registry lock around a socket request is rejected because the lock/format is not public and can deadlock against the server actor processing both operations.

**Consequence:** Sequential post-disable and post-unlink requests fail closed in the spike, but the Phase 0 lifecycle family remains blocked. Automatic Telegram-to-agent routing stays disabled.

## D-021 — Private bot topic mode replaces the supergroup

**Decision:** Bind sessions to topics in one configured private chat with the bot. Do not require or support a forum supergroup in MVP.

**Reason:** Bot API 10.2 documents `User.has_topics_enabled`, `User.allows_users_to_create_topics`, `message_thread_id` for private bot chats, and `createForumTopic`/`editForumTopic` for a private chat with a user. A redacted read-only probe of the newly provisioned development bot returned topic mode enabled, user topic creation enabled, no webhook, and the configured chat as `type=private`.

**Admission contract:** Require the exact configured private `chat.id`, exact allowlisted `from.id`, `from.is_bot == false`, no `sender_chat`, `is_topic_message == true`, a valid nonzero `message_thread_id`, and no `business_connection_id` or `guest_query_id`. The latter fields select alternate Telegram namespaces even when numeric identities overlap and therefore fail closed. The configured chat and user IDs may be numerically equal but are validated as separate envelope fields. Unknown or unbound topics fail closed.

**Prerequisites:** Require `getMe.has_topics_enabled == true`, exact bot identity, `getChat.type == private`, exact chat identity, and an empty webhook. Group privacy mode, `getChatMember`, administrator status, and `can_manage_topics` are not private-chat prerequisites.

**Consequence:** Existing supergroup-specific requirements and fixtures are obsolete and must be replaced before Telegram adapter work. The read-only probe is not disposable mutation/recovery evidence; topic creation, ambiguous outcomes, polling conflicts, and cleanup remain Phase 0 gates.

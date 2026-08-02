# Architecture

> This is the target bridge architecture. See `current-state.md` for what is implemented and currently blocked.

## 1. Context

Herdr can host several independent coding-agent sessions in adjacent panes. A Telegram bridge that binds a topic to a tab and forwards input to the tab's focused pane cannot preserve the intended recipient when focus changes.

This project models the conversation explicitly:

```text
Telegram topic → stable Herdr agent session → current pane
```

The bridge is a Herdr-native plugin companion, but not an in-process extension. Herdr plugin v1 launches ordinary executable commands and exposes its stable CLI/socket API.

## 2. System boundary

MVP supports exactly:

```text
one bot
one allowlisted private chat with bot topic mode
one Herdr server
one local daemon
one SQLite database
```

The design does not contain generic chat or multiplexer backends.

## 3. Identities

### TopicKey

```text
bot_instance_id
+ telegram_chat_id
+ normalized_topic_id
```

Private bot chats with topic mode use `message_thread_id` for their forum topics even though `getChat.type` is `private` and `getChat.is_forum` is not the capability signal. Topic identity must be normalized deliberately; omitted, `null`, and zero-like values must not become separate destinations accidentally. Routed input requires `chat.type == private`, the exact configured chat/user identity, `is_topic_message == true`, a valid nonzero thread ID, and absence of `business_connection_id` and `guest_query_id` in MVP.

### AgentSessionKey

```text
herdr_instance_id
+ agent_session.source
+ agent_session.agent
+ agent_session.kind
+ agent_session.value
```

The configured Herdr instance ID is persisted on first initialization. A later mismatch is fatal until explicit migration/reset.

### LiveRoute

```text
AgentSessionKey
+ pane_id
+ workspace_id
+ tab_id
+ route_generation
+ current server epoch
```

Live routes are in-memory snapshot projections. They are rebuilt and resolved before a mutation request, but this projection is not a dispatch safety guarantee; the server-side operation must still enforce the expected session atomically.

### Durable TopicBinding

```text
TopicKey ↔ AgentSessionKey
binding_state = active | retired | externally_missing
```

Route availability is derived separately:

```text
live | offline | conflict
```

A session becoming temporarily absent does not retire its binding. A new session appearing in its previous pane does not inherit or retire the old topic.

## 4. Runtime components

```text
┌──────────────────────────────────────────────────────┐
│ herdr-telegram daemon                                │
│                                                      │
│ Telegram ingress ──► inbox/turn coordinator          │
│       ▲                         │                    │
│       │                         ▼                    │
│ Telegram outbox ◄──── application services           │
│                                 │                    │
│ Herdr event buffer ─► serialized reconciler          │
│ Herdr snapshot ─────► authoritative route projection │
│                                 │                    │
│ SQLite ◄──────── durable bindings and recovery       │
└──────────────────────────────────────────────────────┘
```

### Telegram adapter

Implements only required Bot API methods and exposes explicit result classes:

- success;
- rejected `ok:false`;
- retryable 5xx;
- rate limited with `retry_after`;
- competing poller `409`;
- malformed response;
- ambiguous network timeout.

### Herdr adapter

The public `herdr` package owns protocol framing, unary envelopes, protocol-derived JSON types, bounded transport, and typed errors. The internal bridge adapter composes `herdr.Client` and owns bridge policy. It:

- subscribes to events through the public SDK;
- requests snapshots through the public SDK;
- validates supported native session identity semantics;
- resolves exact pane targets;
- prompts and sends allowlisted keys;
- reads bounded output;
- enforces endpoint trust checks and understands protocol/version failures.

### Serialized reconciler

All topology changes pass through one worker. It consumes authoritative snapshots plus event hints and produces an immutable current projection. It quarantines duplicate session claims and increments route generation on meaningful route replacement.

### Per-topic turn coordinator

One topic has at most one in-flight turn. Later Telegram updates are durably ordered. The coordinator owns the inbox state machine and validates route/fencing generation immediately before Herdr mutation.

### SQLite state

Minimum durable tables:

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

No ORM. Live routes and replay fingerprints stay in memory unless tests prove persistence necessary.

## 5. Invariants

1. One topic maps to at most one stable agent session.
2. One stable session maps to at most one active topic in the configured private bot chat.
3. No input is sent without exactly one validated live route.
4. Focus is never a routing input or fallback.
5. Prompt release requires a server-side atomic expected-session condition; daemon-side read-then-prompt revalidation alone cannot close the occupant-replacement race.
6. Only the current daemon fencing generation may produce side effects.
7. Snapshot reconstructs current topology without event history.
8. Events are hints and may replay.
9. One turn executes per topic.
10. Duplicate Telegram updates are rejected at durable admission.
11. Ambiguous downstream acceptance is never blindly retried.
12. Agents without native session identity receive no automatic topic.
13. Destructive lifecycle and arbitrary shell execution are absent from MVP.
14. Topic creation is bounded by eligibility and a circuit breaker.
15. Plugin disabled/unlinked state prevents every remote mutation.

## 6. Bootstrap and event ordering

Snapshot-before-subscribe has a gap. Subscribe-after-snapshot may miss a change, and retained-event replay does not provide a reliable cut-over cursor.

Required bootstrap:

```text
open subscription
buffer events
request fresh snapshot
install snapshot projection atomically
apply buffered events as hints
reconcile again when uncertain
continue stream + periodic snapshot
```

`revision` and `state_change_seq` are used only within their documented resource and current Herdr server lifetime. They are not persisted as cross-restart clocks.

## 7. Telegram inbox semantics

`getUpdates` confirms earlier updates when a later request uses a greater offset. Therefore:

1. store each update before handling;
2. process in ascending order;
3. classify the envelope as admitted, rejected, or ignored;
4. for admitted mutations, atomically create the durable turn and mark the inbox row acknowledgeable;
5. mark rejected/ignored rows acknowledgeable after their durable classification;
6. persist and advance `next_offset` only through the highest contiguous acknowledgeable row;
7. do not poll past an earlier update until its admission row is durably acknowledgeable; downstream turn resolution is separate.

Inbox admission states:

```text
received
→ admitted | rejected | ignored
```

Telegram acknowledgment is independent from eventual Herdr execution. For an admitted mutating update, one SQLite transaction creates the durable `turns` row and marks the inbox row acknowledgeable. The contiguous `next_offset` may then advance even while that turn is queued, waiting, or ambiguous. Otherwise one ambiguous turn would stop polling for every topic indefinitely.

An admitted inbox row is immutable evidence of input receipt; operator recovery changes the associated turn, not the Telegram update. Exactly-once remote execution is impossible with current APIs.

### Turn state machine

```text
queued
  → dispatching
      → submitted → waiting → done | blocked
      → rejected
      → ambiguous

waiting
  → waiting_timed_out
  → waiting_offline
  → done | blocked
```

- `dispatching` is the narrow cross-system uncertainty window. A restart while in this state converts it to `ambiguous`; it is never resubmitted automatically.
- `submitted` means Herdr returned known acceptance. The bridge should use the single `agent.prompt(..., wait=...)` operation so submit and initial semantic wait are server-owned rather than separate client calls.
- `waiting` remains the one active turn until Herdr reports `done` or `blocked` for the same stable session.
- `waiting_timed_out` and `waiting_offline` are nonterminal holds. They do not release the next queued turn because the agent may still be executing.
- `done`, `blocked`, `rejected`, and explicit local-operator resolution release the next queued turn. `blocked` releases it because the next Telegram message may be the requested answer.
- `ambiguous` requires local operator resolution as delivered, dropped, or intentionally requeued as a new turn. It never mutates the original inbox row.

After restart, `submitted`/`waiting*` turns resume observation only if the same `AgentSessionKey` is present. A later status from a replacement session cannot complete them. If upstream cannot tie completion evidence to the same resolved occupant/session, that limitation remains a release blocker alongside atomic dispatch.

### Telegram outbox state machine

```text
pending → sending → sent
                  → failed_permanent
                  → retryable_known_failure → pending
                  → ambiguous
                       → resolved_delivered
                       → resolved_dropped
                       → resolved_retry_as_new ──creates──► new pending row
```

Only a failure known to have occurred before Telegram acceptance may retry automatically. Each known-safe retry increments the row's attempt counter and preserves the classified failure metadata; bounded retry exhaustion becomes `failed_permanent`.

`ambiguous` delivery never retries automatically. Local operator resolution records one immutable terminal resolution on the original row. `retry-as-new` creates a separate `pending` row with `retry_of_outbox_id` pointing to the ambiguous row, so the possible duplicate and the human decision remain auditable.

## 8. Prompt routing

For a normal topic message:

1. validate the complete private-chat Telegram envelope, exact chat/user allowlists, and topic identity;
2. admit/deduplicate the update;
3. resolve `TopicBinding`;
4. resolve one current `LiveRoute`;
5. verify plugin enabled state and daemon fence; this preflight is necessary but cannot by itself provide revocation atomicity;
6. persist `dispatching` with the expected `AgentSessionKey`, pane, route generation, and daemon generation;
7. issue one explicit pane-targeted `agent.prompt` request that carries the expected-session precondition atomically and requests semantic wait; product release additionally requires plugin enabled/installed state to be linearized with mutation acceptance rather than checked in a separate call;
8. persist known result or `ambiguous`;
9. enqueue Telegram response in the durable outbox.

The implementation must prove explicit `pane_id` behavior and occupant-replacement race semantics with a disposable capability-advertising server. During development that server may be the temporary personal Herdr fork, but the dependency must remain explicit and capability-gated until ordinary upstream Herdr ships an equivalent contract. If the server cannot atomically reject a target whose native session differs from the expected `AgentSessionKey`, normal text prompting must remain disabled. A local precheck does not satisfy the invariant.

## 9. Topic lifecycle

### Eligible discovery

A new topic may be created only when:

- agent kind is supported;
- native session identity exists and parses;
- exactly one live route claims it;
- no binding exists;
- no pending/ambiguous creation exists;
- creation circuit breaker is closed.

Default cap: ten creations per reconciliation/startup.

### Ambiguous creation

`createForumTopic` has no idempotency key and the Bot API cannot reliably enumerate all forum topics. If the response is lost after Telegram may have committed, the attempt becomes `ambiguous` and no automatic retry occurs.

Operator recovery:

- `/bind-pending <attempt-id>` inside the visible topic;
- confirmed `/retry-create <attempt-id>` when absence is verified.

### External changes

When Telegram reports a topic closed/missing, mark binding `externally_missing`, stop automatic sends, and require confirmed recovery. Topic rename is presentation-only; internal identity never depends on the title.

## 10. Outbound delivery

Telegram responses are written to an outbox before send. Delivery is at-least-once, not exactly-once: a timeout after Telegram accepts `sendMessage` can leave uncertainty. Replayed Herdr status alone never emits a second lifecycle notice; checkpoints are scoped to server epoch, stable session, route generation, and turn.

## 11. Plugin and daemon lifecycle

Herdr `[[startup]]` hooks are one-shot. Linux MVP uses `systemd --user` for supervision.

- companion daemon package installed independently;
- plugin startup atomically refreshes a restrictive, versioned runtime descriptor carrying stable configured instance identity and ephemeral server process identity;
- systemd restarts daemon on failure with bounded restart policy;
- daemon waits for a valid same-UID descriptor/socket and rejects stale process identity;
- each mutation confirms plugin is still installed and enabled;
- disable/unlink must linearize revocation with mutation acceptance even if the service remains alive; the disposable spike proves that a separate `plugin.list` check is TOCTOU-vulnerable, so this remains blocked;
- purge is separate from uninstall.

See `operations.md`.

## 12. Output policy

Terminal content is not a structured assistant-message stream. It may include redraws, prompts, stale scrollback, and tool output. MVP sends:

- explicit command responses;
- bounded `/recent` output;
- conservative blocked/done lifecycle notices;
- no raw terminal mirroring;
- no automatic “assistant answer extraction” until agent-specific adapters have separate acceptance tests.

## 13. Failure principles

- **Telegram unavailable:** bounded backoff; no hidden delayed agent prompts.
- **Herdr unavailable:** routes offline; input gets explicit terminal unavailable outcome.
- **Daemon restart:** fence, subscribe/buffer, snapshot, classify uncertain turns, drain safe outbox.
- **Duplicate session routes:** quarantine and operator-visible conflict.
- **Ambiguous prompt:** inspect and report; never auto-resend.
- **Ambiguous topic creation:** operator bind/retry only.
- **Competing poller:** terminal health fault.
- **Plugin disabled:** no remote mutations.

## 14. Extension boundaries

Potential later work must not pre-shape MVP abstractions:

- more agents with native identity;
- multiple Herdr instances under one control plane;
- automatic session resume;
- richer lifecycle notifications;
- macOS/Windows service packaging;
- additional chat providers.

Each requires a new decision and evidence, not a speculative interface now.

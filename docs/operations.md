# Operations design

> This describes the intended MVP lifecycle. Commands and units are specifications until implementation lands.

## Process model

```text
Herdr plugin manifest
    └── one-shot startup: register runtime descriptor

systemd --user
    └── herdr-telegram daemon
            ├── Telegram long poller
            ├── Herdr socket subscriber
            ├── serialized reconciler
            └── SQLite state
```

Herdr startup hooks are not supervision. They only publish the current `HERDR_SOCKET_PATH`, `HERDR_BIN_PATH`, plugin config directory, plugin state directory, Herdr version/protocol, and stable configured instance ID into a restrictive runtime descriptor.

## Packaging boundary

The daemon executable must be installed independently of Herdr's GitHub-managed plugin checkout, normally from a checksummed release binary or a local `go build`. This prevents plugin reinstall/uninstall from removing code under a running process.

The Herdr plugin contains:

- `herdr-plugin.toml`;
- one-shot runtime-registration entrypoint;
- `doctor`, `status`, and `reconcile` actions;
- metadata identifying the companion daemon version range.

The independently installed package contains:

- daemon and CLI;
- systemd unit template/install command;
- migrations;
- Bot API and Herdr adapters.

## Lifecycle matrix

| Event | Required behavior |
|---|---|
| Plugin linked | Validate manifest; no daemon is silently detached |
| Herdr starts/restores | Startup hook atomically refreshes runtime descriptor and exits |
| systemd service starts | Validate config/state, plugin enabled state, descriptor, Herdr socket, Telegram prerequisites |
| Daemon crashes | systemd restarts with bounded backoff |
| Herdr restarts/handoffs | Subscription reconnects; fresh descriptor/snapshot wins; routes rebuild |
| Plugin disabled | Every mutation fails closed on enabled-state check; health reports disabled |
| Plugin unlinked/uninstalled | Same fail-closed behavior; operator may then stop/remove service |
| User logout | User service stops unless linger is explicitly configured |
| Machine reboot | User service starts according to enable/linger policy; waits safely for valid Herdr registration |
| Package update | Stop service, migrate/validate, restart, reconcile; never retry ambiguous turns |
| Service removal | Stop/disable unit; retain state unless explicit purge is confirmed |

## Startup sequence

1. Open SQLite with restrictive permissions.
2. Apply transactional forward-only migrations.
3. Acquire singleton lock and durable fencing generation.
4. Load and validate persisted Herdr instance identity.
5. Verify plugin exists and is enabled.
6. Validate runtime descriptor and same-user Herdr socket.
7. Validate Telegram with `getMe`, `getChat`, `getChatMember`, and `getWebhookInfo`.
8. Refuse active webhook, missing forum support, missing `can_manage_topics`, insecure config, protocol incompatibility, or absent `agent_prompt_expected_session` capability when prompt routing is enabled.
9. Open Herdr event subscription and buffer events.
10. Fetch `session.snapshot`.
11. Atomically install snapshot-derived route state through one reconciler.
12. Apply buffered events as hints; reconcile again if uncertain.
13. Classify prior `dispatching` turns as `ambiguous`; do not retry.
14. Resume observation of `submitted`/`waiting*` turns only for the same stable session; hold timed-out/offline turns.
15. Drain known-safe Telegram outbox items in per-topic order.
16. Start `getUpdates` from the durable next offset.

## Telegram polling

- one long poller per bot token;
- explicit `allowed_updates`;
- ascending sequential handling;
- next offset advances only through contiguous durably acknowledgeable inbox rows;
- once an admitted update and its turn are committed together, eventual turn outcome—including `ambiguous`—does not block Telegram acknowledgment;
- `409` is terminal health failure;
- `429` honors `retry_after`;
- 5xx uses bounded exponential backoff with jitter;
- malformed JSON and `ok:false` are classified explicitly;
- network loss after possible mutation acceptance becomes `ambiguous`.

## Reconciliation triggers

- daemon startup;
- Herdr reconnect;
- buffered-event bootstrap completion;
- pane/session lifecycle hints;
- route mismatch before mutation;
- periodic timer;
- operator `/reconcile`;
- suspicious event ordering or duplicate session identity.

All reconciliation passes through one serialized worker. Snapshot is authoritative.

## Health states

Suggested top-level states:

```text
starting
healthy
degraded_telegram
degraded_herdr
plugin_disabled
conflict
ambiguous_operator_action
fatal_configuration
fatal_competing_poller
```

`/doctor` and local CLI should report:

- daemon/service version;
- Herdr version/protocol/instance ID;
- plugin installed/enabled state;
- Telegram bot and forum identity;
- last successful poll and snapshot;
- next update offset;
- active/offline/conflicted routes;
- ambiguous inbox/turn/topic-creation counts;
- outbox backlog;
- circuit-breaker state;
- no bot token or raw native session IDs.

## Recovery procedures

### Ambiguous prompt dispatch

1. Do not resubmit automatically.
2. Mark the turn `ambiguous`; keep its Telegram inbox row durably admitted/acknowledged.
3. Inspect current session status and bounded recent output.
4. Report evidence in `/doctor`.
5. A local operator resolves with `herdr-telegram admin turn resolve ...` as delivered, dropped, or requeued as a new auditable turn.

### Ambiguous topic creation

If the topic visibly exists, enter it and send:

```text
/bind-pending <attempt-id>
```

If it does not exist, explicitly confirm:

```text
/retry-create <attempt-id>
```

No background retry is allowed.

### Ambiguous Telegram outbox delivery

1. Do not retry automatically.
2. Inspect the topic and local delivery evidence.
3. Resolve locally with `herdr-telegram admin outbox resolve ...` as delivered, dropped, or retry-as-new.
4. Retrying creates a new outbox row and preserves the ambiguous historical row.

Known failures that provably occurred before Telegram acceptance may return the same row to `pending` with an incremented attempt counter. Automatic retries are bounded; exhaustion becomes `failed_permanent`. A timeout or transport failure after request transmission is not known-safe and therefore becomes `ambiguous`, not retryable.

### Topic closed/deleted externally

1. Mark binding `externally_missing` after Telegram reports it.
2. Stop notifications and routed input for that binding.
3. Require confirmed rebind/recreate.
4. Never silently create a replacement.

### Herdr unavailable

- keep polling only while input can receive a terminal unavailable outcome without advancing unresolved work incorrectly;
- mark routes offline;
- do not queue hidden prompts for later automatic execution;
- reconnect, subscribe/buffer, snapshot, and rebuild.

### Bot-token rotation

1. Stop service.
2. Replace the restrictive token file.
3. Verify old webhook/poller state as possible.
4. Start service and run `/doctor`.
5. Preserve SQLite bindings and next offset only when bot identity is unchanged; a different bot requires explicit instance migration.

## Backup and purge

Back up:

- configuration without copying token into ordinary archives;
- SQLite database using an SQLite-safe backup/checkpoint procedure;
- persisted instance identity and local display-hash salt.

Do not treat live routes as durable backup data.

Purge must be a separate confirmed operation. Plugin uninstall alone should not delete configuration, mappings, audit metadata, or ambiguous-recovery evidence.

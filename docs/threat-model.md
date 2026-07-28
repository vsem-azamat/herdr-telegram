# Threat model

## Security statement

Telegram access to this bridge is effectively remote control over local coding agents. A prompt can cause an agent to read files, change code, run tools, use credentials available to that agent, or approve follow-on actions. The bridge is not merely a notification bot.

## Assets

- source code and local files reachable by Herdr agents;
- agent session transcripts and terminal output;
- SSH, cloud, GitHub, package-manager, and other user credentials;
- Telegram bot token;
- durable topic/session mapping;
- integrity of prompt routing;
- availability of Herdr and Telegram control paths;
- audit evidence needed to understand an incident.

## Trust boundaries

```text
Telegram user/account
        │ Internet + Telegram servers
        ▼
Telegram Bot API
        │ HTTPS
        ▼
local bridge daemon
        ├── config/token files
        ├── SQLite state
        └── Herdr Unix socket
                ▼
        terminal agents and user environment
```

Herdr plugins run as the local user and are not sandboxed. Local compromise is outside the bridge's ability to contain, but the bridge must not widen access accidentally.

## Threats and controls

### Unauthorized Telegram sender

**Threat:** A user, bot, channel, anonymous administrator, forwarded identity, or unexpected update shape triggers an agent action.

**Controls:**

- exact allowlist of one `chat_id` and explicit user IDs;
- require `chat.type == supergroup`;
- require routed messages to be topic messages with normalized `message_thread_id`;
- require `from`, reject `from.is_bot`, reject `sender_chat`;
- reject edited messages, channel posts, business updates, anonymous admins, and automatic forwards;
- configure an explicit `allowed_updates` set;
- durable audit outcome for every admitted/ignored update.

### Bot token compromise

**Threat:** An attacker impersonates the bot, consumes updates, or controls responses.

**Controls:**

- token stored outside repository in a mode-restricted file;
- verify owner, file type, and non-symlink status where practical;
- never log Bot API URLs containing the token;
- redact exceptions/traces;
- detect webhook configuration and competing poller `409`;
- documented token rotation and state-preserving recovery.

**Residual risk:** Bot-token compromise remains severe. Rotation cannot undo prompts already delivered.

### Telegram account compromise

**Threat:** An attacker controls an allowlisted user's account.

**Controls:**

- narrow command surface;
- no arbitrary shell;
- no destructive lifecycle in MVP;
- rate and length limits;
- require the atomic expected-session dispatch condition for every prompt mutation;
- clear local audit metadata.

**Residual risk:** Allowlisted account compromise still permits agent prompting. Telegram 2FA and account security are external requirements.

### Stale or wrong pane routing

**Threat:** A topic's prompt reaches a different agent session.

**Controls:**

- stable `AgentSessionKey` is canonical;
- `pane_id` is only an in-memory route;
- snapshot reconciliation;
- server-side atomic expected-session condition at dispatch; a local read-then-prompt check is explicitly insufficient;
- route and daemon fencing generations;
- no focused-pane or agent-name fallback;
- duplicate live routes quarantine the binding.

### Replay and duplicate execution

**Threat:** Duplicate Telegram updates, retained Herdr events, retries, or daemon restarts submit a prompt twice.

**Controls:**

- unique `(bot_instance_id, update_id)` admission;
- sequential contiguous offset advancement;
- one in-flight turn per topic;
- inbox state machine;
- retained events treated as hints;
- `ambiguous` state after possible downstream acceptance;
- no blind retry of ambiguous prompt dispatch.

**Residual risk:** Exactly-once execution cannot be guaranteed across SQLite and Herdr.

### Duplicate or wrong topic creation

**Threat:** A timeout after Telegram creates a topic causes automatic duplicate creation or wrong binding.

**Controls:**

- durable creation attempt before request;
- no automatic retry after ambiguous timeout/crash;
- creation circuit breaker;
- operator recovery from inside the visible topic;
- confirmed retry only when the operator verifies absence.

### Competing daemon or stale process

**Threat:** Two daemon instances prompt agents or send notifications.

**Controls:**

- singleton lock;
- durable fencing generation;
- generation check immediately before side effects;
- Telegram `409` is terminal health failure;
- systemd owns process supervision.

### Local path substitution

**Threat:** Symlink/socket replacement redirects state or Herdr commands.

**Controls:**

- same-UID owner checks;
- regular-file/directory checks;
- mode restrictions;
- non-symlink checks where practical;
- same-user Unix socket validation;
- create SQLite and lock files with restrictive umask/modes.

### Sensitive output disclosure

**Threat:** `/recent`, logs, topic names, or errors expose secrets/session IDs.

**Controls:**

- bounded output;
- no raw session ID in topic titles;
- local salted display suffix;
- no message bodies in normal logs;
- secret patterns redacted from diagnostics where feasible;
- explicit command and user authorization for output reads.

## Out of scope

- compromise of the local user or kernel;
- malicious code already running with access to the Herdr socket;
- Telegram service compromise;
- vulnerabilities in an agent provider itself;
- safe execution of arbitrary prompts by coding agents.

These are not dismissed; they sit outside the bridge's enforcement boundary.

## Security acceptance gates

Before production enablement:

1. unauthorized envelope matrix passes;
2. bot token never appears in repository, process arguments, normal logs, or captured exceptions;
3. focused-pane changes cannot alter routing;
4. stale route/session mismatch fails before input;
5. duplicate and ambiguous dispatch tests pass;
6. plugin disable/unlink blocks mutations;
7. competing poller and daemon tests fail closed;
8. manual topic close/delete does not silently recreate;
9. production credentials are excluded from all automated tests;
10. a human reviews and explicitly approves the threat model and production rollout.

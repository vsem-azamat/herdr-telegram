# References

References are evidence, not hidden requirements. The repository's architecture and tests remain the specification.

## Herdr — authoritative

- [Plugins](https://herdr.dev/docs/plugins/) — plugin v1 manifest, trust model, startup hooks, config/state paths, install/link behavior.
- [Socket API](https://herdr.dev/docs/socket-api/) — snapshots, panes, agents, events, protocol stability, and raw request shapes.
- [CLI reference](https://herdr.dev/docs/cli-reference/) — portable wrappers and plugin commands.
- [Plugin marketplace](https://herdr.dev/plugins/) — community discovery model.
- [Herdr v0.7.5 release](https://github.com/herdrdev/herdr/releases/tag/v0.7.5) — current verified baseline.
- [Herdr v0.7.5 tagged API schema source](https://github.com/herdrdev/herdr/tree/v0.7.5/src/api/schema) — exact request, response, snapshot, and agent field contracts used by the Go SDK fixtures.
- [Herdr development source audited for the expected-session spike](https://github.com/herdrdev/herdr/tree/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965) — protocol 18 schema and server implementation reviewed on 2026-07-29; it still had no atomic expected-session prompt precondition. See [`spikes/herdr-expected-session.md`](spikes/herdr-expected-session.md).
- [Herdr Discussion #2016: Allow `agent.prompt` to check the expected session ID](https://github.com/herdrdev/herdr/discussions/2016) — owner-authorized upstream direction request for atomic stable-session dispatch; not yet an accepted upstream contract.
- [Personal Herdr fork PR #1](https://github.com/vsem-azamat/herdr/pull/1) — independently reviewed temporary implementation of the capability-gated expected-session contract, squash-merged only to the personal fork. Conceptual lesson: native provider session identity is useful only when the server compares it atomically with pane input; reading or scraping the ID before an unconditional prompt does not close the occupant-replacement race. No fork source or tests are copied into this repository.
- [Disposable expected-session live probe](spikes/herdr-expected-session-live-probe.md) — redacted local evidence produced through this repository's Go client and disposable Herdr resources; no production credentials or active user sessions.
- [Disposable plugin/systemd lifecycle probe](spikes/plugin-systemd-lifecycle.md) — redacted local evidence for descriptor/restart behavior and the unresolved atomic plugin-revocation race.
- [Issue #1270: retained event replay](https://github.com/herdrdev/herdr/issues/1270) — why snapshots are authoritative and event bootstrap needs buffering/reconciliation.
- [Herdr contributing guide](https://github.com/herdrdev/herdr/blob/master/CONTRIBUTING.md) — project direction and proposal process.

## Telegram Bot API — authoritative

- [Bot API 10.2 documentation observed 2026-08-02](https://core.telegram.org/bots/api)
- [`getUpdates`](https://core.telegram.org/bots/api#getupdates) — offset confirmation semantics and long polling.
- [`Update`](https://core.telegram.org/bots/api#update) — update identity and supported payloads.
- [`User`](https://core.telegram.org/bots/api#user) / [`getMe`](https://core.telegram.org/bots/api#getme) — `has_topics_enabled` and `allows_users_to_create_topics` for private bot chats.
- [`Message`](https://core.telegram.org/bots/api#message) — `message_thread_id`, `is_topic_message`, sender/forward fields, and alternate `business_connection_id` / `guest_query_id` namespaces; current docs include private chats with bot topic mode.
- [`sendMessage`](https://core.telegram.org/bots/api#sendmessage)
- [`createForumTopic`](https://core.telegram.org/bots/api#createforumtopic)
- [`editForumTopic`](https://core.telegram.org/bots/api#editforumtopic)
- [`getWebhookInfo`](https://core.telegram.org/bots/api#getwebhookinfo)
- [`getChat`](https://core.telegram.org/bots/api#getchat)
- [Redacted private-topic prerequisite checkpoint](spikes/telegram-private-topics.md) — read-only evidence from the newly provisioned development bot; no mutation or production enablement.
- [Bots FAQ: privacy mode](https://core.telegram.org/bots/faq#what-messages-will-my-bot-get)

Important conclusions derived from the documented API:

- calling `getUpdates` with `offset > update_id` confirms earlier updates;
- bots can enable forum topic mode in private chats; this is advertised by `getMe.has_topics_enabled`, while `getChat` still reports `type=private` rather than a forum supergroup;
- `createForumTopic` supports a private chat with a user and has no caller-provided idempotency key;
- Telegram has no reliable Bot API method to enumerate/search every forum topic;
- `sendMessage` has no caller-provided idempotency key;
- a configured webhook and `getUpdates` are mutually exclusive;
- `409` can indicate another long poller;
- `429` may include `retry_after`.

## Go and runtime

- [Go documentation](https://go.dev/doc/)
- [`context`](https://pkg.go.dev/context)
- [`net/http`](https://pkg.go.dev/net/http)
- [`database/sql`](https://pkg.go.dev/database/sql)
- [`embed`](https://pkg.go.dev/embed)
- [Go race detector](https://go.dev/doc/articles/race_detector)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [SQLite WAL](https://sqlite.org/wal.html)

## Service lifecycle

- [`systemd.service`](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html)
- [`systemd.exec`](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html)
- [`systemctl --user`](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html)

## Existing community work — comparative only

These projects helped validate demand and failure modes. Their code and Git history are not imported:

- [CCGram](https://github.com/alexei-led/ccgram) — mature Telegram bridge with tmux/Herdr backends and current tab-oriented Herdr mapping.
- [herdr-telegram-remote](https://github.com/etinpres/herdr-telegram-remote) — direct pane-oriented Telegram topics.
- [herdr-telegram-plugin](https://github.com/mvallebr/herdr-telegram-plugin) — community Herdr plugin experiment.
- [herdr-remote](https://github.com/dcolinmorgan/herdr-remote) — remote/mobile control patterns.
- [Herdres](https://github.com/luminexord/herdres) — durable worker/topic delivery design with a substantially larger operational model.

Allowed lessons:

- stable target identity matters;
- focused-pane routing is unsafe for independent agents;
- recovery and authorization dominate the hard parts;
- forum-topic creation and output delivery have ambiguous network boundaries;
- a small single-user bridge should avoid distributed infrastructure.

Not allowed:

- copying modules, tests, README sections, or schemas;
- preserving another project's internal structure by default;
- presenting imported code as clean-slate work.

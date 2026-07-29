# References

References are evidence, not hidden requirements. The repository's architecture and tests remain the specification.

## Herdr — authoritative

- [Plugins](https://herdr.dev/docs/plugins/) — plugin v1 manifest, trust model, startup hooks, config/state paths, install/link behavior.
- [Socket API](https://herdr.dev/docs/socket-api/) — snapshots, panes, agents, events, protocol stability, and raw request shapes.
- [CLI reference](https://herdr.dev/docs/cli-reference/) — portable wrappers and plugin commands.
- [Plugin marketplace](https://herdr.dev/plugins/) — community discovery model.
- [Herdr v0.7.5 release](https://github.com/ogulcancelik/herdr/releases/tag/v0.7.5) — current verified baseline.
- [Herdr v0.7.5 tagged API schema source](https://github.com/ogulcancelik/herdr/tree/v0.7.5/src/api/schema) — exact request, response, snapshot, and agent field contracts used by the Go SDK fixtures.
- [Herdr development source audited for the expected-session spike](https://github.com/ogulcancelik/herdr/tree/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965) — protocol 18 schema and server implementation reviewed on 2026-07-29; it still has no atomic expected-session prompt precondition. See [`spikes/herdr-expected-session.md`](spikes/herdr-expected-session.md).
- [Issue #1270: retained event replay](https://github.com/ogulcancelik/herdr/issues/1270) — why snapshots are authoritative and event bootstrap needs buffering/reconciliation.
- [Herdr contributing guide](https://github.com/ogulcancelik/herdr/blob/master/CONTRIBUTING.md) — project direction and proposal process.

## Telegram Bot API — authoritative

- [Bot API](https://core.telegram.org/bots/api)
- [`getUpdates`](https://core.telegram.org/bots/api#getupdates) — offset confirmation semantics and long polling.
- [`Update`](https://core.telegram.org/bots/api#update) — update identity and supported payloads.
- [`Message`](https://core.telegram.org/bots/api#message) — `message_thread_id`, `is_topic_message`, sender and forward fields.
- [`sendMessage`](https://core.telegram.org/bots/api#sendmessage)
- [`createForumTopic`](https://core.telegram.org/bots/api#createforumtopic)
- [`editForumTopic`](https://core.telegram.org/bots/api#editforumtopic)
- [`getWebhookInfo`](https://core.telegram.org/bots/api#getwebhookinfo)
- [`getChat`](https://core.telegram.org/bots/api#getchat)
- [`getChatMember`](https://core.telegram.org/bots/api#getchatmember)
- [Bots FAQ: privacy mode](https://core.telegram.org/bots/faq#what-messages-will-my-bot-get)

Important conclusions derived from the documented API:

- calling `getUpdates` with `offset > update_id` confirms earlier updates;
- `createForumTopic` has no caller-provided idempotency key;
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

# Telegram private-topic prerequisite checkpoint

> Recorded: 2026-08-02. This is read-only development evidence, not disposable mutation/recovery evidence or production enablement.

## Specification correction

The repository originally assumed Telegram topics required a forum supergroup. Current Bot API 10.2 documentation instead exposes bot topic mode in private chats:

- `getMe.result.has_topics_enabled` reports private topic mode;
- `getMe.result.allows_users_to_create_topics` reports whether users may create/delete private topics;
- private topic messages carry `is_topic_message` and `message_thread_id`;
- `createForumTopic` and `editForumTopic` accept a private chat with a user;
- `getChat` still reports the chat as `type = "private"`; `is_forum` is a supergroup field and is not the capability check for this mode.

Therefore the intended one-user UX does not need a supergroup, bot administrator status, `can_manage_topics`, `getChatMember`, or group privacy-mode configuration. D-021 records the revised scope.

## Credential handling

The repository-local `.env` is ignored by Git and remained untracked. Its values were not printed. The file initially had mode `0644`; it was changed to `0600` before any Telegram request. `CHAT_ID` was derived locally from the configured `ADMIN_ID` without displaying either value.

After the read-only probe, the token was removed from `.env` and placed in `~/.config/herdr-telegram/dev-bot.token`, outside the worktree, with a mode-`0700` parent and mode-`0600` regular file. `.env` now contains only `BOT_TOKEN_FILE`, `ADMIN_ID`, and `CHAT_ID`.

The read-only probe loaded the token in process memory and used direct HTTPS requests. The token was not put in process arguments, logs, committed fixtures, or a Bot API error string.

## Redacted read-only result

```text
getMe:
  ok=true
  bot=true
  topics_enabled=true
  users_may_create_topics=true

getWebhookInfo:
  ok=true
  webhook_empty=true
  pending_updates=0

getChat:
  ok=true
  id_matches=true
  type=private
  is_forum=false
```

This proves the configured bot/chat satisfy the read-only private-topic capability and webhook prerequisites at the observation time. It does not prove update admission, topic creation, ambiguous recovery, competing polling, sending, or cleanup.

## Remaining mutation gate

The supplied bot is intended for later personal use, so it is not automatically treated as disposable merely because it is new. Before a live mutation probe, the owner must explicitly permit:

- creating and deleting clearly named test topics in the private bot chat;
- consuming test updates with `getUpdates`;
- coordinating a second poller to observe `409` only while no real poller is active;
- stopping on any ambiguous topic-creation or send outcome rather than retrying;
- rotating the token if test handling exposes or ambiguously compromises it.

`429`, malformed-response, and post-transmission timeout classification should use local redacted HTTP fixtures unless Telegram provides a safe deterministic disposable trigger. Production routing remains disabled.

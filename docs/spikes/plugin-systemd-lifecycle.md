# Disposable plugin/systemd lifecycle spike

> Recorded: 2026-07-31. This is redacted Phase 0 development evidence, not a production service or routing enablement.

## Question

Can a separately installed `systemd --user` companion safely consume one-shot Herdr plugin registration, survive bounded restart, refresh across Herdr restart, and deny modeled mutations after plugin disable or unlink?

## Environment and isolation

- Herdr server: side-by-side personal fork binary built from merged fork commit `a8758ff3`.
- Herdr config, state, socket, plugin checkout, descriptor, control socket, and mutation log: created under `testing.T.TempDir`.
- Plugin ID and Herdr instance ID: fixed disposable values with no native agent or Telegram identity.
- Companion: the Go test helper executable launched as a transient, uniquely named `systemd --user` service; it is outside the linked plugin directory.
- Telegram: not used.
- Agent prompt or terminal mutation: not used. The companion appends a local marker only after its mutation authorization gate succeeds.
- System `/usr/bin/herdr`: unchanged.

Run:

```text
HERDR_PLUGIN_LIFECYCLE_BIN="$HOME/.local/bin/herdr-expected-session" \
  go test -race ./spikes -run '^TestPluginSystemdLifecycleLive$' -v
```

The ordinary test suite skips this probe when `HERDR_PLUGIN_LIFECYCLE_BIN` is absent.

## Descriptor contract exercised

The plugin `[[startup]]` command wrote this versioned shape to its Herdr-provided state directory:

```json
{
  "version": 1,
  "plugin_id": "<disposable-plugin>",
  "instance_id": "<configured-disposable-instance>",
  "socket_path": "<disposable-socket>",
  "herdr_binary": "<side-by-side-binary>",
  "herdr_version": "<reported-version>",
  "protocol": 18,
  "plugin_config_dir": "<disposable-config-dir>",
  "plugin_state_dir": "<disposable-state-dir>",
  "server_pid": 123,
  "server_start_ticks": "<linux-proc-start-time>",
  "registration_nonce": "<disposable-nonce>"
}
```

The hook created a mode-`0600` temporary file, flushed it, and atomically renamed it over `runtime.json`. A concurrent-reader stress boundary alternated 100 larger old/new documents through the same publication function and observed only complete old or complete new bytes—never a missing, partial, mixed, or invalid file. The companion rejected a descriptor unless it was:

- a same-UID regular non-symlink file with exact mode `0600`;
- an at-most-16-KiB JSON document, opened with Linux `O_NOFOLLOW` and validated by `fstat`, with the expected descriptor version, plugin ID, stable configured instance ID, binary, Herdr version/protocol, socket, config directory, and state directory;
- tied to a currently live Linux process by PID plus `/proc/<pid>/stat` start time;
- paired with a same-UID Unix socket that granted no group/other mode bits and whose Linux `SO_PEERCRED` PID/UID matched the descriptor's live server identity.

The process start time prevents a recycled PID from making an old descriptor current. This is a Linux spike contract, not yet a committed production descriptor API.

## Observed lifecycle

1. The plugin was linked offline in an isolated Herdr registry.
2. Starting disposable Herdr ran the one-shot startup hook after socket readiness and produced a complete descriptor.
3. A transient companion service intentionally failed its first start. `Restart=on-failure`, `RestartSec=100ms`, and `StartLimitBurst=3` restarted it exactly once into a ready state. A second persistently failing unit reached `Result=start-limit-hit`; its start counter then remained unchanged inside the configured ten-second interval.
4. With the plugin enabled, the companion re-read and validated the descriptor, queried authoritative `plugin.list`, and accepted one modeled mutation.
5. After `plugin disable` returned, the next modeled mutation returned `plugin_disabled` and the mutation log did not advance.
6. After re-enable, mutation was accepted again.
7. After `plugin unlink` returned, the next mutation returned `plugin_not_found` and the log did not advance.
8. While Herdr was stopped, the old descriptor failed closed.
9. Restarting Herdr reran the hook and replaced the descriptor with a different server process identity and nonce.
10. Replaying the old descriptor against the reused socket path returned `stale_descriptor`.
11. Changing the current descriptor to mode `0644` returned `descriptor_invalid`.
12. Appending enough trailing bytes to exceed the 16-KiB descriptor bound also returned `descriptor_invalid`.

Redacted terminal result:

```text
startup_descriptor=atomic
restart_policy=bounded
disable_after_return=denied
unlink_after_return=denied
restart_descriptor=refreshed
stale_descriptor=denied
insecure_descriptor=denied
oversized_descriptor=denied
disable_race=accepted_blocker
```

## Blocking race discovered

A separate deterministic boundary test paused the companion after `plugin.list` returned enabled but before the modeled side effect. The test then waited for `plugin disable` to return and created the release marker; the helper refuses to commit if that marker does not arrive before its coordination deadline. The released operation still committed, and the mutation log advanced only after disable had returned.

Therefore:

```text
plugin.list(enabled) → plugin disable returns → mutation
```

is possible for an already-authorized in-flight operation. Rechecking enabled state immediately before mutation narrows the window but does not make disable/unlink a revocation fence. Filesystem watching, polling, descriptor freshness, and systemd restart policy do not close this TOCTOU boundary.

The current Herdr `agent.prompt` request has no atomic plugin-enabled precondition or authenticated plugin caller identity. The bridge also cannot safely solve this by holding Herdr's private plugin-registry lock around a socket mutation: that is an internal format/lock contract and can deadlock with the server actor that processes both plugin registry and prompt requests.

## Conclusion

The one-shot descriptor, separately installed companion, bounded systemd restart, sequential disable/unlink denial, restart refresh, stale replay rejection, and unsafe-file rejection are feasible.

The Phase 0 lifecycle family does **not** pass because disable/unlink cannot yet be proven to fence an already-authorized remote mutation. Automatic Telegram-to-agent routing remains disabled.

A product implementation needs one accepted linearization mechanism, for example a server-owned mutation precondition tied atomically to current plugin registration/enabled state, or a different lifecycle authority explicitly approved in the architecture. Do not add such a Herdr API or weaken the plugin-disable invariant without separate alignment.

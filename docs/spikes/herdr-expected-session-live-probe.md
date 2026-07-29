# Disposable expected-session live probe

> Recorded: 2026-07-29. This is redacted Phase 0 development evidence, not production enablement.

## Environment

- Client: this repository's Go SDK at commit `edbd461` plus the live-probe test in PR #8.
- Server: side-by-side personal fork binary built from `vsem-azamat/herdr` commit `b610183d`.
- Server protocol: 18.
- Installed probe command: `~/.local/bin/herdr-expected-session`.
- System `/usr/bin/herdr`: left unchanged.
- Workspace, panes, fake Pi process, config, socket, session paths, and input log: created under `testing.T.TempDir` and removed after the test.
- Focus: moved to a second disposable workspace before prompting the target pane.

The probe uses Herdr's public socket methods to create disposable workspaces and report native session transitions. Prompt dispatch itself goes through `herdr.Client`. No Telegram or active user agent session is involved.

## Capability gate

The same test was first run against ordinary installed Herdr v0.7.5. It failed before mutation as required:

```text
expected-session capability absent:
AgentPromptExpectedSession:false
```

It was then run against the side-by-side fork:

```text
HERDR_EXPECTED_SESSION_BIN=$HOME/.local/bin/herdr-expected-session \
  go test ./herdr -run '^TestExpectedSessionForkLive$' -v
```

The server advertised:

```json
{
  "type": "pong",
  "protocol": 18,
  "capabilities": {
    "agent_prompt_expected_session": true
  }
}
```

## Redacted request sequence

The target pane initially reported disposable native session A. The bridge client sent all four identity components while focus was on another workspace:

```json
{
  "method": "agent.prompt",
  "params": {
    "target": "<disposable-pane>",
    "text": "<accepted-marker>",
    "expected_session": {
      "source": "herdr:pi",
      "agent": "pi",
      "kind": "path",
      "value": "<disposable-session-a>"
    }
  }
}
```

The matching prompt succeeded, returned session A, and exactly one accepted marker appeared in the disposable process input log.

A second accepted prompt requested a wait for `blocked`. After its input arrived, the probe changed the same pane's native identity to session B and reported B as blocked. The wait returned `agent_not_running` rather than allowing B to satisfy A's wait. Because prompt submission had already succeeded, the Go SDK correctly retained this outcome as `AmbiguousPromptError` with the typed API error underneath.

The client then retried only as a separate stale-target assertion, not as automatic recovery. It sent expected session A after the authoritative pane occupant had become B:

```json
{
  "method": "agent.prompt",
  "params": {
    "target": "<same-disposable-pane>",
    "text": "<must-not-arrive-marker>",
    "expected_session": {
      "source": "herdr:pi",
      "agent": "pi",
      "kind": "path",
      "value": "<disposable-session-a>"
    }
  }
}
```

The correlated response was:

```json
{
  "error": {
    "code": "agent_session_mismatch",
    "message": "agent target no longer hosts the expected session"
  }
}
```

After waiting beyond Herdr's delayed Enter interval, neither the rejected marker nor delayed rejected input appeared in the disposable process input log.

## Result

```text
protocol=18
capability=true
focus_independent=true
matching_prompt=accepted
wait_session_pinned=true
replacement_prompt=agent_session_mismatch
replacement_input=false
```

This closes the disposable client-level expected-session dispatch and wait-identity evidence for the temporary fork. It does not prove completion correlation to a submitted turn, upstream acceptance, plugin disable/unlink fencing, or Telegram prerequisites. Automatic Telegram routing remains disabled.

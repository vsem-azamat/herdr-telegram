# Herdr atomic expected-session contract spike

> Status: still unavailable in ordinary upstream Herdr. A temporary personal fork implements the proposed contract for development and disposable validation; it is not an upstream release or production approval.

## Scope and safety boundary

This spike asks one question: can `agent.prompt` atomically reject an explicit pane target when that pane no longer hosts the native agent session observed by the caller?

The answer is **no** for both the latest stable release and the current upstream development branch reviewed below. Consequently:

- automatic Telegram-to-agent prompt routing remains disabled;
- `Snapshot → compare occupant → Prompt(pane)` is not an acceptable substitute;
- focus is not a routing input or fallback;
- `pane_id` remains an ephemeral address, not durable identity;
- no upstream repository, issue, fork, pull request, or release was changed or created during this spike.

## Authoritative review

The review used a separate read-only checkout of [`ogulcancelik/herdr`](https://github.com/ogulcancelik/herdr), not this repository's SDK fixtures.

| Item | Reviewed value |
|---|---|
| Latest stable release | [`v0.7.5`](https://github.com/ogulcancelik/herdr/releases/tag/v0.7.5), protocol 17 |
| Development branch | [`master` at `73d92004f50d3f5fafe64e0f9b7fddbcf4d99965`](https://github.com/ogulcancelik/herdr/tree/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965) |
| Development protocol | [18](https://github.com/ogulcancelik/herdr/blob/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965/src/protocol/wire.rs#L15-L16) |
| Review time | 2026-07-29 UTC |

At the audited upstream commit, the generated schema and Rust request type exposed only `target`, `text`, and optional `wait`:

- [`AgentPromptParams`](https://github.com/ogulcancelik/herdr/blob/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965/src/api/schema/agents.rs#L175-L181);
- [generated development JSON schema](https://github.com/ogulcancelik/herdr/blob/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965/docs/next/api/herdr-api.schema.json#L1225-L1254);
- [`handle_agent_prompt`](https://github.com/ogulcancelik/herdr/blob/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965/src/app/api/agents.rs#L62-L112).

The handler resolves `target`, checks the detected agent/runtime, and sends input. It does not accept or compare `AgentInfo.agent_session`. Protocol 18 therefore does not add the required contract.

The optional `wait` did not close the dispatch race. Its pre-submit `agent.get` was a separate operation, and its identity check pinned terminal/name/agent label rather than the native `agent_session` tuple. It also observes lifecycle state and does not correlate completion to the submitted turn. See [`prompt_agent`](https://github.com/ogulcancelik/herdr/blob/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965/src/api/wait.rs#L176-L304) and [`agent_wait_identity_matches`](https://github.com/ogulcancelik/herdr/blob/73d92004f50d3f5fafe64e0f9b7fddbcf4d99965/src/api/wait.rs#L524-L538).

## Temporary fork implementation

With explicit owner approval, the contract was implemented in the personal [`vsem-azamat/herdr` fork](https://github.com/vsem-azamat/herdr) at commit [`b610183d`](https://github.com/vsem-azamat/herdr/commit/b610183dd9a4424d61ad96c062cef8cb99839759) and opened as [fork-only draft PR #1](https://github.com/vsem-azamat/herdr/pull/1). The fork advertises `agent_prompt_expected_session`, compares the complete native identity before input, returns `agent_session_mismatch`, and pins waits to the native session. Its full `just check` passed, including deterministic serialized replacement-order and no-input-on-rejection tests. A side-by-side installed binary also advertised the capability in a disposable socket smoke test; the system Herdr binary was not replaced.

This fork is temporary development infrastructure. It does not establish upstream acceptance, and clients must never select the behavior by version or fork name. They must require the capability. Reading provider IDs directly from Codex, Claude, or Pi files/processes is not an alternative: those IDs identify session history but do not provide an atomic input endpoint, so pane occupant replacement remains racy.

The redacted [`herdr-expected-session-live-probe.md`](herdr-expected-session-live-probe.md) now records a disposable end-to-end probe through the Go SDK: capability absence failed closed on ordinary v0.7.5, while the fork accepted a matching unfocused-pane prompt, rejected session A after replacement by B without rejected input, and prevented B from satisfying A's wait. The separate plugin/systemd and Telegram prerequisite families remain open, and lifecycle status is still not completion evidence correlated to a particular turn. Production routing remains disabled.

## Minimal backward-compatible extension

### Request field

Add an optional `expected_session` field to `AgentPromptParams`, using the existing `AgentSessionInfo` wire shape:

```json
{
  "id": "request-redacted",
  "method": "agent.prompt",
  "params": {
    "target": "w3:p2",
    "text": "Review the current diff",
    "expected_session": {
      "source": "herdr:codex",
      "agent": "codex",
      "kind": "id",
      "value": "<redacted-session-id>"
    },
    "wait": {
      "until": ["idle", "done", "blocked"],
      "timeout_ms": 120000
    }
  }
}
```

Comparison covers all four fields exactly:

```text
source + agent + kind + value
```

The Herdr server instance is already selected by the socket endpoint and is not duplicated in this request object. Empty or otherwise invalid expected-session fields must produce a known validation error before input is emitted.

When `expected_session` is absent, existing `agent.prompt` behavior remains unchanged. Existing clients therefore continue to work against an extended server.

### Capability discovery

The request field alone is unsafe for new clients because older serde request structs accept and ignore unknown object fields. An old server could therefore execute an unconditional prompt after receiving an unrecognized `expected_session`.

Add a default-false capability such as:

```json
{
  "capabilities": {
    "agent_prompt_expected_session": true
  }
}
```

A session-safe client must require this affirmative capability before sending any automatic prompt. Protocol 17, protocol 18, field echoing, and absence of a parse error are not proof of support. Adding a default-false capability is backward-compatible with existing capability decoding.

### Mismatch error

If the resolved occupant has no native session or any component differs from `expected_session`, return:

```json
{
  "id": "request-redacted",
  "error": {
    "code": "agent_session_mismatch",
    "message": "agent target no longer hosts the expected session"
  }
}
```

Requirements:

- the error is a known non-submission outcome when the response is received;
- no prompt text, Enter key, delayed submission, or other terminal input is emitted;
- the message does not expose either the expected or current native session value;
- ordinary target-resolution errors may retain their existing codes, but a resolved pane with an absent/different session uses `agent_session_mismatch`;
- loss of the response after request transmission remains ambiguous to the client and must not be retried automatically.

### Atomic server semantics

For a request carrying `expected_session`, the server must perform one linearizable compare-and-submit operation:

1. Resolve `target` once to one current pane/runtime.
2. Read the native `AgentSessionInfo` from the same authoritative in-memory occupant state used for the mutation.
3. Compare all four session fields.
4. On mismatch, return `agent_session_mismatch` before enqueueing any input.
5. On match, enqueue the complete text-plus-Enter submission to that resolved runtime as one ordered submission and return success for that same occupant.

The linearization point is inside the serialized application mutation handler: either occupant replacement commits first and the prompt is rejected without input, or prompt submission commits first for the expected occupant. A client-side snapshot, an API-server-side `agent.get` followed by a later app dispatch, or a second validation after an unconditional send does not satisfy the contract.

If `wait` is present, the accepted native session must also be included in the wait's pinned identity. A replacement session—even the same agent kind in the same terminal—must terminate observation as an occupant change and must never satisfy the wait. This session pin still does **not** make lifecycle status completion evidence for a particular turn; turn correlation remains a separate release question.

## Required upstream tests

These tests belong in the upstream Herdr repository. They must use deterministic barriers or serialized command ordering rather than timing sleeps for the race assertion.

### Schema and compatibility

1. Round-trip `agent.prompt` with and without `expected_session`.
2. Prove omission preserves the legacy request and behavior.
3. Prove the capability defaults false when absent and is true only on an implementing server.
4. Reject malformed/empty expected-session components before terminal input.

### Compare-and-submit behavior

1. Matching all four fields succeeds and emits one complete submission to the explicit pane independently of focus.
2. A different `source`, `agent`, `kind`, or `value` fails with `agent_session_mismatch`; test each component.
3. A pane with no native session fails with `agent_session_mismatch`.
4. Every mismatch test asserts that neither text nor Enter is written or scheduled.
5. The successful response's `agent.agent_session` equals the request's `expected_session`.

### Occupant-replacement race

Start with pane `P` occupied by native session `A`, then coordinate replacement by session `B` in the same pane:

1. **Replacement wins:** pause before compare-and-submit, commit `A → B`, then release the prompt. Expect `agent_session_mismatch` and zero input for both occupants.
2. **Prompt wins:** pause replacement, commit compare-and-submit for expected `A`, then release `A → B`. Expect success and exactly one complete submission ordered for `A`; no delayed part of that submission may be inherited by `B`.
3. Repeat the replacement-wins case with `A` and `B` using the same agent kind and differing only in native session value.
4. Repeat both orderings while focus points at another pane; outcomes must be identical.
5. Run the matching and replacement cases with and without `wait`.
6. During `wait`, replace `A` with `B` after accepted submission and prove `B` cannot satisfy the wait, even when terminal ID, agent label, and status match.

The tests must observe the terminal/runtime input queue directly and assert absence of both immediate and delayed input on rejection. A test that only checks the JSON response is insufficient.

## Bridge consequence

Until an implementing server advertises the capability, passes the race suite above, and the other Phase 0 gates are complete, this project must retain all current blocks. During development that implementing server may be the temporary personal fork; a release must clearly disclose that dependency until ordinary upstream Herdr ships an equivalent contract.

- no automatic Telegram-to-agent prompt routing;
- no focused-pane fallback;
- no claim that protocol 18 is safer than protocol 17 for stable-session dispatch;
- no automatic retry after an ambiguous prompt outcome.

Before product routing, this repository still needs redacted live fixtures against a disposable server and an internal adapter that requires the capability. The SDK now models the wire field but does not itself own bridge startup policy. The `PromptWait` turn-correlation limitation must remain documented and separately resolved before completion evidence is used to release queued turns.

## Upstream publication status

With explicit owner approval, the contract gap was published as [Herdr Discussion #2016: Allow `agent.prompt` to check the expected session ID](https://github.com/ogulcancelik/herdr/discussions/2016). It remains a direction request rather than an accepted upstream contract. The authorized personal fork and fork-only draft PR are documented above. Do not create an issue or PR against `ogulcancelik/herdr` without separate explicit owner approval and maintainer alignment.

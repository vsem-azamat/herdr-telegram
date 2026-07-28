# Security policy

The project is not yet runnable. Security reports are still welcome for design flaws, unsafe defaults, credential-handling mistakes, or future implementation vulnerabilities.

## Reporting

Before a public repository and private reporting channel exist, contact the repository owner directly. Do not post bot tokens, Telegram chat/user IDs, native agent session IDs, terminal output, or local paths in a public issue.

## Security posture

Telegram access is treated as remote control over local coding agents. The required controls and residual risks are documented in [`docs/threat-model.md`](docs/threat-model.md).

No production deployment is supported until the implementation passes the security and dev-stage gates in [`docs/implementation-plan.md`](docs/implementation-plan.md).

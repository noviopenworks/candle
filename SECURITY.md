# Security Policy

## Supported versions

candle is pre-1.0. Security fixes are applied to the latest released
`0.1.x` line and the `master` branch.

| Version | Supported |
|---------|-----------|
| `0.1.x` | ✅ |
| `< 0.1` | ❌ |

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report vulnerabilities privately through GitHub's
[**Report a vulnerability**](https://github.com/noviopenworks/candle/security/advisories/new)
flow (the repository's *Security* tab → *Advisories*). This opens a private
channel with the maintainers.

When reporting, please include:

- a description of the issue and its impact,
- steps to reproduce (a minimal manifest or input is ideal),
- affected version(s) or commit, and
- any suggested remediation.

We aim to acknowledge a report within a few days and will coordinate a fix and
disclosure timeline with you. candle is a private knowledge layer that
ingests code graphs and contract files; reports about path traversal, unsafe
file handling, or data exposure across indexed repositories are especially
welcome.

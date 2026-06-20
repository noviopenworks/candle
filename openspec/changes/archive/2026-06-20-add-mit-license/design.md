# Design: add-mit-license

Tweak: add the MIT `LICENSE` file and update the README "License" section to
reference it explicitly.

## Choices considered

- **MIT** — short, permissive, the most common choice for Go tooling and MCP
  ecosystem projects. No explicit patent grant (acceptable for this project).
- **Apache-2.0** — broader patent protections, heavier notice/attribution
  obligations. Considered, but the user selected MIT for simplicity.
- **Proprietary** — rejected; the project is meant for public sharing.

## Decision

MIT, copyright holder `noviopenworks` (matches the module path
`github.com/noviopenworks/candlegraph`). Year `2026` (creation year).

## Files

- `LICENSE` — canonical MIT text from SPDX (MIT identifier).
- `README.md` — "## License" section rewritten to: short prose + link to
  `LICENSE`. Keeps the heading structure of the existing README.

No other files carry copyright headers today, so none are modified.

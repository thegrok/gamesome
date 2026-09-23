---
project: "gamesome"
last_updated: 2026-09-23
---

# Agent context

> Human-authored repository context. Do not modify during managed execution.

## Environment

- Go CLI and MCP server for cross-store game-library import and sommelier selection.
- Supported importers include Steam, Epic/Heroic, GOG, and itch.io.
- MCP and bundle packaging are part of the shipped surface.

## Codebase orientation

- `cmd/` — CLI commands and MCP integration.
- `internal/` — importer, library, persona, and selection logic.
- `.mind-harness/specs/` — mirrored feature specs.
- `.mind-harness/agent-logs/` — preserved historical work logs.

## Standards and boundaries

- Never push directly to `main`; use a feature branch and PR.
- Fail loudly on unknown platform data or malformed configuration; do not silently fall back.
- Keep credentials out of the repository and use the existing config mechanism.
- Do not hand-edit generated bundles or release artifacts.

## How to validate

- `gofmt` on changed Go files.
- `go test ./...`
- `go vet ./...`
- Run the relevant importer tests and packaging checks for the touched surface.

## Known pitfalls

- Installed-state detection is intentionally distinct from ownership.
- Windows and macOS importer behavior cannot be fully verified in this Linux sandbox.

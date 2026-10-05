# AGENTS.md

This file is the first-read brief for agents working in Gamesome.

## Hard rules

1. Never push directly to `main`; use a freshly resynced feature branch and PR.
2. Never merge a PR from an agent context.
3. Do not commit secrets, local game-library data, credentials, or generated release artifacts.
4. Do not skip repository hooks or use `--no-verify` without explicit operator instruction.

## Read first

- `.mind-harness/README.md` — execution-surface orientation.
- `.mind-harness/agent-context.md` — stack, boundaries, and validation.
- `.mind-harness/specs/` — mirrored feature plans; the vault remains canonical.
- `README.md` — user-facing product and build instructions.

## Code style

- `gofmt` and `go vet` clean on every commit.
- Prefer the standard library and existing dependencies.
- Fail loudly on malformed data; never silently default a required field to empty.
- Keep hand-written lines near 100 characters where practical.

## Managed execution

The host supplies the exact Action and canonical spec. Implement only that scope, preserve
the worktree when blocked, and do not edit workspace execution or review state.

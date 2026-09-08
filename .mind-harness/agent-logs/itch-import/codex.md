# Work log — itch-import (A063)

**Date**: 2026-06-20
**Agents**: Codex (implementation) + Claude (verify + land)

## What Codex did

- Created `internal/importer/itch.go` — `Itch(database)` function
- Modified `cmd/import.go` — added `importItchCmd` subcommand + `init()` wiring
- Ran `gofmt`
- Confirmed `go build ./...` passed (via `GOCACHE=/tmp/gamesom-go-build GOPROXY=off`)

## Claude verification

- Reviewed diff: code is idiomatic, matches Steam/Heroic patterns exactly
- `go vet ./...` — clean (0 warnings)
- `go test ./...` — `internal/normalize` ok; all other packages have no tests (pre-existing state)
- Note: disk space on LXC was 99% full; used `GOMODCACHE=/home/claude/gopath/pkg/mod` to point at already-extracted sqlite module

## Scope delivered

- `gamesom import itch` reads `~/.config/itch/db/butler.db` (download_keys + caves)
- Writes `library_entries` with `source=itchio`; idempotent via `UNIQUE(source, source_game_id)`
- butler.db opened read-only with busy_timeout=5000
- install root defaults to `~/Applications/itch` (no config file reading)

## Not in scope (as designed)

- No launcher_uri (itch:// protocol undocumented)
- No playtime (not in butler.db)
- No install root config parsing

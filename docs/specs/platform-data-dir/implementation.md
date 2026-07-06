---
feature: platform-data-dir
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/itch-unkeyed-games   # stacked, PR 3 of the 2026-07-06 run
---

# Implementation — platform-idiomatic data dir + legacy migration (A096)

## Load-bearing contract

`db.Open()` must land at the platform-idiomatic location —
`%LOCALAPPDATA%\gamesom` (Windows), `~/Library/Application Support/gamesom`
(macOS), `~/.local/share/gamesom` (Linux), `$XDG_DATA_HOME/gamesom`
overriding everywhere — and an existing legacy DB
(`~/.local/share/gamesom/gamesom.db`, Victor's) must survive: moved to the
idiomatic path when possible, still opened in place when not. Data loss is
the one unacceptable outcome.

## Files changed

### `internal/db/db.go`

**1. Rewrite `dataDir()` (db.go:347–357):**

```go
// dataDir returns the platform-idiomatic gamesom data directory.
// XDG_DATA_HOME overrides on every OS — it's also the hermetic-test hook.
func dataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gamesom"), nil
	}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(localAppData, "gamesom"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "gamesom"), nil
	default: // linux and friends
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "gamesom"), nil
	}
}
```

(`runtime` joins the import list.)

**2. New `legacyDataDir()`** — the pre-A096 non-XDG fallback, verbatim:
`~/.local/share/gamesom` via `os.UserHomeDir()`.

**3. New `resolveDBPath(dir, legacyDir string) string`** — the migration
seam, pure function of two dirs so it unit-tests with `t.TempDir()`:

```go
// resolveDBPath returns the DB path under dir, first moving a legacy DB
// (and its -wal/-shm sidecars) from legacyDir if dir has no DB yet. If the
// move fails, it returns the legacy path — opening data where it lives
// beats losing it to an idiomatic location.
func resolveDBPath(dir, legacyDir string) string {
	newPath := filepath.Join(dir, "gamesom.db")
	legacyPath := filepath.Join(legacyDir, "gamesom.db")
	if newPath == legacyPath {
		return newPath
	}
	if _, err := os.Stat(newPath); err == nil {
		return newPath // idiomatic DB already exists; legacy (if any) is stale
	}
	if _, err := os.Stat(legacyPath); err != nil {
		return newPath // nothing to migrate
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("warning: create data dir %s failed (%v); using legacy %s", dir, err, legacyPath)
		return legacyPath
	}
	if err := os.Rename(legacyPath, newPath); err != nil {
		log.Printf("warning: migrate db %s → %s failed (%v); using legacy path", legacyPath, newPath, err)
		return legacyPath
	}
	for _, ext := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(legacyPath + ext); err == nil {
			if err := os.Rename(legacyPath+ext, newPath+ext); err != nil {
				log.Printf("warning: migrate sidecar %s failed: %v", legacyPath+ext, err)
			}
		}
	}
	log.Printf("migrated gamesom.db: %s → %s", legacyPath, newPath)
	return newPath
}
```

(`log` joins the imports — stderr, never stdout: MCP mode reserves stdout
for JSON-RPC.)

**4. `Open()` (db.go:65–71)** wires them:

```go
func Open() (*sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	legacyDir, err := legacyDataDir()
	if err != nil {
		return nil, err
	}
	return OpenAt(resolveDBPath(dir, legacyDir))
}
```

### `internal/db/db_test.go` (new)

- `TestResolveDBPath_MigratesLegacy` — legacy dir holds `gamesom.db` +
  `gamesom.db-wal`; new dir empty. Assert returned path is the new one, DB +
  sidecar physically moved, legacy gone.
- `TestResolveDBPath_NewAlreadyExists` — both dirs hold DBs; assert new path
  returned and legacy left untouched (never clobber the newer DB).
- `TestResolveDBPath_NothingToMigrate` — both empty; returns new path, no
  legacy dir created.
- `TestResolveDBPath_SamePath` — dir == legacyDir (the Linux case); returns
  the path, no side effects.
- `TestDataDir_XDGOverride` — `t.Setenv("XDG_DATA_HOME", …)`; assert
  override wins on the current OS.

## Integration points

- `OpenAt` — untouched (it already MkdirAlls the parent and applies schema).
- Every caller goes through `Open()`/`OpenAt()`; no other file reads
  `dataDir` directly (grep-verified at spec time).
- A095's MCPB bundle inherits the idiomatic path with no manifest work —
  that's the "prerequisite-ish" relationship in the action.

## Sequencing

PR 3 of the stack, based on `feature/itch-unkeyed-games` (PR #21). No file
overlap with PRs 1–2 (different package); stacked for merge-order clarity
only.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — migration tests are
  hermetic (`t.TempDir()`, `t.Setenv`).
- Windows/macOS branch of `dataDir()` can't execute on the Linux sandbox —
  logic-review only here; Victor's next `gamesom.exe` run on the Windows box
  is the ground truth (expected: one-time "migrated gamesom.db" log line to
  stderr, DB now under `%LOCALAPPDATA%\gamesom`).

## Scope boundary

Only `internal/db` path resolution + migration. Do not touch launcher
config-dir resolvers (they locate *other* apps' data), schema, importers, or
MCPB packaging.

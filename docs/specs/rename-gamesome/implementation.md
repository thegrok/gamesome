---
feature: rename-gamesome
status: ready
created: 2026-07-10
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/persona-instructions   # stacked on open PR #29 (A117)
---

# Implementation — rename gamesom → gamesome (A112)

## Load-bearing contract

Every place the name `gamesom` appears in the built product moves to
`gamesome` together — module path, CLI/binary, MCPB bundle, data dir — in one
landing, so no intermediate commit ships a half-renamed binary. The one
exception with a real migration contract: **an existing `gamesom.db` at any
of the three historical data-dir locations (pre-A096 hardcoded XDG, A096
idiomatic-but-`gamesom`-named, or already-migrated) must still open correctly
and end up at the new idiomatic `gamesome`-named path with `gamesome.db` as
the filename.** Data loss is the one unacceptable outcome, same bar A096 set.

## Files changed

### `go.mod` + all internal imports

`module github.com/thegrok/gamesom` → `module github.com/thegrok/gamesome`.
Every file importing `github.com/thegrok/gamesom/...` updates to
`github.com/thegrok/gamesome/...` (14 files: `main.go`, `cmd/*.go`,
`internal/importer/*.go`, `internal/db/db.go`, plus the corresponding
`_test.go` files). Grep-and-replace the exact string `thegrok/gamesom/` →
`thegrok/gamesome/` (trailing slash keeps it from touching anything else);
`go build ./...` catches any import it missed.

### `internal/db/db.go` — the migration seam

**1. `dataDir()` (db.go:361–387)** — same per-OS structure, folder literal
changes `"gamesom"` → `"gamesome"` in all four branches (XDG override,
windows, darwin, default).

**2. Rename the *current* function body to `legacyIdiomaticDataDir()`** —
keep it byte-for-byte (still resolves the A096 per-OS idiomatic path, folder
`"gamesom"`) so it becomes a migration source instead of the live path:

```go
// legacyIdiomaticDataDir returns the A096-era idiomatic-per-OS path, still
// named "gamesom" (pre-A112 rename). Migration source only.
func legacyIdiomaticDataDir() (string, error) {
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
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "gamesom"), nil
	}
}
```

**3. `legacyDataDir()` (db.go:395–401)** — unchanged (pre-A096 hardcoded XDG
path, folder `"gamesom"`); now the oldest of two legacy candidates instead of
the only one. Update its comment to say so.

**4. `resolveDBPath` generalizes from `(dir, legacyDir string)` to
`(dir string, legacyDirs []string)`** — checked in order, so pass
newest-legacy-first:

```go
// resolveDBPath returns the DB path under dir, first moving a legacy DB
// (and its -wal/-shm sidecars) from the first legacyDirs entry that has
// one — callers pass newest-legacy-first so a DB already sitting at a more
// recent legacy location isn't skipped in favor of an older one. If the
// move fails, it returns the legacy path — opening data where it lives
// beats losing it to an idiomatic location.
func resolveDBPath(dir string, legacyDirs []string) string {
	newPath := filepath.Join(dir, "gamesome.db")
	if _, err := os.Stat(newPath); err == nil {
		return newPath // idiomatic DB already exists; legacy (if any) is stale
	}
	for _, legacyDir := range legacyDirs {
		legacyPath := filepath.Join(legacyDir, "gamesom.db")
		if _, err := os.Stat(legacyPath); err != nil {
			continue // nothing here, try the next candidate
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
		log.Printf("migrated db: %s → %s", legacyPath, newPath)
		return newPath
	}
	return newPath
}
```

Note: the old `newPath == legacyPath` short-circuit (handled the Linux case
where the A096 hop was a no-op because the folder name didn't change) no
longer applies — `gamesome.db` and `gamesom.db` are always distinct paths
even when `dir` and a `legacyDir` are the same directory, so that hop is now
a real in-place rename on Linux too, which is correct and doesn't need a
special case.

**5. `Open()` (db.go:73–85)** wires three locations instead of two:

```go
func Open() (*sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	legacyIdiomatic, err := legacyIdiomaticDataDir()
	if err != nil {
		return nil, err
	}
	legacyPreA096, err := legacyDataDir()
	if err != nil {
		return nil, err
	}
	return OpenAt(resolveDBPath(dir, []string{legacyIdiomatic, legacyPreA096}))
}
```

### `internal/db/db_test.go`

Update the four `resolveDBPath` tests to the new `(dir, []string)` signature
and `gamesome.db`/`gamesom.db` filenames:
- `TestResolveDBPath_MigratesLegacy` — one legacy dir in the slice, same
  assertions (new file present + sidecar, legacy gone), `want` is now
  `gamesome.db`.
- `TestResolveDBPath_NewAlreadyExists` — legacy untouched when
  `gamesome.db` already exists at `dir`.
- `TestResolveDBPath_NothingToMigrate` — empty slice or a slice of dirs with
  nothing in them; legacy dir(s) not created as a side effect.
- `TestResolveDBPath_SamePath` → reframe as
  `TestResolveDBPath_MigratesInPlace`: `dir` used as both the target and the
  sole legacy dir (the real Linux shape now) — `gamesom.db` renamed to
  `gamesome.db` in the same directory.
- Add `TestResolveDBPath_ChecksLegacyDirsInOrder`: two legacy dirs, DB only
  in the second — confirm it still migrates (loop doesn't stop at the first
  miss).
- `TestDataDir_XDGOverride` — `want` folder becomes `gamesome`.

### `cmd/root.go`, `cmd/version.go`, `cmd/status.go`

Literal string swaps only: `Use: "gamesom"` → `"gamesome"`; `"Print the
gamesom version"` / `"gamesom %s\n"` → `gamesome`; `"gamesom library: %d
games%s\n"` → `gamesome`.

### `cmd/mcp.go`

- `mcp.Implementation{Name: "gamesom", ...}` → `"gamesome"`.
- `writeLaunchEnvDump` doc comment + the dump header line
  `"gamesom mcp launch-environment dump (A102 diagnostic)"` → `gamesome`.
- `set_steam_credentials` tool description ("local gamesom database") →
  `gamesome`.
- `sommelierBriefing` text: "local gamesom database" and the
  `gamesom://library/summary` mention → `gamesome`.
- `registerResources`: `const uri = "gamesom://library/summary"` →
  `"gamesome://library/summary"` (not persisted anywhere — recomputed fresh
  on every read, no migration concern).

### `internal/importer/{gog,epic,legendary}.go`

Help-text error strings reference the CLI by name: `"use gamesom import
gog"`, `"use gamesom import heroic"`, `"use 'gamesom import heroic' instead"`
→ `gamesome`.

### Test files (string-literal updates, no logic change)

`cmd/mcp_env_test.go`, `cmd/mcp_steam_credentials_test.go`,
`internal/importer/steam_credentials_test.go`,
`internal/importer/e2e_windows_test.go` — update `"gamesom"` /
`"gamesom.db"` literals and comments to `gamesome` to match the renamed
product; these don't touch `internal/db` so they use the new name directly
(no legacy-path testing needed outside `db_test.go`).

### `packaging/mcpb/manifest.template.json`

`"name": "gamesom"` → `"gamesome"`; `"repository": {"url":
"https://github.com/thegrok/gamesom"}` → `.../gamesome` (live once Victor
does the GitHub rename; redirects until then). `display_name`,
`long_description`, and the `sommelier`/`game` prompt text already say
"Game Sommelier" / don't name the binary — no change needed there beyond the
`gamesom database`/`gamesom://` mentions inside the `sommelier` prompt text
mirroring the `cmd/mcp.go` sommelierBriefing changes above (the two are kept
in sync by hand; grep both after editing).

### `scripts/build-mcpb.sh`

`BINNAME=gamesom` / `BINNAME=gamesom.exe` → `gamesome`(`.exe`); output path
`dist/mcpb/gamesom_${VERSION}_${GOOS}_${GOARCH}.mcpb` → `gamesome_...`;
comment header → `gamesome`.

### `.goreleaser.yaml`

- `ldflags: -X github.com/thegrok/gamesom/cmd.Version=...` → `gamesome`.
- Add explicit `project_name: gamesome` (top level) and `binary: gamesome`
  under `builds:` — without these, GoReleaser's archive/binary naming can
  fall back to inferring from the local clone directory, which stays
  `gamesom` on disk regardless of the code/module rename; pinning both keeps
  release artifact names correct independent of the checkout dir name.

## Out of scope (see design.md)

`docs/specs/*`, `docs/agent-logs/*` (immutable history), the stray `game`
prompt entry in `manifest.template.json` (pre-existing A117 drift, not this
feature's concern), README/asciinema (A069/A080, not written yet), vault
project folder/tags (stay `game-sommelier`), the actual GitHub repo rename
(Victor's manual step, any time before the v0.1.0 tag).

## Sequencing

Stacked on `feature/persona-instructions` (open PR #29, A117) rather than
`main` — A117 isn't merged yet and the BRIEF's suggested order is A117 →
A099 → A112. `cmd/mcp.go` is the one file both branches touch (A117 added
the `Instructions` wiring and cut the `game` prompt; A112 does pure string
renames elsewhere in the same file) — low collision risk, but land in the
stated merge order: **#29 merges first, then this rebases onto `main` (or
GoReleaser/GitHub merges the stack top-down) before tagging v0.1.0.**

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...`.
- `internal/db` migration tests are hermetic (`t.TempDir()`, `t.Setenv`) and
  cover all three historical data-dir shapes converging on the new one.
- Windows/macOS `dataDir()`/`legacyIdiomaticDataDir()` branches can't execute
  on the Linux sandbox — logic-review only; call out in the PR body as a
  manual-pass item (Victor's next `gamesome.exe` run on the Windows box is
  ground truth: expect a one-time "migrated db" log line to stderr, DB now
  under `%LOCALAPPDATA%\gamesome`).
- `scripts/build-mcpb.sh` is shell, not covered by `go test` — run it by
  hand against a throwaway binary (or trust the GoReleaser CI run) to
  confirm the `.mcpb` still zips correctly with the renamed binary/manifest.

## Scope boundary

Only the rename + its one migration hop. No new features, no touching A117's
persona-instructions logic beyond the string literals that happen to share
its file, no README/demo/tag work (those are A069/A080/A081, downstream of
this).

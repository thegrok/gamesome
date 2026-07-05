---
feature: itch-install-location
status: draft
created: 2026-07-05
actions: A100
---

# Design — itch.io installed-state: resolve real install locations (A100, itch half)

*Lite design note — bugfix. Fix shape confirmed in A100 (butler source read, on-machine check 2026-07-04).*

## Problem

On Victor's real Windows machine, `gamesom import itch` reports installed games
as not-installed. His itch install location is `Games\itch.io` under his user
folder — a custom location set via itch's own Preferences; `%APPDATA%\itch\apps`
is empty (confirmed by `Get-ChildItem`, findings 003 fourth addendum).

`Itch()` (`internal/importer/itch.go`) queries only `install_folder_name` from
butler's `caves` table — a bare leaf name — and assumes it lives under
`itchConfigDir()/apps/` (itch.go:94, 137). The `os.Stat` existence gate then
fails for anything installed elsewhere, so every cave in a non-default location
imports as `installed=0`.

## Fix shape (per A100)

Butler's own models (`itchio/butler`: `database/models/cave.go` +
`install_location.go`) carry exactly the needed data:

- `Cave` has `InstallLocationID`, `InstallFolderName` (the only one read
  today), and `CustomInstallFolder` — used precisely "when InstallLocationID
  is empty", i.e. the custom-location case.
- `InstallLocation` is `{ID, Path}` — so *default*-location installs also
  resolve through the DB rather than a hardcoded `apps/` assumption.

New cave query:

```sql
SELECT c.game_id, c.install_folder_name, c.custom_install_folder, l.path
FROM caves c
LEFT JOIN install_locations l ON c.install_location_id = l.id
```

Real install path per cave, in precedence order:

1. `custom_install_folder` if non-empty (it is already a full path);
2. `filepath.Join(location_path, install_folder_name)` if the join produced a
   location path;
3. legacy `filepath.Join(itchConfigDir(), "apps", install_folder_name)` as the
   last resort (both columns empty).

The existing `os.Stat` existence gate stays — a resolved path only counts as
installed if it exists on disk.

**Schema-drift guard:** the column names are inferred from butler's Go field
names via the same snake_case convention the existing working queries already
confirm butler uses (`install_folder_name`, `download_keys`, `game_id`,
`classification`) — but they have *not* been checked against the live schema
(A100 flags this; `sqlite3` may not be available on the Windows box). So the
new query degrades loudly-but-safely: if it errors (older butler schema,
renamed column), log a warning and fall back to today's query + `apps/`
assumption — strictly no worse than current behavior, and the warning names
the failed query so the schema mismatch is visible instead of silent.

## Scope

In:
- `internal/importer/itch.go` only: the cave query + install-path resolution.

Out:
- The Epic half of A100 (PR #18, separate fix), A101 (e2e blind spot), A102
  (Steam via MCP subprocess).
- Launcher URIs, the `download_keys` owned-library side, and any importer
  behavior beyond installed-state/install-path accuracy.

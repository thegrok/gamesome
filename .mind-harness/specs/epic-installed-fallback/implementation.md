---
feature: epic-installed-fallback
status: draft
created: 2026-07-05
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: main
---

# Implementation — Epic installed-state: EGL manifest cross-check in the Heroic path (A100, Epic half)

## Load-bearing contract

When `epicFromHeroicCache` cannot read Heroic's
`legendaryConfig/legendary/installed.json`, it must still import the full owned
library from Heroic's `legendary_library.json` **and** derive per-game installed
state + install path from the native Epic Games Launcher manifests via
`EpicInstalledMap()` (keyed by the same `AppName`). Only if the EGL manifests
are also unreadable (e.g. Linux, or EGL absent) does it fall back to today's
behavior: warn and import everything as not-installed. `epicFromHeroicCache`
still returns `nil` in all these cases — `Epic()`'s fall-through chain remains
reserved for a missing/unreadable Heroic library cache.

## Files changed

### `internal/importer/epic.go`

**1. `epicFromHeroicCache` (~line 167)** — replace the empty-map substitution:

```go
installInfo, err := readHeroicInstalled()
if err != nil {
	log.Printf("warning: could not read heroic installed.json: %v", err)
	installInfo = heroicInstalledFromEGL()
}
```

**2. New helper (near `EpicInstalledMap`)**:

```go
// heroicInstalledFromEGL builds heroic-shaped install info from the native
// Epic Games Launcher manifests, for when Heroic's installed.json is
// unreadable (e.g. Heroic used only for GOG, Epic games installed natively).
func heroicInstalledFromEGL() map[string]heroicInstalledEntry {
	installInfo := map[string]heroicInstalledEntry{}
	egl, err := EpicInstalledMap()
	if err != nil {
		log.Printf("warning: could not read EGL manifests for install status: %v", err)
		return installInfo
	}
	fmt.Println("Using Epic Games Launcher manifests for install status.")
	for appName, m := range egl {
		installInfo[appName] = heroicInstalledEntry{
			AppName:     appName,
			InstallPath: m.InstallLocation,
		}
	}
	return installInfo
}
```

Both maps are keyed by Epic `AppName`, so the existing lookup loop in
`epicFromHeroicCache` needs no changes. `EpicInstalledMap()` already filters
incomplete installs (`BIsIncompleteInstall`) and non-game manifests are
irrelevant here (lookups come from the library side).

## Integration points

- `EpicInstalledMap()` (`epic.go`) — existing, unchanged; already used by the
  Legendary path and `epicFromManifests`.
- `heroicInstalledEntry` (`heroic.go`) — existing struct, unchanged.
- No signature, schema, or CLI changes.

## Sequencing

Single commit; no migration, no ordering constraints.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` in the repo.
- What can't be verified here: the real-machine behavior (Linux sandbox — no
  EGL manifests, no Heroic-for-GOG-only state). Ground truth is Victor
  re-running `gamesom.exe import epic` on the Windows machine, per the A100
  verification loop. Note A101: the windows-e2e suite as it stands would not
  catch a regression here.

## Scope boundary

Only the `installed.json`-unreadable path changes. Do not touch the
happy path (installed.json reads fine), the Legendary/EGL fall-through in
`Epic()`, `epicFromManifests`, launcher URIs, or anything in the itch importer.

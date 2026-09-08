---
feature: heroic-improvements
status: draft
created: 2026-06-23
actions: A064
---

# Design — Add GOG import via Heroic nile cache (A064)

## Problem

gamesom imports Epic games via Heroic's Legendary cache but has no GOG importer.
Heroic uses **nile** as its GOG backend (analogous to Legendary for Epic), and the
nile library cache lives alongside the Legendary cache in the same Flatpak config
dir — it's a free addition while we're already there.

Flatpak is the recommended install method for Heroic; native installs are not supported.

## Approach

Heroic (Flatpak) caches the GOG library at:

```
~/.var/app/com.heroicgameslauncher.hgl/config/heroic/store_cache/nile_library.json
```

Structure mirrors `legendary_library.json`: a top-level `library` array of objects
with `app_name`, `title`, `is_installed`. A separate installed manifest:

```
~/.var/app/com.heroicgameslauncher.hgl/config/heroic/nileConfig/nile/installed.json
```

Same key-by-app_name map as `legendary/installed.json`.

Source tag: `"gog"`. LauncherURI: `"nile://launch/<app_name>"`.

## Scope

In:
- Read nile library + installed caches from the existing Flatpak Heroic path
- Write `library_entries` with `source=gog`

Out:
- Native Heroic install support (Flatpak is the recommended method)
- Windows/macOS (covered by A076 — GOG Galaxy direct)

## Files changed

- `internal/importer/heroic.go` — add `HeroicGOG(database)` func following the
  same pattern as `Heroic()`
- `cmd/import.go` — wire `import gog` subcommand (or extend `import heroic` to
  cover both Epic + GOG)

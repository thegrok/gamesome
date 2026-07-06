---
feature: steam-x86-fallback
status: approved
created: 2026-07-06
author: claude
type: lite   # confirmed root-cause fix, one small function
---

# Design (lite) — resilient x86 Program Files resolution (A109)

## Problem

`steamAppsDirectories()` (`internal/importer/steam.go:151`) only adds the
32-bit Steam path when `os.Getenv("PROGRAMFILES(X86)")` is non-empty.
Confirmed via the A102 `--debug-env` diagnosis (2026-07-06,
findings/003-sommelier-layer-human-verify.md Sixth addendum): the Claude
Desktop MCP subprocess's environment doesn't include that variable at all
(not a casing issue — genuinely absent), so on this machine — where all 82
Steam games live under `Program Files (x86)\Steam\steamapps` — the directory
is silently dropped from the scan and `refresh_library` reports 0 installed.
A direct terminal launch has the variable and correctly finds all 82.

## Decision

Prefer the env var when present (matches today's behavior exactly whenever
the variable exists — the terminal case, and any future case where the
subprocess environment does carry it). **Fall back to deriving the path from
`PROGRAMFILES`** when it's absent: on every 64-bit Windows install,
`%ProgramFiles(x86)%` is guaranteed to be `%ProgramFiles%` with `" (x86)"`
appended — same drive, same convention, by design of WOW64 — and
`PROGRAMFILES` was present in both the terminal and the Claude-Desktop-spawned
dumps. No new dependency on an env var that launch context may or may not
provide.

If the derived directory doesn't exist (e.g. a genuine 32-bit Windows install
with no x86 counterpart), the existing glob-based manifest scan simply finds
nothing there — same as today's behavior when Steam isn't installed at a
candidate path. No behavior change for machines where the env var is present
and correct; this only affects the case that was silently broken.

## Out of scope

`libraryfolders.vdf` / secondary Steam Library folders (ruled out for this
machine in findings/003's Fourth/Fifth addenda — Victor's Steam Storage
setting is the plain default path), the itch/Epic install-status bugs
(already fixed via A100), any other importer.

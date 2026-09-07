---
feature: mcp-env-dump
status: approved
created: 2026-07-06
author: claude
type: lite   # diagnostic tooling; the A102 diagnosis itself stays open
---

# Design (lite) — MCP launch-environment dump (A102 instrumentation)

## Problem

Steam import finds 82 installed when `gamesom.exe import steam` runs from a
terminal, but 0 when the same binary runs via `refresh_library` inside
Claude Desktop's MCP subprocess — cause genuinely unknown (A102). The only
established fact is that launch context matters. Per the action: **do not
fix based on any guess**; the one way to know is to dump `os.Environ()` and
cwd from inside a `gamesom mcp` process actually launched by Claude Desktop
and diff it against a terminal launch.

This feature ships the instrument. The diagnosis (running it on the Windows
box, reading the diff) is Victor's; **A102 stays open** after this merges.

## Decision

**Opt-in `--debug-env` flag on `gamesom mcp`**, not an env-var trigger: the
environment is the thing under suspicion, so an env-var switch could fail to
fire in exactly the broken context. CLI args pass reliably through
`claude_desktop_config.json`'s `args` array.

**Dump goes to a file in the gamesom data dir** (post-A096:
`%LOCALAPPDATA%\gamesom`), never stdout — stdout is the JSON-RPC stream. The
data dir is proven reachable from the Desktop-spawned process (DB reads
worked there), unlike `%TMP%`-derived paths which themselves depend on the
suspect environment. One timestamped+pid file per launch so the two launches
being compared can't overwrite each other. The chosen path is also logged to
stderr (Claude Desktop captures MCP stderr into its logs).

**Contents:** timestamp, pid, executable path, args, cwd, current user, and
the full sorted environment — sorted so `diff` of two dumps is directly
readable. Header warns the file may contain secrets (e.g. `STEAM_API_KEY`)
and should be deleted after diagnosis. Dump failure is logged, never fatal —
the instrument must not take down the server.

## Diagnosis procedure (for the human, post-merge)

1. Add `"--debug-env"` to gamesom's `args` in `claude_desktop_config.json`;
   restart Claude Desktop (dump happens at server launch, no tool call
   needed).
2. From a terminal: `gamesom.exe mcp --debug-env` (Ctrl-C after a moment).
3. Diff the two `mcp-env-*.log` files in `%LOCALAPPDATA%\gamesom`; also
   compare `cwd:` and `user:` lines. Remove the flag + files when done.

## Out of scope

Any Steam-import fix (that's the diagnosis's output, a future action),
always-on logging, log rotation.

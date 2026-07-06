# Work log — mcp-env-dump (A102 instrumentation)

Agent: Claude (direct implementation, /feature default path)
Date: 2026-07-06

## What was built

Per `docs/specs/mcp-env-dump/implementation.md`, no deviations:

- `gamesom mcp --debug-env` (cobra flag) → `writeLaunchEnvDump()` first
  thing in `RunE`, before `db.Open()` so the dump survives any later
  failure. Dump failure logs a stderr warning, never kills the server.
- `writeLaunchEnvDump` → `mcp-env-<timestamp>-pid<pid>.log` in
  `db.DataDir()` (new one-line export of A096's `dataDir()`), `O_EXCL` so
  a filename collision errors instead of clobbering the dump being
  compared against.
- `renderLaunchEnv(w io.Writer)`: secrets warning header, time/pid/exe/
  args/cwd/user identity lines (failures reported inline in the dump, not
  returned — partial dump is still diagnostic data), then `os.Environ()`
  sorted for clean diffs.
- `cmd/mcp_env_test.go`: content assertions (marker env var, cwd line,
  header, env-block sortedness) + file-writing path hermetically via the
  `XDG_DATA_HOME` override.

## Load-bearing choices

- **CLI flag over env-var trigger**: the environment is the suspect, so an
  env switch could silently not fire in exactly the broken (Claude
  Desktop) context. `claude_desktop_config.json` `args` pass reliably.
- **Data dir over `%TMP%`**: the DB provably works in the Desktop-spawned
  process, so its directory is reachable there; TMP-derived paths depend
  on the environment under suspicion.
- stderr for the path echo — Claude Desktop captures MCP stderr into its
  logs, so the dump is findable even from the Desktop side.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — clean, both new
  tests confirmed running via `-v`.
- Not verifiable here: an actual Claude-Desktop-spawned run (Windows box).
  The diagnosis procedure is in the design doc / PR body.

## Action status

**A102 stays open.** This PR is the instrument; the diagnosis (run both
launches, diff the dumps) is Victor's, and whatever it reveals becomes the
fix action.

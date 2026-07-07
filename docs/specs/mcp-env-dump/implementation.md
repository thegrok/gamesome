---
feature: mcp-env-dump
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/e2e-installed-assertions   # stacked, PR 5 of the 2026-07-06 run
---

# Implementation — MCP launch-environment dump (A102 instrumentation)

## Load-bearing contract

With `--debug-env`, `gamesom mcp` writes one complete launch-context snapshot
(env + cwd + user + exe + args) to a uniquely-named file in the gamesom data
dir **before** anything else can fail, logs the path to stderr, and never
writes a byte of it to stdout. Without the flag, behavior is byte-identical
to today. A dump failure is a stderr warning, never a server-fatal error.

## Files changed

### `internal/db/db.go`

**1. Export the data-dir resolver** (the dump writes next to the DB — the one
location proven reachable from the Desktop-spawned process):

```go
// DataDir exposes the platform data directory for siblings that store
// operational files (e.g. the mcp --debug-env dump) beside the DB.
func DataDir() (string, error) { return dataDir() }
```

### `cmd/mcp.go`

**2. Flag wiring** in `init()`:

```go
mcpCmd.Flags().BoolVar(&debugEnv, "debug-env", false,
	"write a launch-environment dump (env, cwd, user) to the data dir at startup — A102 diagnostic")
```

with `var debugEnv bool` at package level.

**3. First thing in `RunE`** (before `db.Open()` — the dump must survive any
later failure):

```go
if debugEnv {
	if path, err := writeLaunchEnvDump(); err != nil {
		log.Printf("warning: --debug-env dump failed: %v", err)
	} else {
		log.Printf("mcp: launch environment dumped to %s", path)
	}
}
```

**4. The dump writer** — two layers so the content is unit-testable:

```go
// writeLaunchEnvDump writes the launch context to a timestamped file in the
// gamesom data dir and returns its path. File per launch: the diagnosis is
// a diff between a Claude-Desktop launch and a terminal launch, so the two
// dumps must not overwrite each other.
func writeLaunchEnvDump() (string, error) {
	dir, err := db.DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("mcp-env-%s-pid%d.log",
		time.Now().Format("20060102-150405"), os.Getpid()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := renderLaunchEnv(f); err != nil {
		return "", err
	}
	return path, nil
}

// renderLaunchEnv writes the launch-context report: header, identity lines,
// then the environment sorted so two dumps diff cleanly.
func renderLaunchEnv(w io.Writer) error { … }
```

`renderLaunchEnv` writes:

- header: `gamesom mcp launch-environment dump (A102 diagnostic)` + a
  `WARNING: may contain secrets — delete after diagnosis` line
- `time:` RFC3339, `pid:`, `exe:` (`os.Executable()`, error text inline on
  failure), `args:` (`%q` of `os.Args`), `cwd:` (`os.Getwd()`, error text
  inline), `user:` (`os/user.Current()`, error text inline — a
  different-user/service context is one of the A102 hypotheses)
- `--- environment (sorted) ---` then `sort.Strings(os.Environ())`, one per
  line

Identity-line failures (exe/cwd/user) are **reported in the dump, not
returned as errors** — a partial dump is diagnostic data, not a failure.

**5. Imports:** `cmd/mcp.go` adds `log`, `path/filepath`, `sort`, `time`,
`os/user`, `io` (`os`/`fmt` present).

### `cmd/mcp_env_test.go` (new)

- `TestRenderLaunchEnv` — render to a buffer with `t.Setenv("GAMESOM_TEST_MARKER", "x")`;
  assert the marker line, the `cwd:` line matching `os.Getwd()`, the warning
  header, and sortedness of the env block.
- `TestWriteLaunchEnvDump` — `t.Setenv("XDG_DATA_HOME", t.TempDir())` (the
  A096 override, which is exactly the hermetic-test hook it's documented
  as); call `writeLaunchEnvDump()`; assert the file exists under
  `<tmp>/gamesom/` and contains the header.

## Integration points

- `db.DataDir()` is a read-only export of A096's `dataDir()` — after the
  stack merges, the dump lands in `%LOCALAPPDATA%\gamesom` on Windows,
  right next to the DB Victor already knows how to find.
- No MCP tool/resource/prompt surface changes; stdout untouched (contract
  above).
- **A102 stays open** after this merges — this is the instrument, not the
  diagnosis.

## Sequencing

PR 5 of the stack, based on `feature/e2e-installed-assertions` (PR #23).
Uses A096's `dataDir()` (PR #22) via the new export — a real dependency on
PR 3 of the stack, not just merge-order convention.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — both new tests are
  hermetic.
- Manual smoke here: `XDG_DATA_HOME=/tmp/... go run . mcp --debug-env`
  can't cleanly run (stdio server blocks), but the unit tests cover the
  full dump path; the flag wiring is exercised by `--help` output check if
  needed.
- The real diagnosis run is Victor's, per the design doc's procedure.

## Scope boundary

Only the flag, the dump writer, and the `db.DataDir()` export. No importer
changes, no Steam-fix attempts (explicitly forbidden by A102 until the diff
is read), no always-on logging.

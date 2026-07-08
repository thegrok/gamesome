---
feature: steam-x86-fallback
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/mcpb-packaging   # stacked, PR 7 (top) of the 2026-07-06 run
---

# Implementation — resilient x86 Program Files resolution (A109)

## Load-bearing contract

`steamAppsDirectories()` must include the 32-bit Steam path
(`<x86 Program Files>\Steam\steamapps`) in its scan whenever a 64-bit Windows
system has one — whether or not `PROGRAMFILES(X86)` happens to be present in
the calling process's environment. Behavior must be byte-identical to today
whenever the env var **is** present.

## Files changed

### `internal/importer/steam.go`

**Replace the Windows branch of `steamAppsDirectories()` (~line 146):**

```go
case "windows":
	var dirs []string
	if pf86 := programFilesX86(); pf86 != "" {
		dirs = append(dirs, filepath.Join(pf86, "Steam", "steamapps"))
	}
	if pf := os.Getenv("PROGRAMFILES"); pf != "" {
		dirs = append(dirs, filepath.Join(pf, "Steam", "steamapps"))
	}
	return dirs
```

**New helper, same file:**

```go
// programFilesX86 returns the 32-bit Program Files directory. It prefers
// PROGRAMFILES(X86) directly — some launch contexts (confirmed: the Claude
// Desktop MCP subprocess, A102) don't pass that variable through at all, so
// falling back to deriving it from PROGRAMFILES is required for Steam
// installed-state accuracy there. On every 64-bit Windows install,
// %ProgramFiles(x86)% is %ProgramFiles% with " (x86)" appended — same drive,
// guaranteed by WOW64 — so this never invents a wrong path, only a
// nonexistent one on genuine 32-bit Windows (where the manifest glob below
// simply finds nothing, same as any other absent candidate directory).
func programFilesX86() string {
	if pf86 := os.Getenv("PROGRAMFILES(X86)"); pf86 != "" {
		return pf86
	}
	if pf := os.Getenv("PROGRAMFILES"); pf != "" {
		return pf + " (x86)"
	}
	return ""
}
```

### `internal/importer/steam_x86_test.go` (new)

- `TestProgramFilesX86_PrefersEnvVar` — `t.Setenv("PROGRAMFILES(X86)", ...)`
  set to a distinct value; assert it wins even when `PROGRAMFILES` is also
  set to something else (today's exact behavior, unchanged).
- `TestProgramFilesX86_FallsBackWhenUnset` — `t.Setenv("PROGRAMFILES(X86)", "")`,
  `t.Setenv("PROGRAMFILES", "C:\\Program Files")`; assert
  `"C:\\Program Files (x86)"`. This is the A102 case.
- `TestProgramFilesX86_EmptyWhenNeitherSet` — both unset (`t.Setenv(..., "")`);
  assert `""` (matches today's silent-skip behavior on a system with
  neither, e.g. non-Windows or a stripped environment).

`steamAppsDirectories()` itself isn't independently unit-tested today (it's
exercised via the Windows-only e2e suite); the new tests target the
extracted helper directly, which is testable on any OS since it's pure
env-var logic with no filesystem or GOOS branch.

## Integration points

- `steamManifestScan` — untouched; consumes `steamAppsDirectories()`'s output
  unchanged.
- `expectedSteamAppIDs` (e2e oracle, `e2e_windows_test.go`) — untouched; it
  calls `steamAppsDirectories()` too, so it automatically benefits from the
  same fallback when the suite runs in a context missing the env var.
- No signature change to `steamAppsDirectories()` — same `() []string`.

## Sequencing

PR 7 (top) of the 2026-07-06 stack, based on `feature/mcpb-packaging`
(PR #25). No file overlap with any earlier PR in the stack. Merge order:
#20 → #21 → #22 → #23 → #24 → #25 → this.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — the new helper tests
  are hermetic and OS-independent.
- The Windows-only branch of `steamAppsDirectories()` itself can't execute
  here; `programFilesX86()` has no `GOOS` branch, so its tests run
  everywhere and cover the actual bug.
- Real confirmation: Victor re-running `refresh_library` from Claude Desktop
  post-merge should now report 82 installed, matching the terminal run.

## Scope boundary

Only `steamAppsDirectories()`'s Windows branch and the new helper. No
changes to `libraryfolders.vdf` handling (not needed — ruled out), the
Steam Web API path, or any other importer.

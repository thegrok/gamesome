# Work-log — untrack-binary (A120)

## What shipped

- `git rm --cached gamesom` — untracked the ~15MB binary committed in
  `308806a`; local file left in place (harmless — it's `.gitignore`'d now).
- `.gitignore` — added `/gamesom` and `/gamesome` (root-anchored) under the
  existing "Binaries for programs and plugins" section.

## Spec corrections

None — spec matched repo state exactly (binary present at root, untracked
by no existing rule).

## Verification

- No Go source touched; no build/vet/test surface, per spec.
- `git status --short` after `git rm --cached`: `D  gamesom` (staged
  deletion from index), file still present on disk.
- Sanity check per spec: copied `gamesom` → `gamesome`, ran
  `git status --short --ignored` — both `gamesom` and `gamesome` show as
  `!!` (ignored), neither as untracked. Removed the copy afterward (not
  part of the commit).

## Codex cross-model review

Ran read-only against `main...HEAD` at a point where Stage 5's changes were
still unstaged in the working tree (only the spec commit existed on the
branch). Two **High** findings, both **dismissed as sequencing artifacts**:
`git rm --cached gamesom` not reflected in `HEAD`, and the `.gitignore`
entries not reflected in `HEAD` — Codex's own trailing note correctly
identified these as unstaged working-tree changes it wasn't counting as
implemented yet. This commit is exactly what resolves both. Findings at
`docs/agent-logs/untrack-binary/codex-review.md`.

---
feature: release-pipeline
status: draft
created: 2026-06-24
actions: A071
---

# Design — Cross-platform release pipeline (A071)

## Problem

gamesom binaries can only be installed by running from source. A tagged release
pipeline produces downloadable binaries for all target platforms, making the tool
usable without a Go toolchain.

## Decisions

**GoReleaser + GitHub Actions** — standard open-source Go CLI pattern. GoReleaser
handles cross-compilation, archiving, and GitHub Release creation. GitHub Actions
runs it on tag push. No alternatives considered; this is the obvious choice.

**Platforms**: Linux amd64/arm64, macOS amd64/arm64 (Apple Silicon + Intel),
Windows amd64. Windows arm64 excluded — negligible demand for CLI tools.

**CGO_ENABLED=0** — valid here. modernc.org/sqlite is pure Go; no cgo needed.
Cross-compilation works without platform-specific toolchains.

**Signing**: checksums only (checksums.txt, SHA256). OS-level code signing
(Apple notarization, Windows Authenticode) requires paid developer accounts and
is out of scope for a personal open-source project. Checksums are the standard
for open-source Go CLIs.

**Version injection**: GoReleaser injects {{.Version}} via ldflags into
github.com/thegrok/gamesom/cmd.Version (the variable already exists from PR #7).

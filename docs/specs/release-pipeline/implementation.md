---
feature: release-pipeline
status: draft
created: 2026-06-24
owner: codex
---

# Implementation — Cross-platform release pipeline (A071)

## Scope

**In**: .goreleaser.yaml at repo root + .github/workflows/release.yml
**Out**: no changes to Go source; no OS-level code signing; no Homebrew formula

## Load-bearing contract

- GoReleaser v2 config syntax (version: 2 at top)
- CGO_ENABLED=0 in build env (modernc.org/sqlite is pure Go — this is safe)
- Version injected via: -X github.com/thegrok/gamesom/cmd.Version={{.Version}}
- Trigger: tag matching v* pushed to GitHub
- Release artifacts: tar.gz (Unix), zip (Windows), checksums.txt

## Files to create

### .goreleaser.yaml (repo root)

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w -X github.com/thegrok/gamesom/cmd.Version={{.Version}}

archives:
  - formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
```

### .github/workflows/release.yml

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Verification

- go build ./... should be unaffected (no source changes)
- goreleaser check validates the config locally if GoReleaser is installed
- End-to-end test: push a v0.0.1-test tag, confirm the Actions run completes
  and a GitHub Release is created with the expected artifacts. Delete the tag +
  release after confirming. Cannot verify in sandbox — requires a real tag push
  to GitHub.

## Work-log

docs/agent-logs/release-pipeline/codex.md

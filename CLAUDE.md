# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`core/go-infra` provides infrastructure provider API clients (Hetzner Cloud, Hetzner Robot, CloudNS) and a YAML-based infrastructure configuration parser. Dependencies: `go-log` (error handling), `go-io` (file I/O), `yaml.v3`, and `testify` (tests).

## Build & Test

```bash
go test ./...              # run all tests
go test -race ./...        # run all tests with race detector
go test -run TestFoo ./... # run a single test by name
```

No build step needed for the library. The `cmd/` packages are CLI commands registered via `forge.lthn.ai/core/cli` and are built as part of the parent `core` CLI tool, not standalone binaries.

## Architecture

Single `infra` package (root directory) with four source files:

- **`client.go`** — `APIClient` with retry, exponential backoff + jitter, rate-limit (`429` + `Retry-After`) handling. Functional options pattern (`WithHTTPClient`, `WithRetry`, `WithAuth`, `WithPrefix`). Two execution paths: `Do` (JSON decode) and `DoRaw` (raw bytes).
- **`hetzner.go`** — `HCloudClient` (Bearer token auth, Hetzner Cloud API: servers, load balancers, snapshots) and `HRobotClient` (Basic auth, Hetzner Robot API: dedicated servers). Both embed `APIClient` via delegation.
- **`cloudns.go`** — `CloudNSClient` (query-param auth, CloudNS DNS API). Uses `DoRaw` because CloudNS returns non-standard JSON (e.g. `{}` for empty lists). Includes `EnsureRecord` (idempotent upsert) and ACME DNS-01 challenge helpers.
- **`config.go`** — `Config` struct parsed from `infra.yaml` via `Load()` or `Discover()` (walks parent dirs). Defines the full infrastructure topology: hosts, load balancers, DNS, SSL, database, cache, containers, S3, CDN, CI/CD, monitoring, backups.

### CLI Commands (`cmd/`)

These are subcommands for the parent `core` CLI, registered via `cli.RegisterCommands()` in `init()`:

- **`cmd/prod`** — Production infrastructure management (`status`, `setup`, `dns`, `lb`, `ssh`). Reads `infra.yaml`.
- **`cmd/monitor`** — Security finding aggregator. Pulls from GitHub code scanning, Dependabot, and secret scanning APIs via `gh` CLI.

## Coding Standards

- UK English in comments and strings
- **Error handling**: Use `coreerr.E()` from `go-log`, never `fmt.Errorf` or `errors.New`
- **File I/O**: Use `coreio.Local.Read()` from `go-io`, never `os.ReadFile`
- Tests use `testify` (`assert` + `require`)
- Test naming: `TestType_Method_Good`, `TestType_Method_Bad`, `TestType_Method_Ugly` suffixes (Good = happy path, Bad = expected errors, Ugly = edge cases)
- Tests use `httptest.NewServer` for HTTP mocking — no mock libraries
- License: EUPL-1.2

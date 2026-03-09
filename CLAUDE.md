# CLAUDE.md

## Project Overview

`core/go-infra` provides infrastructure provider API clients (Hetzner Cloud, Hetzner Robot, CloudNS) and a YAML-based infrastructure configuration parser. Zero framework dependencies — stdlib + yaml only.

## Build & Development

```bash
go test ./...
go test -race ./...
```

## Architecture

Single `infra` package with four components:

- `client.go` — Shared HTTP API client with retry, exponential backoff, rate-limit handling
- `hetzner.go` — Hetzner Cloud API (servers, load balancers, snapshots) + Hetzner Robot API (dedicated servers)
- `cloudns.go` — CloudNS DNS API (zones, records, ACME challenges)
- `config.go` — YAML infrastructure configuration parser (`infra.yaml`)

## Coding Standards

- UK English
- All functions have typed params/returns
- Tests use testify
- Test naming: `_Good`, `_Bad`, `_Ugly` suffixes
- License: EUPL-1.2

---
title: Development
description: How to build, test, and contribute to go-infra.
---

# Development

## Prerequisites

- **Go 1.26+**
- **Go workspace** -- this module is part of the workspace at `~/Code/go.work`. After cloning, run `go work sync` if module resolution fails.
- **`gh` CLI** (optional) -- required only for `core monitor` commands.

## Building

The library package (`infra`) has no binary output. The CLI commands in `cmd/prod/` and `cmd/monitor/` are compiled into the `core` binary via the `forge.lthn.ai/core/cli` module -- they are not standalone binaries.

To verify the package compiles:

```bash
cd /Users/snider/Code/core/go-infra
go build ./...
```

## Running Tests

```bash
# All tests
go test ./...

# With race detector
go test -race ./...

# A specific test
go test -run TestAPIClient_Do_Good_Success

# Verbose output
go test -v ./...
```

If the `core` CLI is available:

```bash
core go test
core go test --run TestAPIClient_Do_Good_Success
```

### Test Organisation

Tests follow the `_Good`, `_Bad`, `_Ugly` suffix convention:

| Suffix | Purpose | Example |
|--------|---------|---------|
| `_Good` | Happy path -- expected successful behaviour | `TestAPIClient_Do_Good_Success` |
| `_Bad` | Expected error conditions -- invalid input, auth failures, exhausted retries | `TestAPIClient_Do_Bad_ClientError` |
| `_Ugly` | Edge cases -- context cancellation, malformed data, panics | `TestAPIClient_Do_Ugly_ContextCancelled` |

### Test Approach

All API client tests use `net/http/httptest.Server` to mock HTTP responses. No real API calls are made during tests. The test servers simulate:

- Successful JSON responses
- HTTP error codes (400, 401, 403, 404, 500, 502, 503)
- Rate limiting (429 with `Retry-After` header)
- Transient failures that succeed after retries
- Authentication verification (bearer tokens, basic auth, query parameters)

The config tests use `Discover()` to find a real `infra.yaml` in parent directories (skipped if not present) and also test error paths with nonexistent and malformed files.

### Test Coverage by File

| File | Tests | Coverage Focus |
|------|-------|----------------|
| `client_test.go` | 20 tests | Constructor defaults/options, `Do` JSON decoding, `DoRaw` raw responses, retry on 5xx, no retry on 4xx, rate-limit handling, context cancellation, `parseRetryAfter`, integration with HCloud/CloudNS clients |
| `hetzner_test.go` | 10 tests | HCloud/HRobot constructors, `ListServers`, JSON deserialisation of servers/load balancers/Robot servers, auth header verification, error responses |
| `cloudns_test.go` | 16 tests | Constructor, auth params, raw HTTP calls, zone/record JSON parsing, CRUD round-trips, ACME challenge helpers, `EnsureRecord` logic (already correct / needs update / needs create), edge cases (empty body, empty map) |
| `config_test.go` | 4 tests | `Load` with real config, missing file, invalid YAML, `expandPath` with tilde/absolute/relative paths |

## Code Style

- **UK English** in all documentation, comments, and user-facing strings (colour, organisation, centre, serialisation).
- **Strict typing** -- all function parameters and return values have explicit types.
- **Error wrapping** -- use `fmt.Errorf("context: %w", err)` to preserve error chains.
- **Formatting** -- standard `gofmt`. Run `go fmt ./...` or `core go fmt` before committing.

## Adding a New Provider Client

To add support for a new infrastructure provider:

1. Create a new file (e.g. `vultr.go`) in the package root.
2. Define a client struct that embeds or holds an `*APIClient`:

```go
type VultrClient struct {
    apiKey  string
    baseURL string
    api     *APIClient
}

func NewVultrClient(apiKey string) *VultrClient {
    c := &VultrClient{
        apiKey:  apiKey,
        baseURL: "https://api.vultr.com/v2",
    }
    c.api = NewAPIClient(
        WithAuth(func(req *http.Request) {
            req.Header.Set("Authorization", "Bearer "+c.apiKey)
        }),
        WithPrefix("vultr API"),
    )
    return c
}
```

3. Add internal helper methods (`get`, `post`, `delete`) that delegate to `c.api.Do(req, result)`.
4. Write tests using `httptest.NewServer` -- never call real APIs in tests.
5. Follow the `_Good`/`_Bad`/`_Ugly` test naming convention.

## Adding CLI Commands

CLI commands live in subdirectories of `cmd/`. Each command package:

1. Calls `cli.RegisterCommands(AddXyzCommands)` in an `init()` function (see `cmd/prod/cmd_commands.go`).
2. Defines a root `*cli.Command` with subcommands.
3. Uses `loadConfig()` to auto-discover `infra.yaml` when needed.

The `core` binary picks up these commands via blank imports in its main package.

## Project Structure

```
go-infra/
  client.go           Shared APIClient
  client_test.go      APIClient tests (20 tests)
  config.go           YAML config types + parser
  config_test.go      Config tests (4 tests)
  hetzner.go          HCloudClient + HRobotClient
  hetzner_test.go     Hetzner tests (10 tests)
  cloudns.go          CloudNSClient
  cloudns_test.go     CloudNS tests (16 tests)
  cmd/
    prod/
      cmd_commands.go  Command registration
      cmd_prod.go      Root 'prod' command + flags
      cmd_status.go    Parallel host health checks
      cmd_setup.go     Foundation setup pipeline (discover, lb, dns)
      cmd_dns.go       DNS record management
      cmd_lb.go        Load balancer management
      cmd_ssh.go       SSH into production hosts
    monitor/
      cmd_commands.go  Command registration
      cmd_monitor.go   Security finding aggregation
  go.mod
  go.sum
  CLAUDE.md
```

## Licence

EUPL-1.2

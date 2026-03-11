---
title: Architecture
description: Internal design of go-infra -- shared HTTP client, provider clients, configuration model, and CLI command structure.
---

# Architecture

go-infra is organised into four layers: a shared HTTP client, provider-specific API clients, a declarative configuration parser, and CLI commands that tie them together.

```
cmd/prod/        CLI commands (setup, status, dns, lb, ssh)
cmd/monitor/     CLI commands (security finding aggregation)
    |
    v
config.go        YAML config parser (infra.yaml)
hetzner.go       Hetzner Cloud + Robot API clients
cloudns.go       CloudNS DNS API client
    |
    v
client.go        Shared APIClient (retry, backoff, rate-limit)
    |
    v
net/http          Go standard library
```

## Shared HTTP Client (`client.go`)

All provider-specific clients delegate HTTP requests to `APIClient`, which provides:

- **Exponential backoff with jitter** -- retries on 5xx errors and network failures
- **Rate-limit compliance** -- honours `Retry-After` headers on 429 responses
- **Configurable authentication** -- each provider injects its own auth function
- **Context-aware cancellation** -- all waits respect `context.Context` deadlines

### Key Types

```go
type APIClient struct {
    client       *http.Client
    retry        RetryConfig
    authFn       func(req *http.Request)
    prefix       string          // error message prefix, e.g. "hcloud API"
    mu           sync.Mutex
    blockedUntil time.Time       // rate-limit backoff window
}

type RetryConfig struct {
    MaxRetries     int           // 0 = no retries
    InitialBackoff time.Duration // delay before first retry
    MaxBackoff     time.Duration // upper bound on backoff duration
}
```

### Configuration via Options

`APIClient` uses the functional options pattern:

```go
client := infra.NewAPIClient(
    infra.WithHTTPClient(customHTTPClient),
    infra.WithAuth(func(req *http.Request) {
        req.Header.Set("Authorization", "Bearer "+token)
    }),
    infra.WithRetry(infra.RetryConfig{
        MaxRetries:     5,
        InitialBackoff: 200 * time.Millisecond,
        MaxBackoff:     10 * time.Second,
    }),
    infra.WithPrefix("my-api"),
)
```

Default configuration (from `DefaultRetryConfig()`): 3 retries, 100ms initial backoff, 5s maximum backoff.

### Request Flow

The `Do(req, result)` and `DoRaw(req)` methods follow this flow for each attempt:

1. **Rate-limit check** -- if a previous 429 response set `blockedUntil`, wait until that time passes (or the context is cancelled).
2. **Apply authentication** -- call `authFn(req)` to inject credentials.
3. **Execute request** -- send via the underlying `http.Client`.
4. **Handle response**:
   - **429 Too Many Requests** -- parse `Retry-After` header, set `blockedUntil`, and retry.
   - **5xx Server Error** -- retryable; sleep with exponential backoff + jitter.
   - **4xx Client Error** (except 429) -- not retried; return error immediately.
   - **2xx Success** -- if `result` is non-nil, JSON-decode the body into it.
5. If all attempts are exhausted, return the last error.

The backoff calculation uses `base = initialBackoff * 2^attempt`, capped at `maxBackoff`, with jitter applied as a random factor between 50% and 100% of the calculated value.

### Do vs DoRaw

- `Do(req, result)` -- decodes the response body as JSON into `result`. Pass `nil` for fire-and-forget requests (e.g. DELETE).
- `DoRaw(req)` -- returns the raw `[]byte` response body. Used by CloudNS, whose responses need manual parsing due to inconsistent JSON shapes.

## Hetzner Clients (`hetzner.go`)

Two separate clients cover Hetzner's two distinct APIs.

### HCloudClient (Hetzner Cloud API)

Manages cloud servers, load balancers, and snapshots via `https://api.hetzner.cloud/v1`. Uses bearer token authentication.

```go
hc := infra.NewHCloudClient("your-token")
```

**Operations:**

| Method | Description |
|--------|-------------|
| `ListServers(ctx)` | List all cloud servers |
| `ListLoadBalancers(ctx)` | List all load balancers |
| `GetLoadBalancer(ctx, id)` | Get a load balancer by ID |
| `CreateLoadBalancer(ctx, req)` | Create a load balancer from a typed request struct |
| `DeleteLoadBalancer(ctx, id)` | Delete a load balancer by ID |
| `CreateSnapshot(ctx, serverID, description)` | Create a server snapshot |

**Data model hierarchy:**

```
HCloudServer
  +-- HCloudPublicNet --> HCloudIPv4
  +-- []HCloudPrivateNet
  +-- HCloudServerType (name, cores, memory, disk)
  +-- HCloudDatacenter

HCloudLoadBalancer
  +-- HCloudLBPublicNet --> HCloudIPv4
  +-- HCloudLBAlgorithm
  +-- []HCloudLBService
  |     +-- HCloudLBHTTP (optional)
  |     +-- HCloudLBHealthCheck --> HCloudLBHCHTTP (optional)
  +-- []HCloudLBTarget
        +-- HCloudLBTargetIP (optional)
        +-- HCloudLBTargetServer (optional)
        +-- []HCloudLBHealthStatus
```

### HRobotClient (Hetzner Robot API)

Manages dedicated (bare-metal) servers via `https://robot-ws.your-server.de`. Uses HTTP Basic authentication.

```go
hr := infra.NewHRobotClient("user", "password")
```

**Operations:**

| Method | Description |
|--------|-------------|
| `ListServers(ctx)` | List all dedicated servers |
| `GetServer(ctx, ip)` | Get a server by IP address |

The Robot API wraps each server object in a `{"server": {...}}` envelope. `HRobotClient` unwraps this automatically.

## CloudNS Client (`cloudns.go`)

Manages DNS zones and records via `https://api.cloudns.net`. Uses query-parameter authentication (`auth-id` + `auth-password`).

```go
dns := infra.NewCloudNSClient("12345", "password")
```

**Operations:**

| Method | Description |
|--------|-------------|
| `ListZones(ctx)` | List all DNS zones |
| `ListRecords(ctx, domain)` | List all records in a zone (returns `map[id]CloudNSRecord`) |
| `CreateRecord(ctx, domain, host, type, value, ttl)` | Create a record; returns the new record ID |
| `UpdateRecord(ctx, domain, id, host, type, value, ttl)` | Update an existing record |
| `DeleteRecord(ctx, domain, id)` | Delete a record by ID |
| `EnsureRecord(ctx, domain, host, type, value, ttl)` | Idempotent create-or-update; returns whether a change was made |
| `SetACMEChallenge(ctx, domain, value)` | Create a `_acme-challenge` TXT record with 60s TTL |
| `ClearACMEChallenge(ctx, domain)` | Delete all `_acme-challenge` TXT records in a zone |

**CloudNS quirks handled internally:**

- Empty zone lists come back as `{}` (an object) instead of `[]` (an array). `ListZones` handles this gracefully.
- All mutations use POST with query parameters (not request bodies).
- Response status is checked via a `"status": "Success"` field in the JSON body, not HTTP status codes alone.

## Configuration Model (`config.go`)

The `Config` struct represents the full infrastructure topology, parsed from an `infra.yaml` file. It covers:

```
Config
  +-- Hosts (map[string]*Host)          Servers with SSH details, role, and services
  +-- LoadBalancer                       Hetzner managed LB (name, type, backends, listeners, health)
  +-- Network                           Private network CIDR
  +-- DNS                               Provider config + zone records
  +-- SSL                               Wildcard certificate settings
  +-- Database                          Galera/MariaDB cluster nodes + backup config
  +-- Cache                             Redis/Dragonfly cluster nodes
  +-- Containers (map[string]*Container) Container deployments (image, replicas, depends_on)
  +-- S3                                Object storage endpoint + buckets
  +-- CDN                               CDN provider and zones
  +-- CICD                              CI/CD provider, runner, registry
  +-- Monitoring                        Health endpoints and alert thresholds
  +-- Backups                           Daily and weekly backup jobs
```

### Loading

Two functions load configuration:

- `Load(path)` -- reads and parses a specific file. Expands `~` in SSH key paths and defaults SSH port to 22.
- `Discover(startDir)` -- walks up from `startDir` looking for `infra.yaml`, then calls `Load`. Returns the config, the path found, and any error.

### Host Queries

```go
// Get all hosts with a specific role
appServers := cfg.HostsByRole("app")

// Shorthand for role="app"
appServers := cfg.AppServers()
```

## CLI Commands

### `core prod` (`cmd/prod/`)

The production command group reads `infra.yaml` (auto-discovered or specified via `--config`) and provides:

| Subcommand | Description |
|------------|-------------|
| `status` | Parallel SSH health check of all hosts. Checks Docker, Galera cluster size, Redis, Traefik, Coolify, Forgejo runner. Also queries Hetzner Cloud for load balancer health if `HCLOUD_TOKEN` is set. |
| `setup` | Runs a three-step foundation pipeline: **discover** (enumerate Hetzner Cloud + Robot servers), **lb** (create load balancer from config), **dns** (ensure DNS records via CloudNS). Supports `--dry-run` and `--step` for partial runs. |
| `dns list [zone]` | List DNS records for a zone (defaults to `host.uk.com`). |
| `dns set <host> <type> <value>` | Idempotent create-or-update of a DNS record. |
| `lb status` | Display load balancer details and per-target health status. |
| `lb create` | Create the load balancer defined in `infra.yaml`. |
| `ssh <host>` | Look up a host by name in `infra.yaml` and `exec` into an SSH session. |

The `status` command uses `go-ansible`'s `SSHClient` to connect to each host in parallel, then runs shell commands to probe service state (Docker containers, MariaDB cluster, Redis ping, etc.).

### `core monitor` (`cmd/monitor/`)

Aggregates security findings from GitHub's Security tab using the `gh` CLI:

- **Code scanning alerts** -- from Semgrep, Trivy, Gitleaks, CodeQL, etc.
- **Dependabot alerts** -- dependency vulnerability alerts.
- **Secret scanning alerts** -- exposed secrets/credentials (always classified as critical).

Findings are normalised to a common `Finding` struct, sorted by severity (critical first), and output as either a formatted table or JSON.

## Licence

EUPL-1.2

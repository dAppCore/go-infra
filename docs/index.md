---
title: go-infra
description: Infrastructure provider API clients and YAML-based configuration for managing production environments.
---

# go-infra

`forge.lthn.ai/core/go-infra` provides typed Go clients for infrastructure provider APIs (Hetzner Cloud, Hetzner Robot, CloudNS) and a declarative YAML configuration layer for describing production topology. It also ships CLI commands for production management (`core prod`) and security monitoring (`core monitor`).

The library has no framework dependencies beyond the Go standard library, YAML parsing, and testify for tests. All HTTP communication goes through a shared `APIClient` that handles retries, exponential backoff, and rate-limit compliance automatically.

## Module Path

```
forge.lthn.ai/core/go-infra
```

Requires **Go 1.26+**.

## Quick Start

### Using the API Clients Directly

```go
import "forge.lthn.ai/core/go-infra"

// Hetzner Cloud -- list all servers
hc := infra.NewHCloudClient(os.Getenv("HCLOUD_TOKEN"))
servers, err := hc.ListServers(ctx)

// Hetzner Robot -- list dedicated servers
hr := infra.NewHRobotClient(user, password)
dedicated, err := hr.ListServers(ctx)

// CloudNS -- ensure a DNS record exists
dns := infra.NewCloudNSClient(authID, authPassword)
changed, err := dns.EnsureRecord(ctx, "example.com", "www", "A", "1.2.3.4", 300)
```

### Loading Infrastructure Configuration

```go
import "forge.lthn.ai/core/go-infra"

// Auto-discover infra.yaml by walking up from the current directory
cfg, path, err := infra.Discover(".")

// Or load a specific file
cfg, err := infra.Load("/path/to/infra.yaml")

// Query the configuration
appServers := cfg.AppServers()
for name, host := range appServers {
    fmt.Printf("%s: %s (%s)\n", name, host.IP, host.Role)
}
```

### CLI Commands

When registered with the `core` CLI binary, go-infra provides two command groups:

```bash
# Production infrastructure management
core prod status              # Health check all hosts, services, and load balancer
core prod setup               # Phase 1 foundation: discover topology, create LB, configure DNS
core prod setup --dry-run     # Preview what setup would do
core prod setup --step=dns    # Run a single setup step
core prod dns list            # List DNS records for a zone
core prod dns set www A 1.2.3.4  # Create or update a DNS record
core prod lb status           # Show load balancer status and target health
core prod lb create           # Create load balancer from infra.yaml
core prod ssh noc             # SSH into a named host

# Security monitoring (aggregates GitHub Security findings)
core monitor                  # Scan current repo
core monitor --all            # Scan all repos in registry
core monitor --repo core-php  # Scan a specific repo
core monitor --severity high  # Filter by severity
core monitor --json           # JSON output
```

## Package Layout

| Path | Description |
|------|-------------|
| `client.go` | Shared HTTP API client with retry, exponential backoff, and rate-limit handling |
| `config.go` | YAML infrastructure configuration parser and typed config structs |
| `hetzner.go` | Hetzner Cloud API (servers, load balancers, snapshots) and Hetzner Robot API (dedicated servers) |
| `cloudns.go` | CloudNS DNS API (zones, records, ACME challenge helpers) |
| `cmd/prod/` | CLI commands for production infrastructure management (`core prod`) |
| `cmd/monitor/` | CLI commands for security finding aggregation (`core monitor`) |

## Dependencies

### Direct

| Module | Purpose |
|--------|---------|
| `forge.lthn.ai/core/cli` | CLI framework (cobra-based command registration) |
| `forge.lthn.ai/core/go-ansible` | SSH client used by `core prod status` for host health checks |
| `forge.lthn.ai/core/go-i18n` | Internationalisation strings for monitor command |
| `forge.lthn.ai/core/go-io` | Filesystem abstraction used by monitor's registry lookup |
| `forge.lthn.ai/core/go-log` | Structured error logging |
| `forge.lthn.ai/core/go-scm` | Repository registry for multi-repo monitoring |
| `gopkg.in/yaml.v3` | YAML parsing for `infra.yaml` |
| `github.com/stretchr/testify` | Test assertions |

The core library types (`config.go`, `client.go`, `hetzner.go`, `cloudns.go`) only depend on the standard library and `gopkg.in/yaml.v3`. The heavier dependencies (`cli`, `go-ansible`, `go-scm`, etc.) are confined to the `cmd/` packages.

## Environment Variables

| Variable | Used by | Description |
|----------|---------|-------------|
| `HCLOUD_TOKEN` | `prod setup`, `prod status`, `prod lb` | Hetzner Cloud API bearer token |
| `HETZNER_ROBOT_USER` | `prod setup` | Hetzner Robot API username |
| `HETZNER_ROBOT_PASS` | `prod setup` | Hetzner Robot API password |
| `CLOUDNS_AUTH_ID` | `prod setup`, `prod dns` | CloudNS sub-auth user ID |
| `CLOUDNS_AUTH_PASSWORD` | `prod setup`, `prod dns` | CloudNS auth password |

## Licence

EUPL-1.2

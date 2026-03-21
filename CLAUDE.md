# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`cisco_exporter` is a Prometheus exporter that collects metrics from Cisco network devices (IOS, IOS XE, NX-OS, IOS XR) via SSH, exposing them on HTTP port 9362 (default).

## Build & Run

```bash
# Build static binary
CGO_ENABLED=0 go build -a -installsuffix cgo -o cisco_exporter

# Run with CLI flags
./cisco_exporter -ssh.targets="host1,host2:2233" -ssh.keyfile=/path/to/key

# Run with config file
./cisco_exporter -config.file=config.yml

# Cross-platform release builds
goreleaser build --snapshot
```

## Testing & Linting

```bash
go test ./...           # Run all tests (no existing tests in repo)
go test ./package/...   # Run tests in a specific package
go vet ./...            # Static analysis
```

## Architecture

### Data Flow

```
HTTP /metrics request
  → cisco_collector.go (Prometheus Collect())
    → devices.go (iterate configured devices, parallel goroutines)
      → connector/connection.go (establish SSH session)
        → rpc/rpc_client.go (detect OS type, run commands)
          → bgp/, environment/, facts/, interfaces/, optics/ (parse output → Prometheus metrics)
```

### Key Packages

- **`main.go`** — Entry point; parses flags, loads config, starts HTTP server on `:9362`
- **`config/config.go`** — `Config`, `DeviceConfig`, `FeatureConfig` structs; YAML/flag parsing
- **`connector/`** — SSH connection management; `connection.go` handles auth, terminal setup, command I/O; reads until `#` prompt
- **`rpc/rpc_client.go`** — Wraps SSH connection; detects OS type from `show version` output (`IOSXE`, `NXOS`, `IOS`, `IOSXR`)
- **`collector/rpc_collector.go`** — `RPCCollector` interface all feature collectors must implement
- **`cisco_collector.go`** — Top-level Prometheus `Collector`; fans out to devices concurrently
- **`collectors.go`** — Registry mapping OS type to enabled feature collectors

### Collector Interface

Every feature collector (`bgp`, `environment`, `facts`, `interfaces`, `optics`) implements:

```go
type RPCCollector interface {
    Name() string
    Describe(ch chan<- *prometheus.Desc)
    Collect(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error
}
```

Each collector sends OS-appropriate SSH commands and parses CLI output into Prometheus metrics. Parsing is plain-text (no structured data format from devices).

### OS-Specific Behavior

Collectors branch on `client.OSType` to use different commands and parsers per OS. When adding support for a new OS or command, follow the existing pattern in any collector (e.g., `interfaces/interfaces_collector.go`).

### Configuration Priority

Device-level config overrides global config. Both YAML file and CLI flags are supported; see `config.yml.example` for the full schema.

## Module

`github.com/lwlcom/cisco_exporter` — Go 1.16, produces a fully static binary.

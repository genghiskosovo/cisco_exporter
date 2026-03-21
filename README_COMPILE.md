# Compiling cisco_exporter

## Prerequisites

Install Go 1.16 or newer: https://go.dev/dl/

## Build for Linux (from any OS including macOS)

**x86_64 server (most common):**
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o cisco_exporter .
```

**ARM64 server (AWS Graviton, Raspberry Pi 4, etc.):**
```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o cisco_exporter .
```

**32-bit ARM (Raspberry Pi 2/3):**
```bash
GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o cisco_exporter .
```

## Deploy to server

Copy the binary and your config file:
```bash
scp cisco_exporter user@server:/usr/local/bin/
scp config.yml user@server:/etc/cisco_exporter/
```

Run it:
```bash
cisco_exporter -config.file=/etc/cisco_exporter/config.yml
```

## Notes

- `CGO_ENABLED=0` produces a fully static binary — no runtime dependencies on the target server.
- The binary runs directly, no Go installation needed on the server.
- Default metrics port is `:9362`. Make sure it is reachable by your Prometheus instance.

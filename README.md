# SATVOS Tally Connector

On-premise agent that syncs SATVOS Cloud with Tally Prime. Runs as a Windows service or standalone process, periodically pushing master data (ledgers, stock items, godowns, units, cost centres) to SATVOS and importing approved purchase vouchers back into Tally.

## Overview

The connector sits on the same machine as Tally Prime and acts as a bridge:

1. **Tally to SATVOS** -- Reads master data from Tally via its XML HTTP API and pushes it to SATVOS Cloud.
2. **SATVOS to Tally** -- Pulls approved purchase vouchers from SATVOS, converts them to Tally XML, and imports them.
3. **Heartbeat** -- Reports connectivity status, Tally company name, and agent version on every sync cycle.
4. **Local UI** -- Serves a web dashboard at `http://localhost:8321` for setup and status monitoring.

## Quick Start

### Prerequisites

- Tally Prime running with its XML server enabled (default port 9000)
- A SATVOS service account API key (`sk_...`)

### Windows (Recommended)

1. Download the latest release or build from source:
   ```
   make build-windows
   ```

2. Run the installer (as Administrator):
   ```powershell
   powershell -ExecutionPolicy Bypass -File scripts\install.ps1
   ```

3. The installer will:
   - Copy the binary to `%APPDATA%\satvos-connector\`
   - Register and start a Windows service
   - Open the setup page at `http://localhost:8321/setup`

4. Enter your API key in the setup page, or create a config file at `%APPDATA%\satvos-connector\connector.yaml`.

### Linux / macOS (Development)

```bash
export CONNECTOR_SATVOS_API_KEY="sk_your_key_here"
make run
```

## Configuration

Configuration is loaded from (in order of precedence):

1. **Environment variables** with `CONNECTOR_` prefix
2. **Config file** `connector.yaml` (searched in `.` and `./configs`)

### Config File Reference

```yaml
satvos:
  base_url: "https://api.satvos.com"
  api_key: "sk_your_api_key_here"

tally:
  host: "localhost"
  port: 0          # 0 = auto-discover (scans ports 9000-9010)
  company: ""      # empty = auto-detect from Tally

sync:
  interval_seconds: 30   # minimum 5
  batch_size: 50          # 1-100
  retry_attempts: 3       # minimum 1

ui:
  port: 8321
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONNECTOR_SATVOS_BASE_URL` | `https://api.satvos.com` | SATVOS API base URL |
| `CONNECTOR_SATVOS_API_KEY` | (required) | Service account API key |
| `CONNECTOR_TALLY_HOST` | `localhost` | Tally XML server host |
| `CONNECTOR_TALLY_PORT` | `0` (auto-discover) | Tally XML server port |
| `CONNECTOR_TALLY_COMPANY` | (auto-detect) | Tally company name |
| `CONNECTOR_SYNC_INTERVAL_SECONDS` | `30` | Seconds between sync cycles |
| `CONNECTOR_SYNC_BATCH_SIZE` | `50` | Max outbound items per pull |
| `CONNECTOR_SYNC_RETRY_ATTEMPTS` | `3` | Retry attempts for failed operations |
| `CONNECTOR_UI_PORT` | `8321` | Local web UI port |

## Architecture

```
                    +-------------------+
                    |   SATVOS Cloud    |
                    +--------+----------+
                             |
                     HTTPS (JSON API)
                             |
+----------------------------+----------------------------+
|                    Connector (this)                      |
|                                                         |
|  +----------+    +-----------+    +------------------+  |
|  | Cloud    |--->| Sync      |--->| Tally Client     |  |
|  | Client   |<---| Engine    |<---| (XML over HTTP)  |  |
|  +----------+    +-----+-----+    +------------------+  |
|                        |                                |
|                  +-----+-----+                          |
|                  | Local     |    +------------------+  |
|                  | Store     |    | Web UI Server    |  |
|                  | (JSON)    |    | (localhost:8321)  |  |
|                  +-----------+    +------------------+  |
+----------------------------+----------------------------+
                             |
                      XML over HTTP
                             |
                    +--------+----------+
                    |    Tally Prime     |
                    +-------------------+
```

### Sync Cycle

Each cycle (default every 30 seconds):

1. **Heartbeat** -- POST status to SATVOS (`/sync/v1/heartbeat`)
2. **Push masters** -- Read ledgers, stock items, godowns, units, cost centres from Tally and POST to SATVOS (`/sync/v1/masters`)
3. **Pull outbound** -- GET pending vouchers from SATVOS (`/sync/v1/outbound`), convert to Tally XML, import via Tally's XML API, and ACK results (`/sync/v1/ack`)

### Package Layout

```
cmd/connector/main.go     Entry point, wiring, graceful shutdown
internal/
  config/                  Viper-based configuration (YAML + env vars)
  cloud/                   SATVOS Cloud API client (register, heartbeat, masters, outbound, ack)
  tally/                   Tally Prime XML client (company info, ledgers, stock items, import)
  convert/                 VoucherDef-to-Tally XML converter
  sync/                    Sync engine (orchestrates cloud <-> tally data flow)
  store/                   JSON file-based local state (cursors, discovered port, agent ID)
  ui/                      Embedded web UI server (status dashboard, setup page)
  svc/                     Windows service integration (SCM handler)
scripts/
  install.ps1              Windows service installer (PowerShell)
configs/
  connector.example.yaml   Example configuration file
```

## Development

### Prerequisites

- Go 1.24+
- Make

### Running

```bash
# Set required config
export CONNECTOR_SATVOS_API_KEY="sk_test_key"

# Run directly
make run

# Or build and run
make build
./bin/satvos-connector
```

### Project Structure

The project follows a straightforward layered architecture:

- **config** loads and validates settings from YAML files and environment variables
- **tally** handles all communication with Tally Prime's XML HTTP API
- **cloud** handles all communication with the SATVOS Cloud REST API
- **convert** transforms SATVOS VoucherDef payloads into Tally-compatible XML
- **sync** orchestrates the periodic data exchange between cloud and tally
- **store** persists local state (sync cursors, discovered port) to a JSON file
- **ui** serves an embedded web dashboard for monitoring and setup
- **svc** provides Windows Service Control Manager integration

## Building

```bash
# Linux / macOS
make build

# Windows (cross-compile)
make build-windows

# Clean build artifacts
make clean
```

The binary is output to `bin/satvos-connector` (or `bin/satvos-connector.exe` for Windows).

## Testing

```bash
# Run all tests with race detector
make test

# Run with verbose output
go test ./... -v -count=1 -race

# Run tests for a specific package
go test ./internal/tally/... -v

# Lint
make lint
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `satvos.api_key is required` | Missing API key | Set `CONNECTOR_SATVOS_API_KEY` or add to config file |
| `no Tally instance found` | Tally not running or port mismatch | Start Tally Prime with XML server enabled, or set `CONNECTOR_TALLY_PORT` |
| `registration failed` | Invalid API key or network issue | Verify API key and network connectivity to SATVOS |
| Service won't start | Port conflict or permissions | Check Event Viewer for errors, ensure port 8321 is free |

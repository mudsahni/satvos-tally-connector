# CLAUDE.md — Project Context for Claude Code

## Project Overview

SATVOS Tally Connector is an on-premise Windows service (or standalone agent) that provides bidirectional sync between SATVOS Cloud and Tally Prime ERP. It runs on the user's machine alongside Tally, communicating with the SATVOS backend over HTTPS and with Tally via its local XML HTTP API. The connector discovers Tally's port, registers with SATVOS, pushes master data (ledgers, stock items, godowns, units, cost centres), pulls outbound documents, converts them to Tally voucher XML, and imports them into Tally.

## Key Commands

```bash
make build            # Build native binary to bin/satvos-connector
make build-windows    # Cross-compile Windows binary to bin/satvos-connector.exe
make run              # Run the connector (go run ./cmd/connector)
make test             # Run all tests (go test ./... -v -count=1 -race)
make lint             # Run golangci-lint
make clean            # Remove bin/
```

## Architecture & Code Layout

```
cmd/connector/main.go        Entry point — config, state, discovery, registration, engine+UI startup

internal/
  config/config.go            Viper-based config loading (YAML + env vars, CONNECTOR_ prefix)
  cloud/
    client.go                 SATVOS Cloud REST API client (Register, Heartbeat, PushMasters,
                              PullOutbound, Ack, PushInbound)
    types.go                  All request/response DTOs for cloud API
  tally/
    client.go                 Tally XML HTTP client (SendRequest, IsAvailable, CheckStatus)
    discover.go               Port scan discovery (9000–9010) via GetCompanyInfo probes
    health.go                 GetCompanyInfo, GetLedgers, GetStockItems, GetGodowns, GetUnits, GetCostCentres
    import.go                 ImportVoucher, ImportMaster, ParseImportResponse (CREATED/ALTERED/EXCEPTIONS/LINEERROR)
    masters.go                EnsureLedgersExist, BuildLedgerXML (rich: PAN, GSTIN, address, TDS), deducteeTypeFromPAN
    requests.go               XML request builders (company info, master export, voucher/master import with company name)
    responses.go              XML response types and parsers for each master type
  convert/
    types.go                  VoucherDef, TaxEntry, InventoryItem, PartyDetail structs
    template.go               text/template for Tally voucher XML (3 modes: accounting_invoice, item_invoice, journal)
    xml.go                    ToXML() — VoucherDef to Tally XML with mode routing, REFERENCE, BILLALLOCATIONS
  xmlutil/
    escape.go                 Shared XML escaping function (used by convert, tally packages)
  sync/
    engine.go                 Sync engine: ticker loop, runCycle (heartbeat → masters → outbound → ack)
  store/
    local.go                  JSON file-backed local state (OutboundCursor, TallyPort, TallyCompany, AgentID)
  svc/
    console.go                Non-Windows stub (IsWindowsService=false, Run=passthrough)
    windows.go                Windows SCM handler (golang.org/x/sys/windows/svc)
  ui/
    server.go                 Embedded HTTP server (127.0.0.1:8321) with static dashboard
    handlers.go               GET /api/status, POST /api/sync
    static/index.html         Dashboard — connection status, "Sync Now" button
    static/setup.html         Setup wizard — API key input, Tally connection check

scripts/
  install.ps1                 PowerShell Windows service installer (requires Admin)
configs/
  connector.example.yaml      Reference config file
```

## Configuration

Config is loaded via Viper. Precedence: env vars > config file > defaults.

**Env prefix:** `CONNECTOR_` (with `.` → `_` replacer, e.g., `CONNECTOR_SATVOS_API_KEY`)

**Config file search paths:** `%APPDATA%\satvos-connector\`, `~/.satvos-connector/`, `.`, `./configs`

| Key | Default | Notes |
|-----|---------|-------|
| `satvos.base_url` | `https://api.satvos.com` | SATVOS API base URL |
| `satvos.api_key` | *(required)* | Service account API key (`sk_...`) |
| `tally.host` | `localhost` | Tally host |
| `tally.port` | `0` | 0 = auto-discover (scans 9000–9010) |
| `tally.company` | `""` | Empty = auto-detect from Tally |
| `sync.interval_seconds` | `30` | Min 5, clamped |
| `sync.batch_size` | `50` | Clamped 1–100 |
| `sync.retry_attempts` | `3` | Min 1 (config defined but not yet used in engine) |
| `ui.port` | `8321` | Local dashboard port |

## Sync Cycle

The sync engine runs on a timer (`sync.interval_seconds`). Each cycle:

```
1. Check Tally availability (IsAvailable)
2. Heartbeat → SATVOS (always, even if Tally down; reports tally_connected status)
3. If Tally unavailable → return early
4. GetCompanyInfo → update local state
5. pushMasters:
   a. Fetch all 5 master types from Tally (ledgers, stock items, godowns, units, cost centres)
   b. Map to cloud DTOs → POST /api/v1/sync/v1/masters
   c. Per-fetch errors logged but don't abort remaining fetches
6. processOutbound:
   a. PullOutbound(cursor, batchSize) from SATVOS
   b. Collect all unique ledgers referenced across vouchers (party, purchase, tax)
   c. EnsureLedgersExist — pre-creates missing ledgers with rich details (PAN, GSTIN, address, TDS, state). Uses DUPIGNORECOMBINE to skip existing. Party details come from VoucherDef.PartyDetails
   d. For each item: unmarshal VoucherDef → convert.ToXML (routes to accounting_invoice/item_invoice/journal mode) → tally.ImportVoucher (with company name)
   e. Collect AckResults (success/failure per item, includes TallyVoucherID from LASTVCHID)
   f. Advance cursor in local state
   g. POST /api/v1/sync/v1/ack with results
```

## Cloud API Endpoints Used

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/sync/v1/register` | Register agent, get ID |
| POST | `/api/v1/sync/v1/heartbeat` | Report status |
| POST | `/api/v1/sync/v1/masters` | Upload Tally masters |
| GET | `/api/v1/sync/v1/outbound` | Fetch pending vouchers (cursor-paginated) |
| POST | `/api/v1/sync/v1/ack` | Report import results |
| POST | `/api/v1/sync/v1/inbound` | Push Tally-originated vouchers (wired, not yet called) |

All requests use `Authorization: Bearer <api_key>`.

## Key Conventions

- **Error resilience**: Each master fetch failure is logged but doesn't abort the cycle. Per-item import failures produce failed AckResults; processing continues for remaining items. Ledger pre-creation failures are warnings (voucher imports continue — some may succeed if ledgers already exist)
- **Voucher modes**: `VoucherMode` field routes to different Tally XML structures. `accounting_invoice` = Purchase type without inventory (service invoices). `item_invoice` = Purchase type with ALLINVENTORYENTRIES. `journal` = Journal type for non-GST expenses. Unknown modes return an error
- **Rich ledger creation**: Party ledgers created with PAN (INCOMETAXNUMBER), GSTIN (PARTYGSTIN), address (ADDRESS.LIST), TDS applicability (deductee type derived from PAN 4th character: C=Company, F=Firm, T=Trust, H=HUF, P=Individual, A=AOP/BOI), GST registration type (defaults to "Regular")
- **Supplier invoice reference**: Vouchers include REFERENCE (invoice number) and REFERENCEDATE (invoice date) tags, plus BILLALLOCATIONS.LIST for Tally's bill-wise tracking
- **XML safety**: All user-derived values (company name, ledger names, addresses) are XML-escaped via `xmlutil.Escape()` before interpolation
- **Import response parsing**: Handles both flat `<RESPONSE>` and envelope `<ENVELOPE>` formats. Captures CREATED, ALTERED, EXCEPTIONS, ERRORS, LASTVCHID, LINEERROR. `IsZeroCountOnly` flag distinguishes "nothing happened" from real errors (important for DUPIGNORECOMBINE master imports)
- **Cursor pagination**: `OutboundCursor` persisted in local state; enables resumable batch processing across restarts
- **Port discovery caching**: Discovered Tally port is cached in `state.json`; reused on next startup before re-scanning
- **Windows service detection**: `svc.IsWindowsService()` switches between SCM-managed and standalone modes at startup
- **JSON state persistence**: `state.json` stored in `%APPDATA%\satvos-connector\` (Windows) or `~/.satvos-connector/` (Linux/macOS). Thread-safe via `sync.RWMutex`, atomic writes via temp file + rename
- **Local-only UI binding**: Dashboard binds to `127.0.0.1` only (not `0.0.0.0`)
- **text/template for XML**: Tally's non-standard element names (e.g., `ALLLEDGERENTRIES.LIST`) prevent use of `encoding/xml` struct tags
- **Amount sign convention**: Party ledger = positive (credit), purchase/tax/inventory = negative (debit)

## Tech Stack

Go 1.25, Viper (config), testify (testing), x/sync (errgroup), x/sys (Windows SCM). No database, no web framework — standard library `net/http`, `encoding/xml`, `encoding/json`, `text/template`, `embed`.

## Important Files for Common Tasks

- **Adding a sync step**: `sync/engine.go` (`runCycle` method) — add step after `pushMasters`/`processOutbound`
- **Modifying voucher conversion**: `convert/xml.go` (mode routing + logic), `convert/template.go` (XML template with conditionals), `convert/types.go` (VoucherDef, PartyDetail)
- **Modifying ledger creation**: `tally/masters.go` (LedgerDef, BuildLedgerXML template, EnsureLedgersExist, deducteeTypeFromPAN)
- **Adding XML-escaped fields**: Use `xmlutil.Escape()` from `internal/xmlutil/escape.go` — single source for all XML escaping
- **Adding a Tally master type**: `tally/requests.go` (XML request builder), `tally/responses.go` (response parser + type), `tally/health.go` (client method), `cloud/types.go` (DTO), `sync/engine.go` (`pushMasters`)
- **Changing config**: `config/config.go` — add field to struct, set default, add validation
- **Modifying cloud API calls**: `cloud/client.go` (methods), `cloud/types.go` (DTOs)
- **Updating UI dashboard**: `ui/static/index.html` (dashboard), `ui/static/setup.html` (setup wizard), `ui/handlers.go` (API endpoints)
- **Modifying Windows service**: `svc/windows.go` (SCM handler), `scripts/install.ps1` (installer)

## Gotchas

- **Version is hardcoded**: `const version = "0.1.0"` in `cmd/connector/main.go:24` — no build-time injection yet
- **text/template for XML**: Uses `text/template` (not `encoding/xml`) due to Tally's non-standard element names. Shared `xmlutil.Escape()` handles `&<>"'` (used by convert, tally, and requests packages)
- **Port scan range**: Discovery scans ports 9000–9010 only, with 2s timeout per port
- **state.json contains secrets**: Agent ID stored in plaintext JSON. File created with `0600` permissions
- **Windows service name**: `"SATVOSTallyConnector"` — hardcoded in `svc/windows.go` and `scripts/install.ps1`
- **install.ps1 requires Administrator**: Uses `New-Service` and `Start-Service` PowerShell cmdlets
- **CONNECTOR_ prefix for env vars**: Not `SATVOS_` like the server — uses `CONNECTOR_SATVOS_API_KEY` etc.
- **retry_attempts config defined but unused**: The config field exists and is validated, but `processOutbound` and `pushMasters` don't implement retry loops
- **Inbound (Tally→SATVOS) not yet wired**: `PushInbound` is defined in cloud client and types but not called from the sync engine
- **Registration failure is non-fatal**: Agent starts and syncs even if initial registration fails; retries implicitly via heartbeat
- **No mocking framework**: Tests use `httptest.NewServer` with custom handlers to mock both cloud and Tally endpoints

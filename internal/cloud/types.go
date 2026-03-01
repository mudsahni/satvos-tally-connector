package cloud

import "encoding/json"

// APIResponse is the standard SATVOS API response envelope.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *APIError       `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RegisterRequest is sent to POST /sync/v1/register
type RegisterRequest struct {
	Version string `json:"version"`
	OSInfo  string `json:"os_info"`
}

type RegisterResponse struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

// HeartbeatRequest is sent to POST /sync/v1/heartbeat
type HeartbeatRequest struct {
	TallyConnected bool     `json:"tally_connected"`
	TallyCompany   string   `json:"tally_company"`
	TallyPort      int      `json:"tally_port"`
	Version        string   `json:"version"`
	Errors         []string `json:"errors,omitempty"`
}

// MasterPayload is sent to POST /sync/v1/masters
type MasterPayload struct {
	Ledgers     []MasterLedger     `json:"ledgers,omitempty"`
	StockItems  []MasterStockItem  `json:"stock_items,omitempty"`
	Godowns     []MasterGodown     `json:"godowns,omitempty"`
	Units       []MasterUnit       `json:"units,omitempty"`
	CostCentres []MasterCostCentre `json:"cost_centres,omitempty"` //nolint:misspell // Tally uses British spelling
}

type MasterLedger struct {
	Name        string  `json:"name"`
	ParentGroup string  `json:"parent_group"`
	GSTIN       string  `json:"gstin"`
	State       string  `json:"state"`
	TaxType     string  `json:"tax_type"`
	TaxRate     float64 `json:"tax_rate"`
	IsRevenue   bool    `json:"is_revenue"`
}

type MasterStockItem struct {
	Name        string `json:"name"`
	ParentGroup string `json:"parent_group"`
	HSNCode     string `json:"hsn_code"`
	DefaultUOM  string `json:"default_uom"`
}

type MasterGodown struct {
	Name   string `json:"name"`
	Parent string `json:"parent"`
}

type MasterUnit struct {
	Symbol     string `json:"symbol"`
	FormalName string `json:"formal_name"`
}

type MasterCostCentre struct {
	Name   string `json:"name"`
	Parent string `json:"parent"`
}

// OutboundResponse is returned by GET /sync/v1/outbound
type OutboundResponse struct {
	Items      []OutboundItem `json:"items"`
	NextCursor string         `json:"next_cursor"`
}

type OutboundItem struct {
	DocumentID     string          `json:"document_id"`
	StructuredData json.RawMessage `json:"structured_data"`
	VoucherDef     json.RawMessage `json:"voucher_def"`
	SyncEventID    string          `json:"sync_event_id"`
}

// AckRequest is sent to POST /sync/v1/ack
type AckRequest struct {
	Results []AckResult `json:"results"`
}

type AckResult struct {
	SyncEventID        string `json:"sync_event_id"`
	DocumentID         string `json:"document_id"`
	Success            bool   `json:"success"`
	TallyVoucherID     string `json:"tally_voucher_id,omitempty"`
	TallyVoucherNumber string `json:"tally_voucher_number,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
}

// InboundRequest is sent to POST /sync/v1/inbound
type InboundRequest struct {
	Vouchers []InboundVoucher `json:"vouchers"`
}

type InboundVoucher struct {
	VoucherType   string          `json:"voucher_type"`
	VoucherNumber string          `json:"voucher_number"`
	VoucherDate   string          `json:"voucher_date"`
	PartyName     string          `json:"party_name"`
	PartyGSTIN    string          `json:"party_gstin"`
	Amount        float64         `json:"amount"`
	Narration     string          `json:"narration"`
	LedgerEntries json.RawMessage `json:"ledger_entries"`
	TallyMasterID string          `json:"tally_master_id"`
}

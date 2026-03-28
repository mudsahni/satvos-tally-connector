package convert

// VoucherDef is the smart-matched voucher definition from the SATVOS server.
type VoucherDef struct {
	DocumentID     string          `json:"document_id"`
	VoucherType    string          `json:"voucher_type"`
	VoucherDate    string          `json:"voucher_date"`
	PartyLedger    string          `json:"party_ledger"`
	PurchaseLedger string          `json:"purchase_ledger"`
	TaxEntries     []TaxEntry      `json:"tax_entries"`
	InventoryItems []InventoryItem `json:"inventory_items"`
	TotalAmount    float64         `json:"total_amount"`
	Narration      string          `json:"narration"`
	RemoteID            string          `json:"remote_id"`
	VoucherMode         string          `json:"voucher_mode"`
	SupplierInvoiceNo   string          `json:"supplier_invoice_no"`
	SupplierInvoiceDate string          `json:"supplier_invoice_date"`
	PartyDetails        *PartyDetail    `json:"party_details,omitempty"`
}

type TaxEntry struct {
	LedgerName string  `json:"ledger_name"`
	Amount     float64 `json:"amount"`
}

type InventoryItem struct {
	StockItem string  `json:"stock_item"`
	Quantity  float64 `json:"quantity"`
	Rate      float64 `json:"rate"`
	Amount    float64 `json:"amount"`
	UOM       string  `json:"uom"`
	Godown    string  `json:"godown"`
	HSNCode   string  `json:"hsn_code"`
}

// PartyDetail holds rich party information for ledger creation in Tally.
type PartyDetail struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	PAN       string `json:"pan"`
	GSTIN     string `json:"gstin"`
	State     string `json:"state"`
}

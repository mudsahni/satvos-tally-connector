package convert

import (
	"bytes"
	"fmt"
	"strings"
)

// templateData is the data structure passed to the XML template.
type templateData struct {
	RemoteID            string
	VoucherTypeName     string // "Purchase" or "Journal"
	IsInvoice           string // "Yes" or "No"
	PersistedView       string // "Invoice Voucher View" or "Accounting Voucher View"
	TallyDate           string // YYYYMMDD format
	Narration           string
	PartyLedger         string
	PurchaseLedger      string
	TotalAmount         float64
	PurchaseAmount      float64
	TaxEntries          []TaxEntry
	InventoryItems      []InventoryItem
	VoucherMode         string
	SupplierInvoiceNo   string
	SupplierInvoiceDate string // YYYYMMDD format
}

// ToXML converts a VoucherDef to Tally-importable XML.
func ToXML(def *VoucherDef) (string, error) {
	if def == nil {
		return "", fmt.Errorf("nil voucher definition")
	}

	// Convert date from YYYY-MM-DD to YYYYMMDD.
	// Dates are expected in YYYY-MM-DD format from the backend.
	tallyDate := strings.ReplaceAll(def.VoucherDate, "-", "")

	// Determine voucher mode, defaulting to accounting_invoice
	voucherMode := def.VoucherMode
	if voucherMode == "" {
		voucherMode = "accounting_invoice"
	}

	switch voucherMode {
	case "accounting_invoice", "item_invoice", "journal":
		// valid
	default:
		return "", fmt.Errorf("unrecognized voucher mode: %q", voucherMode)
	}

	// Use VoucherType from definition, with sensible defaults.
	voucherTypeName := def.VoucherType
	if voucherTypeName == "" {
		voucherTypeName = "Purchase"
	}

	isInvoice := "Yes"
	persistedView := "Invoice Voucher View"
	if voucherMode == "journal" {
		voucherTypeName = "Journal"
		isInvoice = "No"
		persistedView = "Accounting Voucher View"
	}

	// Calculate purchase amount = total - sum(tax).
	// In journal mode, tax entries are suppressed in the template, so
	// purchaseAmount must equal TotalAmount to keep debit == credit.
	taxTotal := 0.0
	if voucherMode != "journal" {
		for _, t := range def.TaxEntries {
			taxTotal += t.Amount
		}
	}
	purchaseAmount := def.TotalAmount - taxTotal

	// Convert supplier invoice date from YYYY-MM-DD to YYYYMMDD
	supplierDate := strings.ReplaceAll(def.SupplierInvoiceDate, "-", "")

	data := templateData{
		RemoteID:            def.RemoteID,
		VoucherTypeName:     voucherTypeName,
		IsInvoice:           isInvoice,
		PersistedView:       persistedView,
		TallyDate:           tallyDate,
		Narration:           def.Narration,
		PartyLedger:         def.PartyLedger,
		PurchaseLedger:      def.PurchaseLedger,
		TotalAmount:         def.TotalAmount,
		PurchaseAmount:      purchaseAmount,
		TaxEntries:          def.TaxEntries,
		InventoryItems:      def.InventoryItems,
		VoucherMode:         voucherMode,
		SupplierInvoiceNo:   def.SupplierInvoiceNo,
		SupplierInvoiceDate: supplierDate,
	}

	var buf bytes.Buffer
	if err := voucherTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

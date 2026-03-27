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

	// Calculate purchase amount = total - sum(tax)
	taxTotal := 0.0
	for _, t := range def.TaxEntries {
		taxTotal += t.Amount
	}
	purchaseAmount := def.TotalAmount - taxTotal

	// Convert date from YYYY-MM-DD to YYYYMMDD
	tallyDate := strings.ReplaceAll(def.VoucherDate, "-", "")

	// Determine voucher mode, defaulting to accounting_invoice
	voucherMode := def.VoucherMode
	if voucherMode == "" {
		voucherMode = "accounting_invoice"
	}

	// Derive type-specific fields from voucher mode
	voucherTypeName := "Purchase"
	isInvoice := "Yes"
	persistedView := "Invoice Voucher View"
	if voucherMode == "journal" {
		voucherTypeName = "Journal"
		isInvoice = "No"
		persistedView = "Accounting Voucher View"
	}

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

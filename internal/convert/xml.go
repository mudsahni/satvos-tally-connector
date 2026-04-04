package convert

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
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

// roundAmount rounds a float64 to 2 decimal places.
func roundAmount(f float64) float64 {
	return math.Round(f*100) / 100
}

// ToXML converts a VoucherDef to Tally-importable XML.
func ToXML(def *VoucherDef) (string, error) {
	if def == nil {
		return "", fmt.Errorf("nil voucher definition")
	}

	// Validate and convert date from YYYY-MM-DD to YYYYMMDD.
	if def.VoucherDate == "" {
		return "", fmt.Errorf("VoucherDate is required")
	}
	if _, err := time.Parse("2006-01-02", def.VoucherDate); err != nil {
		return "", fmt.Errorf("invalid VoucherDate %q: expected YYYY-MM-DD format", def.VoucherDate)
	}
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

	// Round total amount to 2 decimal places.
	totalAmount := roundAmount(def.TotalAmount)

	// Calculate purchase amount = total - sum(tax).
	// In journal mode, tax entries are suppressed in the template, so
	// purchaseAmount must equal TotalAmount to keep debit == credit.
	taxTotal := 0.0
	// Copy tax entries to avoid mutating the caller's struct.
	roundedTaxEntries := make([]TaxEntry, len(def.TaxEntries))
	copy(roundedTaxEntries, def.TaxEntries)
	if voucherMode != "journal" {
		for i := range roundedTaxEntries {
			roundedTaxEntries[i].Amount = roundAmount(roundedTaxEntries[i].Amount)
			taxTotal += roundedTaxEntries[i].Amount
		}
	}
	purchaseAmount := roundAmount(totalAmount - taxTotal)
	// Balance is guaranteed by construction: purchaseAmount absorbs all rounding
	// residual since it is defined as totalAmount - taxTotal.

	// Copy inventory items to avoid mutating the caller's struct.
	roundedItems := make([]InventoryItem, len(def.InventoryItems))
	copy(roundedItems, def.InventoryItems)
	for i := range roundedItems {
		roundedItems[i].Amount = roundAmount(roundedItems[i].Amount)
	}

	// Convert supplier invoice date from YYYY-MM-DD to YYYYMMDD.
	// Allow empty (optional), but validate format if set.
	supplierDate := ""
	if def.SupplierInvoiceDate != "" {
		if _, err := time.Parse("2006-01-02", def.SupplierInvoiceDate); err != nil {
			log.Printf("[convert] WARNING: invalid SupplierInvoiceDate %q, skipping", def.SupplierInvoiceDate)
			supplierDate = ""
		} else {
			supplierDate = strings.ReplaceAll(def.SupplierInvoiceDate, "-", "")
		}
	}

	data := templateData{
		RemoteID:            def.RemoteID,
		VoucherTypeName:     voucherTypeName,
		IsInvoice:           isInvoice,
		PersistedView:       persistedView,
		TallyDate:           tallyDate,
		Narration:           def.Narration,
		PartyLedger:         def.PartyLedger,
		PurchaseLedger:      def.PurchaseLedger,
		TotalAmount:         totalAmount,
		PurchaseAmount:      purchaseAmount,
		TaxEntries:          roundedTaxEntries,
		InventoryItems:      roundedItems,
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

package convert

import (
	"bytes"
	"fmt"
	"strings"
)

// templateData is the data structure passed to the XML template.
type templateData struct {
	RemoteID       string
	VoucherType    string
	TallyDate      string // YYYYMMDD format
	Narration      string
	PartyLedger    string
	PurchaseLedger string
	TotalAmount    float64
	PurchaseAmount float64
	TaxEntries     []TaxEntry
	InventoryItems []InventoryItem
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

	data := templateData{
		RemoteID:       def.RemoteID,
		VoucherType:    def.VoucherType,
		TallyDate:      tallyDate,
		Narration:      def.Narration,
		PartyLedger:    def.PartyLedger,
		PurchaseLedger: def.PurchaseLedger,
		TotalAmount:    def.TotalAmount,
		PurchaseAmount: purchaseAmount,
		TaxEntries:     def.TaxEntries,
		InventoryItems: def.InventoryItems,
	}

	var buf bytes.Buffer
	if err := voucherTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

package convert

import (
	"strings"
	"testing"
)

func fullVoucherDef() *VoucherDef {
	return &VoucherDef{
		DocumentID:     "doc-123",
		VoucherType:    "Purchase",
		VoucherDate:    "2024-03-15",
		PartyLedger:    "Acme Corp",
		PurchaseLedger: "Purchase@18%Gst",
		TaxEntries: []TaxEntry{
			{LedgerName: "Input Cgst @9%", Amount: 900.00},
			{LedgerName: "Input Sgst @9%", Amount: 900.00},
		},
		InventoryItems: []InventoryItem{
			{
				StockItem: "Widget A",
				Quantity:  10,
				Rate:      1000.00,
				Amount:    10000.00,
				UOM:       "Nos",
				Godown:    "Main Godown",
				HSNCode:   "84719000",
			},
		},
		TotalAmount: 11800.00,
		Narration:   "Acme Corp - INV-001",
		RemoteID:    "tenant-123-doc-123",
	}
}

func TestToXML_FullVoucher(t *testing.T) {
	def := fullVoucherDef()
	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check REMOTEID is present
	if !strings.Contains(xml, `REMOTEID="tenant-123-doc-123"`) {
		t.Error("expected REMOTEID in output")
	}

	// Check voucher type
	if !strings.Contains(xml, `VCHTYPE="Purchase"`) {
		t.Error("expected VCHTYPE=Purchase in output")
	}

	// Check amounts present
	if !strings.Contains(xml, "11800.00") {
		t.Error("expected total amount 11800.00 in output")
	}

	// Check ledger names present
	if !strings.Contains(xml, "Acme Corp") {
		t.Error("expected party ledger name in output")
	}
	if !strings.Contains(xml, "Purchase@18%Gst") {
		t.Error("expected purchase ledger name in output")
	}
	if !strings.Contains(xml, "Input Cgst @9%") {
		t.Error("expected tax ledger name in output")
	}

	// Check inventory item present
	if !strings.Contains(xml, "Widget A") {
		t.Error("expected stock item name in output")
	}
	if !strings.Contains(xml, "Main Godown") {
		t.Error("expected godown name in output")
	}
}

func TestToXML_AmountSigns(t *testing.T) {
	def := fullVoucherDef()
	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Party amount should be positive (credit): 11800.00
	// It appears inside the first ALLLEDGERENTRIES.LIST block
	partyBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 0)
	if !strings.Contains(partyBlock, "<AMOUNT>11800.00</AMOUNT>") {
		t.Errorf("party amount should be positive 11800.00, got block: %s", partyBlock)
	}

	// Purchase amount should be negative (debit): -(11800 - 900 - 900) = -10000.00
	purchaseBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 1)
	if !strings.Contains(purchaseBlock, "<AMOUNT>-10000.00</AMOUNT>") {
		t.Errorf("purchase amount should be negative -10000.00, got block: %s", purchaseBlock)
	}

	// Tax amounts should be negative (debit): -900.00
	taxBlock1 := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 2)
	if !strings.Contains(taxBlock1, "<AMOUNT>-900.00</AMOUNT>") {
		t.Errorf("tax amount should be negative -900.00, got block: %s", taxBlock1)
	}

	taxBlock2 := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 3)
	if !strings.Contains(taxBlock2, "<AMOUNT>-900.00</AMOUNT>") {
		t.Errorf("tax amount should be negative -900.00, got block: %s", taxBlock2)
	}

	// Inventory amount should be negative (debit): -10000.00
	invBlock := extractBlock(xml, "ALLINVENTORYENTRIES.LIST", 0)
	if !strings.Contains(invBlock, "<AMOUNT>-10000.00</AMOUNT>") {
		t.Errorf("inventory amount should be negative -10000.00, got block: %s", invBlock)
	}
}

func TestToXML_ISDEEMEDPOSITIVE(t *testing.T) {
	def := fullVoucherDef()
	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Party ledger: ISDEEMEDPOSITIVE = No (credit)
	partyBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 0)
	if !strings.Contains(partyBlock, "<ISDEEMEDPOSITIVE>No</ISDEEMEDPOSITIVE>") {
		t.Error("party ledger ISDEEMEDPOSITIVE should be No")
	}

	// Purchase ledger: ISDEEMEDPOSITIVE = Yes (debit)
	purchaseBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 1)
	if !strings.Contains(purchaseBlock, "<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>") {
		t.Error("purchase ledger ISDEEMEDPOSITIVE should be Yes")
	}

	// Tax ledger: ISDEEMEDPOSITIVE = Yes (debit)
	taxBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 2)
	if !strings.Contains(taxBlock, "<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>") {
		t.Error("tax ledger ISDEEMEDPOSITIVE should be Yes")
	}

	// Inventory: ISDEEMEDPOSITIVE = Yes (debit)
	invBlock := extractBlock(xml, "ALLINVENTORYENTRIES.LIST", 0)
	if !strings.Contains(invBlock, "<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>") {
		t.Error("inventory ISDEEMEDPOSITIVE should be Yes")
	}
}

func TestToXML_DateConversion(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-03-15",
		PartyLedger:    "Test",
		PurchaseLedger: "Purchase",
		TotalAmount:    100,
		RemoteID:       "r1",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xml, "<DATE>20240315</DATE>") {
		t.Error("expected date converted to YYYYMMDD format: 20240315")
	}
}

func TestToXML_XMLEscape(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-01-01",
		PartyLedger:    "Smith & Sons <Ltd>",
		PurchaseLedger: `He said "hello"`,
		TotalAmount:    100,
		RemoteID:       "r&1",
		Narration:      "Test's narration",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xml, "Smith &amp; Sons &lt;Ltd&gt;") {
		t.Error("expected ampersand and angle brackets to be escaped in party ledger")
	}
	if !strings.Contains(xml, `He said &quot;hello&quot;`) {
		t.Error("expected double quotes to be escaped in purchase ledger")
	}
	if !strings.Contains(xml, "r&amp;1") {
		t.Error("expected ampersand to be escaped in REMOTEID")
	}
	if !strings.Contains(xml, "Test&apos;s narration") {
		t.Error("expected apostrophe to be escaped in narration")
	}
}

func TestToXML_NoInventory(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-06-01",
		PartyLedger:    "Vendor X",
		PurchaseLedger: "Purchase@5%Gst",
		TaxEntries: []TaxEntry{
			{LedgerName: "Input Igst @5%", Amount: 50.00},
		},
		TotalAmount: 1050.00,
		Narration:   "Service invoice",
		RemoteID:    "r2",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have ledger entries but no inventory entries
	if !strings.Contains(xml, "ALLLEDGERENTRIES.LIST") {
		t.Error("expected ledger entries in output")
	}
	if strings.Contains(xml, "ALLINVENTORYENTRIES.LIST") {
		t.Error("expected no inventory entries in output")
	}

	// Purchase amount = 1050 - 50 = 1000
	purchaseBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 1)
	if !strings.Contains(purchaseBlock, "<AMOUNT>-1000.00</AMOUNT>") {
		t.Errorf("purchase amount should be -1000.00, got block: %s", purchaseBlock)
	}
}

func TestToXML_NilDef(t *testing.T) {
	_, err := ToXML(nil)
	if err == nil {
		t.Fatal("expected error for nil voucher definition")
	}
	if !strings.Contains(err.Error(), "nil voucher definition") {
		t.Errorf("expected 'nil voucher definition' error, got: %v", err)
	}
}

func TestToXML_NoTax(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-01-01",
		PartyLedger:    "Exempt Vendor",
		PurchaseLedger: "Purchase@0%Gst",
		TaxEntries:     nil,
		TotalAmount:    5000.00,
		Narration:      "Exempt purchase",
		RemoteID:       "r3",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Purchase amount should equal total amount when there are no taxes
	purchaseBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 1)
	if !strings.Contains(purchaseBlock, "<AMOUNT>-5000.00</AMOUNT>") {
		t.Errorf("purchase amount should equal negated total (-5000.00) when no tax, got block: %s", purchaseBlock)
	}

	// Party amount should be positive total
	partyBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 0)
	if !strings.Contains(partyBlock, "<AMOUNT>5000.00</AMOUNT>") {
		t.Errorf("party amount should be 5000.00, got block: %s", partyBlock)
	}

	// Should have exactly 2 ledger entry blocks (party + purchase), no tax blocks
	count := strings.Count(xml, "<ALLLEDGERENTRIES.LIST>")
	if count != 2 {
		t.Errorf("expected 2 ALLLEDGERENTRIES.LIST blocks (party + purchase), got %d", count)
	}
}

// extractBlock extracts the nth occurrence of a block delimited by <tag> and </tag>.
func extractBlock(xml, tag string, n int) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	idx := 0
	for i := 0; i <= n; i++ {
		start := strings.Index(xml[idx:], openTag)
		if start == -1 {
			return ""
		}
		start += idx
		end := strings.Index(xml[start:], closeTag)
		if end == -1 {
			return ""
		}
		end += start + len(closeTag)
		if i == n {
			return xml[start:end]
		}
		idx = end
	}
	return ""
}

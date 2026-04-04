package convert

import (
	"strings"
	"testing"
)

func fullVoucherDef() *VoucherDef {
	return &VoucherDef{
		DocumentID:          "doc-123",
		VoucherType:         "Purchase",
		VoucherMode:         "item_invoice",
		VoucherDate:         "2024-03-15",
		PartyLedger:         "Acme Corp",
		PurchaseLedger:      "Purchase@18%Gst",
		SupplierInvoiceNo:   "INV-2024-001",
		SupplierInvoiceDate: "2024-03-10",
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

	// Check inventory item present (item_invoice mode)
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

func TestToXML_InvalidMode(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherMode:    "invalid_mode",
		VoucherDate:    "2024-01-01",
		PartyLedger:    "Test",
		PurchaseLedger: "Purchase",
		TotalAmount:    100,
		RemoteID:       "r-invalid",
	}

	_, err := ToXML(def)
	if err == nil {
		t.Fatal("expected error for invalid voucher mode")
	}
	if !strings.Contains(err.Error(), "unrecognized voucher mode") {
		t.Errorf("expected 'unrecognized voucher mode' error, got: %v", err)
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

func TestToXML_ItemInvoiceMode(t *testing.T) {
	def := fullVoucherDef()
	// fullVoucherDef already uses item_invoice mode
	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have VCHTYPE=Purchase
	if !strings.Contains(xml, `VCHTYPE="Purchase"`) {
		t.Error("expected VCHTYPE=Purchase for item_invoice mode")
	}
	if !strings.Contains(xml, "<ISINVOICE>Yes</ISINVOICE>") {
		t.Error("expected ISINVOICE=Yes for item_invoice mode")
	}
	if !strings.Contains(xml, "<PERSISTEDVIEW>Invoice Voucher View</PERSISTEDVIEW>") {
		t.Error("expected Invoice Voucher View for item_invoice mode")
	}

	// Should have inventory entries
	if !strings.Contains(xml, "ALLINVENTORYENTRIES.LIST") {
		t.Error("expected ALLINVENTORYENTRIES.LIST for item_invoice mode")
	}

	// Should have tax entries
	if !strings.Contains(xml, "Input Cgst @9%") {
		t.Error("expected tax entries for item_invoice mode")
	}
}

func TestToXML_AccountingInvoiceMode(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherMode:    "accounting_invoice",
		VoucherDate:    "2024-06-01",
		PartyLedger:    "Vendor Y",
		PurchaseLedger: "Purchase@18%Gst",
		TaxEntries: []TaxEntry{
			{LedgerName: "Input Cgst @9%", Amount: 90.00},
			{LedgerName: "Input Sgst @9%", Amount: 90.00},
		},
		InventoryItems: []InventoryItem{
			{StockItem: "Widget B", Quantity: 5, Rate: 200, Amount: 1000, UOM: "Nos"},
		},
		TotalAmount: 1180.00,
		RemoteID:    "r-acct",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have VCHTYPE=Purchase
	if !strings.Contains(xml, `VCHTYPE="Purchase"`) {
		t.Error("expected VCHTYPE=Purchase for accounting_invoice mode")
	}
	if !strings.Contains(xml, "<ISINVOICE>Yes</ISINVOICE>") {
		t.Error("expected ISINVOICE=Yes for accounting_invoice mode")
	}

	// Should NOT have inventory entries (accounting_invoice omits inventory)
	if strings.Contains(xml, "ALLINVENTORYENTRIES.LIST") {
		t.Error("expected no ALLINVENTORYENTRIES.LIST for accounting_invoice mode")
	}

	// Should have tax entries
	if !strings.Contains(xml, "Input Cgst @9%") {
		t.Error("expected tax entries for accounting_invoice mode")
	}
}

func TestToXML_DefaultModeIsAccountingInvoice(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-01-01",
		PartyLedger:    "Test Vendor",
		PurchaseLedger: "Purchase",
		TotalAmount:    100,
		RemoteID:       "r-default",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default (empty VoucherMode) should behave as accounting_invoice
	if !strings.Contains(xml, `VCHTYPE="Purchase"`) {
		t.Error("expected VCHTYPE=Purchase for default mode")
	}
	if !strings.Contains(xml, "<ISINVOICE>Yes</ISINVOICE>") {
		t.Error("expected ISINVOICE=Yes for default mode")
	}
	if strings.Contains(xml, "ALLINVENTORYENTRIES.LIST") {
		t.Error("expected no ALLINVENTORYENTRIES.LIST for default mode")
	}
}

func TestToXML_JournalMode(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Journal",
		VoucherMode:    "journal",
		VoucherDate:    "2024-07-01",
		PartyLedger:    "Expense Account",
		PurchaseLedger: "Cash",
		TaxEntries: []TaxEntry{
			{LedgerName: "Input Cgst @9%", Amount: 50.00},
		},
		TotalAmount: 550.00,
		Narration:   "Journal adjustment",
		RemoteID:    "r-journal",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have Journal voucher type
	if !strings.Contains(xml, `VCHTYPE="Journal"`) {
		t.Error("expected VCHTYPE=Journal for journal mode")
	}
	if !strings.Contains(xml, "<VOUCHERTYPENAME>Journal</VOUCHERTYPENAME>") {
		t.Error("expected VOUCHERTYPENAME=Journal for journal mode")
	}
	if !strings.Contains(xml, "<ISINVOICE>No</ISINVOICE>") {
		t.Error("expected ISINVOICE=No for journal mode")
	}
	if !strings.Contains(xml, "<PERSISTEDVIEW>Accounting Voucher View</PERSISTEDVIEW>") {
		t.Error("expected Accounting Voucher View for journal mode")
	}

	// Should NOT have tax entries
	if strings.Contains(xml, "Input Cgst @9%") {
		t.Error("expected no tax entries for journal mode")
	}

	// Should NOT have inventory entries
	if strings.Contains(xml, "ALLINVENTORYENTRIES.LIST") {
		t.Error("expected no ALLINVENTORYENTRIES.LIST for journal mode")
	}

	// Should have exactly 2 ledger entries (party debit + purchase credit)
	count := strings.Count(xml, "<ALLLEDGERENTRIES.LIST>")
	if count != 2 {
		t.Errorf("expected 2 ALLLEDGERENTRIES.LIST blocks for journal, got %d", count)
	}

	// In journal mode, purchase amount should equal total (taxes suppressed),
	// so debit == credit == 550.00.
	purchaseBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 1)
	if !strings.Contains(purchaseBlock, "<AMOUNT>-550.00</AMOUNT>") {
		t.Errorf("journal purchase amount should be -550.00 (full total, no tax subtraction), got block: %s", purchaseBlock)
	}
	partyBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 0)
	if !strings.Contains(partyBlock, "<AMOUNT>550.00</AMOUNT>") {
		t.Errorf("journal party amount should be 550.00, got block: %s", partyBlock)
	}
}

func TestToXML_ReferenceFields(t *testing.T) {
	def := &VoucherDef{
		VoucherType:         "Purchase",
		VoucherDate:         "2024-05-01",
		PartyLedger:         "Test",
		PurchaseLedger:      "Purchase",
		TotalAmount:         100,
		RemoteID:            "r-ref",
		SupplierInvoiceNo:   "SUP-123",
		SupplierInvoiceDate: "2024-04-28",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xml, "<REFERENCE>SUP-123</REFERENCE>") {
		t.Error("expected REFERENCE with supplier invoice number")
	}
	if !strings.Contains(xml, "<REFERENCEDATE>20240428</REFERENCEDATE>") {
		t.Error("expected REFERENCEDATE converted to YYYYMMDD format")
	}
}

func TestToXML_BillAllocations(t *testing.T) {
	def := &VoucherDef{
		VoucherType:       "Purchase",
		VoucherDate:       "2024-05-01",
		PartyLedger:       "Test Vendor",
		PurchaseLedger:    "Purchase",
		TotalAmount:       5000.00,
		RemoteID:          "r-bill",
		SupplierInvoiceNo: "BILL-456",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// BILLALLOCATIONS should be inside the party ledger entry
	partyBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 0)
	if !strings.Contains(partyBlock, "<BILLALLOCATIONS.LIST>") {
		t.Error("expected BILLALLOCATIONS.LIST in party ledger entry")
	}
	if !strings.Contains(partyBlock, "<NAME>BILL-456</NAME>") {
		t.Error("expected bill allocation NAME to match supplier invoice number")
	}
	if !strings.Contains(partyBlock, "<BILLTYPE>New Ref</BILLTYPE>") {
		t.Error("expected BILLTYPE=New Ref")
	}
	if !strings.Contains(partyBlock, "<AMOUNT>5000.00</AMOUNT>") {
		t.Error("expected bill allocation AMOUNT to match total amount")
	}
}

func TestToXML_NoBillAllocationsWithoutInvoiceNo(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-05-01",
		PartyLedger:    "Test",
		PurchaseLedger: "Purchase",
		TotalAmount:    100,
		RemoteID:       "r-nobill",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(xml, "BILLALLOCATIONS.LIST") {
		t.Error("expected no BILLALLOCATIONS.LIST when SupplierInvoiceNo is empty")
	}

	// REFERENCE and REFERENCEDATE should also be absent when not set (S4)
	if strings.Contains(xml, "<REFERENCE>") {
		t.Error("expected no REFERENCE tag when SupplierInvoiceNo is empty")
	}
	if strings.Contains(xml, "<REFERENCEDATE>") {
		t.Error("expected no REFERENCEDATE tag when SupplierInvoiceDate is empty")
	}
}

func TestToXML_EmptyVoucherDate(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "",
		PartyLedger:    "Test",
		PurchaseLedger: "Purchase",
		TotalAmount:    100,
		RemoteID:       "r-nodate",
	}

	_, err := ToXML(def)
	if err == nil {
		t.Fatal("expected error for empty VoucherDate")
	}
	if !strings.Contains(err.Error(), "VoucherDate is required") {
		t.Errorf("expected 'VoucherDate is required' error, got: %v", err)
	}
}

func TestToXML_MalformedVoucherDate(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024/03/15",
		PartyLedger:    "Test",
		PurchaseLedger: "Purchase",
		TotalAmount:    100,
		RemoteID:       "r-baddate",
	}

	_, err := ToXML(def)
	if err == nil {
		t.Fatal("expected error for malformed VoucherDate")
	}
	if !strings.Contains(err.Error(), "invalid VoucherDate") {
		t.Errorf("expected 'invalid VoucherDate' error, got: %v", err)
	}
}

func TestToXML_AmountRounding(t *testing.T) {
	def := &VoucherDef{
		VoucherType:    "Purchase",
		VoucherDate:    "2024-01-01",
		PartyLedger:    "Vendor",
		PurchaseLedger: "Purchase",
		TaxEntries: []TaxEntry{
			{LedgerName: "CGST", Amount: 90.005},
			{LedgerName: "SGST", Amount: 90.005},
		},
		TotalAmount: 1000.005,
		RemoteID:    "r-round",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TotalAmount 1000.005 rounds to 1000.01
	partyBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 0)
	if !strings.Contains(partyBlock, "<AMOUNT>1000.01</AMOUNT>") {
		t.Errorf("expected rounded total 1000.01 in party block, got: %s", partyBlock)
	}

	// Tax amounts: 90.005 rounds to 90.01 each
	taxBlock1 := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 2)
	if !strings.Contains(taxBlock1, "<AMOUNT>-90.01</AMOUNT>") {
		t.Errorf("expected rounded tax -90.01, got: %s", taxBlock1)
	}

	// Purchase = 1000.01 - 90.01 - 90.01 = 819.99
	purchaseBlock := extractBlock(xml, "ALLLEDGERENTRIES.LIST", 1)
	if !strings.Contains(purchaseBlock, "<AMOUNT>-819.99</AMOUNT>") {
		t.Errorf("expected rounded purchase -819.99, got: %s", purchaseBlock)
	}

	// Verify balance: party = purchase + taxes => 1000.01 = 819.99 + 90.01 + 90.01
	// This confirms debit == credit
}

func TestToXML_ReferenceDateWithoutReference(t *testing.T) {
	def := &VoucherDef{
		VoucherType:         "Purchase",
		VoucherDate:         "2024-05-01",
		PartyLedger:         "Test",
		PurchaseLedger:      "Purchase",
		TotalAmount:         100,
		RemoteID:            "r-refdate-only",
		SupplierInvoiceNo:   "",
		SupplierInvoiceDate: "2024-04-28",
	}

	xml, err := ToXML(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(xml, "<REFERENCE>") {
		t.Error("expected no REFERENCE tag when SupplierInvoiceNo is empty")
	}
	if strings.Contains(xml, "<REFERENCEDATE>") {
		t.Error("expected no REFERENCEDATE tag when SupplierInvoiceNo is empty, even if SupplierInvoiceDate is set")
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

package tally

import (
	"strings"
	"testing"
)

func TestBuildLedgerXML_AllFields(t *testing.T) {
	def := LedgerDef{
		Name:        "Acme Corp",
		ParentGroup: "Sundry Creditors",
		Address:     "123 Main St, Mumbai",
		PAN:         "ABCCE1234F",
		GSTIN:       "27ABCCE1234F1Z5",
		State:       "Maharashtra",
	}
	xml, err := BuildLedgerXML(&def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		label    string
		contains string
	}{
		{"ledger name attr", `LEDGER NAME="Acme Corp"`},
		{"name element", `<NAME>Acme Corp</NAME>`},
		{"parent", `<PARENT>Sundry Creditors</PARENT>`},
		{"address", `<ADDRESS>123 Main St, Mumbai</ADDRESS>`},
		{"pan", `<INCOMETAXNUMBER>ABCCE1234F</INCOMETAXNUMBER>`},
		{"tds applicable", `<ISTDSAPPLICABLE>Yes</ISTDSAPPLICABLE>`},
		{"deductee type", `<TDSDEDUCTEETYPE>Company</TDSDEDUCTEETYPE>`},
		{"gstin", `<PARTYGSTIN>27ABCCE1234F1Z5</PARTYGSTIN>`},
		{"gst reg type", `<GSTREGISTRATIONTYPE>Regular</GSTREGISTRATIONTYPE>`},
		{"state", `<LEDSTATENAME>Maharashtra</LEDSTATENAME>`},
	}

	for _, c := range checks {
		if !strings.Contains(xml, c.contains) {
			t.Errorf("%s: expected XML to contain %q, got:\n%s", c.label, c.contains, xml)
		}
	}
}

func TestBuildLedgerXML_MinimalFields(t *testing.T) {
	def := LedgerDef{
		Name:        "Simple Ledger",
		ParentGroup: "Purchase Accounts",
	}
	xml, err := BuildLedgerXML(&def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain basic fields
	if !strings.Contains(xml, `<NAME>Simple Ledger</NAME>`) {
		t.Errorf("expected NAME tag, got:\n%s", xml)
	}
	if !strings.Contains(xml, `<PARENT>Purchase Accounts</PARENT>`) {
		t.Errorf("expected PARENT tag, got:\n%s", xml)
	}

	// Should NOT contain optional fields
	absent := []string{
		"ADDRESS", "INCOMETAXNUMBER", "ISTDSAPPLICABLE", "TDSDEDUCTEETYPE",
		"PARTYGSTIN", "GSTREGISTRATIONTYPE", "LEDSTATENAME",
	}
	for _, tag := range absent {
		if strings.Contains(xml, "<"+tag+">") {
			t.Errorf("expected no <%s> tag in minimal output, got:\n%s", tag, xml)
		}
	}
}

func TestBuildLedgerXML_XMLEscape(t *testing.T) {
	def := LedgerDef{
		Name:        `M&M "Foods" <Pvt>`,
		ParentGroup: "Sundry Creditors",
	}
	xml, err := BuildLedgerXML(&def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xml, `M&amp;M &quot;Foods&quot; &lt;Pvt&gt;`) {
		t.Errorf("XML escaping not applied correctly, got:\n%s", xml)
	}
}

func TestDeducteeTypeFromPAN(t *testing.T) {
	tests := []struct {
		pan      string
		expected string
	}{
		{"ABCCE1234F", "Company"},
		{"ABCcE1234F", "Company"},
		{"ABCFE1234F", "Firm"},
		{"ABCfE1234F", "Firm"},
		{"ABCTE1234F", "Trust"},
		{"ABCtE1234F", "Trust"},
		{"ABCHE1234F", "HUF"},
		{"ABChE1234F", "HUF"},
		{"ABCAE1234F", "AOP/BOI"},
		{"ABCaE1234F", "AOP/BOI"},
		{"ABCPE1234F", "Individual"},
		{"ABCpE1234F", "Individual"},
		{"ABCXE1234F", "Individual"}, // unknown defaults to Individual
		{"AB", ""},                    // too short
		{"", ""},                      // empty
	}

	for _, tc := range tests {
		got := deducteeTypeFromPAN(tc.pan)
		if got != tc.expected {
			t.Errorf("deducteeTypeFromPAN(%q) = %q, want %q", tc.pan, got, tc.expected)
		}
	}
}

func TestBuildLedgerXML_CustomGSTRegType(t *testing.T) {
	def := LedgerDef{
		Name:        "Custom GST Vendor",
		ParentGroup: "Sundry Creditors",
		GSTIN:       "27ABCCE1234F1Z5",
		GSTRegType:  "Composition",
	}
	xml, err := BuildLedgerXML(&def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(xml, "<GSTREGISTRATIONTYPE>Composition</GSTREGISTRATIONTYPE>") {
		t.Errorf("expected custom GSTRegType=Composition, got:\n%s", xml)
	}
}

func TestBuildLedgerXML_DefaultGSTRegType(t *testing.T) {
	def := LedgerDef{
		Name:        "Default GST Vendor",
		ParentGroup: "Sundry Creditors",
		GSTIN:       "27ABCCE1234F1Z5",
	}
	xml, err := BuildLedgerXML(&def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(xml, "<GSTREGISTRATIONTYPE>Regular</GSTREGISTRATIONTYPE>") {
		t.Errorf("expected default GSTRegType=Regular, got:\n%s", xml)
	}
}

// TestIsZeroCountOnly_MasterImport verifies that ImportResult.IsZeroCountOnly
// is set when the only "error" is zero created/altered counts, which is expected
// with DUPIGNORECOMBINE when all ledgers already exist.
func TestIsZeroCountOnly_MasterImport(t *testing.T) {
	// Simulate a Tally response where CREATED=0, ALTERED=0, no LINEERROR.
	result := buildResult(0, 0, 0, 0, "", "", "", []byte("<ENVELOPE/>"))
	if !result.IsZeroCountOnly {
		t.Error("expected IsZeroCountOnly=true when CREATED=0 ALTERED=0 with no real errors")
	}
	if result.Success {
		t.Error("expected Success=false for zero counts")
	}

	// Simulate a Tally response with LINEERROR — should NOT be IsZeroCountOnly.
	result2 := buildResult(0, 0, 0, 0, "", "", "some error", []byte("<ENVELOPE/>"))
	if result2.IsZeroCountOnly {
		t.Error("expected IsZeroCountOnly=false when LINEERROR is present")
	}

	// Simulate success case — should NOT be IsZeroCountOnly.
	result3 := buildResult(1, 0, 0, 0, "V1", "", "", []byte("<ENVELOPE/>"))
	if result3.IsZeroCountOnly {
		t.Error("expected IsZeroCountOnly=false when CREATED > 0")
	}
	if !result3.Success {
		t.Error("expected Success=true when CREATED > 0")
	}
}

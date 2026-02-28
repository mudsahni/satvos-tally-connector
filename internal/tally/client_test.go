package tally

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient creates a Client backed by the given httptest.Server.
func newTestClient(ts *httptest.Server) *Client {
	return NewClientWithHTTPClient(ts.URL, ts.Client())
}

func TestClient_GetCompanyInfo_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "text/xml; charset=utf-8" {
			t.Errorf("expected Content-Type text/xml; charset=utf-8, got %s", ct)
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<RESULT>Acme Corp Pvt Ltd</RESULT>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	info, err := client.GetCompanyInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "Acme Corp Pvt Ltd" {
		t.Errorf("expected company name 'Acme Corp Pvt Ltd', got '%s'", info.Name)
	}
}

func TestClient_GetCompanyInfo_ConnectionRefused(t *testing.T) {
	// Point to a port that is not listening.
	client := NewClient("127.0.0.1", 1)
	_, err := client.GetCompanyInfo(context.Background())
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
}

func TestClient_GetLedgers_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<BODY>
<DATA>
<COLLECTION>
<LEDGER>
<NAME>Cash</NAME>
<PARENT>Cash-in-Hand</PARENT>
<GSTIN></GSTIN>
<LEDSTATENAME>Maharashtra</LEDSTATENAME>
<TAXTYPE></TAXTYPE>
<RATEOFTAXCALCULATION>0</RATEOFTAXCALCULATION>
<ISREVENUE>No</ISREVENUE>
</LEDGER>
<LEDGER>
<NAME>Input CGST @9%</NAME>
<PARENT>Duties &amp; Taxes</PARENT>
<GSTIN>27AAACR5055K1ZS</GSTIN>
<LEDSTATENAME>Maharashtra</LEDSTATENAME>
<TAXTYPE>GST</TAXTYPE>
<RATEOFTAXCALCULATION>9</RATEOFTAXCALCULATION>
<ISREVENUE>Yes</ISREVENUE>
</LEDGER>
</COLLECTION>
</DATA>
</BODY>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	ledgers, err := client.GetLedgers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ledgers) != 2 {
		t.Fatalf("expected 2 ledgers, got %d", len(ledgers))
	}
	if ledgers[0].Name != "Cash" {
		t.Errorf("expected first ledger name 'Cash', got '%s'", ledgers[0].Name)
	}
	if ledgers[0].Parent != "Cash-in-Hand" {
		t.Errorf("expected first ledger parent 'Cash-in-Hand', got '%s'", ledgers[0].Parent)
	}
	if ledgers[1].Name != "Input CGST @9%" {
		t.Errorf("expected second ledger name 'Input CGST @9%%', got '%s'", ledgers[1].Name)
	}
	if ledgers[1].RateOfTax != 9 {
		t.Errorf("expected rate of tax 9, got %f", ledgers[1].RateOfTax)
	}
	if ledgers[1].GSTNumber != "27AAACR5055K1ZS" {
		t.Errorf("expected GSTIN '27AAACR5055K1ZS', got '%s'", ledgers[1].GSTNumber)
	}
}

func TestClient_GetStockItems_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<BODY>
<DATA>
<COLLECTION>
<STOCKITEM>
<NAME>Steel Rod 12mm</NAME>
<PARENT>Raw Materials</PARENT>
<GSTDETAILS.LIST>
<HSNCODE>72142000</HSNCODE>
</GSTDETAILS.LIST>
<BASEUNITS>Kg</BASEUNITS>
</STOCKITEM>
<STOCKITEM>
<NAME>Cement OPC 53</NAME>
<PARENT>Raw Materials</PARENT>
<GSTDETAILS.LIST>
<HSNCODE>25232900</HSNCODE>
</GSTDETAILS.LIST>
<BASEUNITS>Bag</BASEUNITS>
</STOCKITEM>
</COLLECTION>
</DATA>
</BODY>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	items, err := client.GetStockItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 stock items, got %d", len(items))
	}
	if items[0].Name != "Steel Rod 12mm" {
		t.Errorf("expected 'Steel Rod 12mm', got '%s'", items[0].Name)
	}
	if items[0].HSNCode != "72142000" {
		t.Errorf("expected HSN '72142000', got '%s'", items[0].HSNCode)
	}
	if items[0].BaseUnit != "Kg" {
		t.Errorf("expected base unit 'Kg', got '%s'", items[0].BaseUnit)
	}
	if items[1].Name != "Cement OPC 53" {
		t.Errorf("expected 'Cement OPC 53', got '%s'", items[1].Name)
	}
}

func TestClient_ImportVoucher_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<RESPONSE>
<CREATED>1</CREATED>
<ALTERED>0</ALTERED>
<LASTVCHID>12345</LASTVCHID>
<LASTMID>67890</LASTMID>
</RESPONSE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	voucherXML := `<VOUCHER VCHTYPE="Purchase" ACTION="Create">
<DATE>20240115</DATE>
<NARRATION>Test purchase</NARRATION>
</VOUCHER>`

	result, err := client.ImportVoucher(context.Background(), voucherXML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Created != 1 {
		t.Errorf("expected created=1, got %d", result.Created)
	}
	if result.Altered != 0 {
		t.Errorf("expected altered=0, got %d", result.Altered)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestClient_ImportVoucher_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<RESPONSE>
<CREATED>0</CREATED>
<ALTERED>0</ALTERED>
<LINEERROR>Ledger 'Purchase@18%Gst' does not exist</LINEERROR>
</RESPONSE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	voucherXML := `<VOUCHER VCHTYPE="Purchase" ACTION="Create">
<DATE>20240115</DATE>
</VOUCHER>`

	result, err := client.ImportVoucher(context.Background(), voucherXML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected success=false for error response")
	}
	if result.Created != 0 {
		t.Errorf("expected created=0, got %d", result.Created)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0] != "Ledger 'Purchase@18%Gst' does not exist" {
		t.Errorf("unexpected error message: %s", result.Errors[0])
	}
}

func TestClient_IsAvailable_True(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<RESULT>Test Company</RESULT>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	if !client.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to return true when server responds")
	}
}

func TestClient_IsAvailable_False(t *testing.T) {
	// No server running -- point to a port that is not listening.
	client := NewClient("127.0.0.1", 1)
	if client.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to return false when no server is running")
	}
}

func TestClient_GetGodowns_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<BODY>
<DATA>
<COLLECTION>
<GODOWN>
<NAME>Main Location</NAME>
<PARENT></PARENT>
</GODOWN>
<GODOWN>
<NAME>Warehouse A</NAME>
<PARENT>Main Location</PARENT>
</GODOWN>
</COLLECTION>
</DATA>
</BODY>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	godowns, err := client.GetGodowns(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(godowns) != 2 {
		t.Fatalf("expected 2 godowns, got %d", len(godowns))
	}
	if godowns[0].Name != "Main Location" {
		t.Errorf("expected 'Main Location', got '%s'", godowns[0].Name)
	}
	if godowns[1].Parent != "Main Location" {
		t.Errorf("expected parent 'Main Location', got '%s'", godowns[1].Parent)
	}
}

func TestClient_GetUnits_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<BODY>
<DATA>
<COLLECTION>
<UNIT>
<SYMBOL>Kg</SYMBOL>
<FORMALNAME>Kilogram</FORMALNAME>
</UNIT>
<UNIT>
<SYMBOL>Nos</SYMBOL>
<FORMALNAME>Numbers</FORMALNAME>
</UNIT>
</COLLECTION>
</DATA>
</BODY>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	units, err := client.GetUnits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}
	if units[0].Symbol != "Kg" {
		t.Errorf("expected symbol 'Kg', got '%s'", units[0].Symbol)
	}
	if units[0].FormalName != "Kilogram" {
		t.Errorf("expected formal name 'Kilogram', got '%s'", units[0].FormalName)
	}
}

func TestClient_GetCostCentres_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE>
<BODY>
<DATA>
<COLLECTION>
<COSTCENTRE>
<NAME>Head Office</NAME>
<PARENT></PARENT>
</COSTCENTRE>
</COLLECTION>
</DATA>
</BODY>
</ENVELOPE>`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	centres, err := client.GetCostCentres(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(centres) != 1 {
		t.Fatalf("expected 1 cost centre, got %d", len(centres))
	}
	if centres[0].Name != "Head Office" {
		t.Errorf("expected 'Head Office', got '%s'", centres[0].Name)
	}
}

func TestParseCompanyInfoResponse_EmptyResult(t *testing.T) {
	data := []byte(`<ENVELOPE><RESULT></RESULT></ENVELOPE>`)
	_, err := ParseCompanyInfoResponse(data)
	if err == nil {
		t.Fatal("expected error for empty company name")
	}
}

func TestParseCompanyInfoResponse_InvalidXML(t *testing.T) {
	data := []byte(`not xml at all`)
	_, err := ParseCompanyInfoResponse(data)
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestBuildMasterExportRequest_ContainsMasterType(t *testing.T) {
	req := BuildMasterExportRequest("Ledger")
	reqStr := string(req)
	if !strings.Contains(reqStr, "<ID>LedgerList</ID>") {
		t.Error("expected request to contain <ID>LedgerList</ID>")
	}
	if !strings.Contains(reqStr, `<TYPE>Ledger</TYPE>`) {
		t.Error("expected request to contain <TYPE>Ledger</TYPE>")
	}
	if !strings.Contains(reqStr, `NAME="LedgerList"`) {
		t.Error("expected request to contain NAME=\"LedgerList\"")
	}
}

func TestBuildVoucherImportRequest_ContainsVoucherXML(t *testing.T) {
	voucherXML := `<VOUCHER VCHTYPE="Purchase"><DATE>20240115</DATE></VOUCHER>`
	req := BuildVoucherImportRequest(voucherXML)
	reqStr := string(req)
	if !strings.Contains(reqStr, voucherXML) {
		t.Error("expected import request to contain the voucher XML")
	}
	if !strings.Contains(reqStr, "<TALLYREQUEST>Import</TALLYREQUEST>") {
		t.Error("expected import request header")
	}
}

func TestNewClient_BaseURL(t *testing.T) {
	client := NewClient("localhost", 9000)
	if client.baseURL != "http://localhost:9000" {
		t.Errorf("expected base URL 'http://localhost:9000', got '%s'", client.baseURL)
	}
}

func TestParseImportResponse_EmptyResponse(t *testing.T) {
	data := []byte(`<RESPONSE><CREATED>0</CREATED><ALTERED>0</ALTERED></RESPONSE>`)
	result, err := ParseImportResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true for empty but valid response")
	}
	if result.Created != 0 {
		t.Errorf("expected created=0, got %d", result.Created)
	}
}


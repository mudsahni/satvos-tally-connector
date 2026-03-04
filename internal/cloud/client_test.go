package cloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Register_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync/v1/register" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req RegisterRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.Version != "1.0.0" {
			t.Errorf("expected version 1.0.0, got %s", req.Version)
		}
		if req.OSInfo != "linux" {
			t.Errorf("expected os_info linux, got %s", req.OSInfo)
		}

		w.WriteHeader(http.StatusCreated)
		resp := APIResponse{
			Success: true,
			Data: mustMarshal(t, RegisterResponse{
				ID:       "agent-123",
				TenantID: "tenant-456",
				Status:   "active",
			}),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	result, err := client.Register(context.Background(), RegisterRequest{
		Version: "1.0.0",
		OSInfo:  "linux",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "agent-123" {
		t.Errorf("expected ID agent-123, got %s", result.ID)
	}
	if result.TenantID != "tenant-456" {
		t.Errorf("expected TenantID tenant-456, got %s", result.TenantID)
	}
	if result.Status != "active" {
		t.Errorf("expected Status active, got %s", result.Status)
	}
}

func TestClient_Heartbeat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync/v1/heartbeat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req HeartbeatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if !req.TallyConnected {
			t.Error("expected tally_connected=true")
		}
		if req.TallyCompany != "My Company" {
			t.Errorf("expected tally_company 'My Company', got %s", req.TallyCompany)
		}
		if req.TallyPort != 9000 {
			t.Errorf("expected tally_port 9000, got %d", req.TallyPort)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	err := client.Heartbeat(context.Background(), HeartbeatRequest{
		TallyConnected: true,
		TallyCompany:   "My Company",
		TallyPort:      9000,
		Version:        "1.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_PushMasters_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync/v1/masters" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload MasterPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if len(payload.Ledgers) != 1 {
			t.Fatalf("expected 1 ledger, got %d", len(payload.Ledgers))
		}
		if payload.Ledgers[0].Name != "Purchase Account" {
			t.Errorf("expected ledger name 'Purchase Account', got %s", payload.Ledgers[0].Name)
		}
		if payload.Ledgers[0].GSTIN != "29ABCDE1234F1Z5" {
			t.Errorf("expected GSTIN 29ABCDE1234F1Z5, got %s", payload.Ledgers[0].GSTIN)
		}
		if len(payload.StockItems) != 1 {
			t.Fatalf("expected 1 stock item, got %d", len(payload.StockItems))
		}
		if payload.StockItems[0].HSNCode != "8471" {
			t.Errorf("expected HSN code 8471, got %s", payload.StockItems[0].HSNCode)
		}
		if len(payload.Units) != 1 {
			t.Fatalf("expected 1 unit, got %d", len(payload.Units))
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	err := client.PushMasters(context.Background(), &MasterPayload{
		Ledgers: []MasterLedger{
			{
				Name:        "Purchase Account",
				ParentGroup: "Purchase Accounts",
				GSTIN:       "29ABCDE1234F1Z5",
				State:       "Karnataka",
				TaxType:     "GST",
				TaxRate:     18.0,
				IsRevenue:   false,
			},
		},
		StockItems: []MasterStockItem{
			{
				Name:        "Laptop",
				ParentGroup: "Electronics",
				HSNCode:     "8471",
				DefaultUOM:  "Nos",
			},
		},
		Units: []MasterUnit{
			{
				Symbol:     "Nos",
				FormalName: "Numbers",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_PullOutbound_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync/v1/outbound" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("cursor") != "abc123" {
			t.Errorf("expected cursor=abc123, got %s", r.URL.Query().Get("cursor"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}

		w.WriteHeader(http.StatusOK)
		resp := APIResponse{
			Success: true,
			Data: mustMarshal(t, OutboundResponse{
				Items: []OutboundItem{
					{
						DocumentID:     "doc-001",
						StructuredData: json.RawMessage(`{"invoice_number":"INV-001"}`),
						VoucherDef:     json.RawMessage(`{"type":"purchase"}`),
						SyncEventID:    "evt-001",
					},
					{
						DocumentID:     "doc-002",
						StructuredData: json.RawMessage(`{"invoice_number":"INV-002"}`),
						VoucherDef:     json.RawMessage(`{"type":"purchase"}`),
						SyncEventID:    "evt-002",
					},
				},
				NextCursor: "def456",
			}),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	result, err := client.PullOutbound(context.Background(), "abc123", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].DocumentID != "doc-001" {
		t.Errorf("expected doc-001, got %s", result.Items[0].DocumentID)
	}
	if result.Items[0].SyncEventID != "evt-001" {
		t.Errorf("expected evt-001, got %s", result.Items[0].SyncEventID)
	}
	if result.NextCursor != "def456" {
		t.Errorf("expected next_cursor def456, got %s", result.NextCursor)
	}
}

func TestClient_Ack_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync/v1/ack" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req AckRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if len(req.Results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(req.Results))
		}
		if req.Results[0].SyncEventID != "evt-001" {
			t.Errorf("expected sync_event_id evt-001, got %s", req.Results[0].SyncEventID)
		}
		if !req.Results[0].Success {
			t.Error("expected first result success=true")
		}
		if req.Results[0].TallyVoucherNumber != "PUR/001" {
			t.Errorf("expected voucher number PUR/001, got %s", req.Results[0].TallyVoucherNumber)
		}
		if req.Results[1].Success {
			t.Error("expected second result success=false")
		}
		if req.Results[1].ErrorMessage != "ledger not found" {
			t.Errorf("expected error message 'ledger not found', got %s", req.Results[1].ErrorMessage)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	err := client.Ack(context.Background(), AckRequest{
		Results: []AckResult{
			{
				SyncEventID:        "evt-001",
				DocumentID:         "doc-001",
				Success:            true,
				TallyVoucherID:     "vid-001",
				TallyVoucherNumber: "PUR/001",
			},
			{
				SyncEventID:  "evt-002",
				DocumentID:   "doc-002",
				Success:      false,
				ErrorMessage: "ledger not found",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_PushInbound_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync/v1/inbound" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req InboundRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if len(req.Vouchers) != 1 {
			t.Fatalf("expected 1 voucher, got %d", len(req.Vouchers))
		}
		v := req.Vouchers[0]
		if v.VoucherType != "Sales" {
			t.Errorf("expected voucher type Sales, got %s", v.VoucherType)
		}
		if v.VoucherNumber != "SAL/001" {
			t.Errorf("expected voucher number SAL/001, got %s", v.VoucherNumber)
		}
		if v.PartyGSTIN != "29ABCDE1234F1Z5" {
			t.Errorf("expected party GSTIN 29ABCDE1234F1Z5, got %s", v.PartyGSTIN)
		}
		if v.Amount != 11800.0 {
			t.Errorf("expected amount 11800.0, got %f", v.Amount)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	err := client.PushInbound(context.Background(), InboundRequest{
		Vouchers: []InboundVoucher{
			{
				VoucherType:   "Sales",
				VoucherNumber: "SAL/001",
				VoucherDate:   "2026-01-15",
				PartyName:     "Acme Corp",
				PartyGSTIN:    "29ABCDE1234F1Z5",
				Amount:        11800.0,
				Narration:     "Sales invoice",
				LedgerEntries: json.RawMessage(`[{"name":"Sales","amount":-10000}]`),
				TallyMasterID: "master-001",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_AuthHeader(t *testing.T) {
	apiKey := "sk_abc123def456789012345678901234567890123456789012345678901234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expected := "Bearer " + apiKey
		if authHeader != expected {
			t.Errorf("expected Authorization header %q, got %q", expected, authHeader)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, apiKey)
	err := client.Heartbeat(context.Background(), HeartbeatRequest{
		TallyConnected: false,
		Version:        "1.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	err := client.Heartbeat(context.Background(), HeartbeatRequest{
		Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "API error 500: internal server error"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "missing required field: version",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "sk_testkey123")
	_, err := client.Register(context.Background(), RegisterRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "API error 400: INVALID_REQUEST - missing required field: version"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// mustMarshal is a test helper that marshals v to json.RawMessage, failing the test on error.
func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	return json.RawMessage(data)
}

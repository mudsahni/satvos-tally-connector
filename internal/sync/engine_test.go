package sync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mudsahni/satvos-tally-connector/internal/cloud"
	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/store"
	"github.com/mudsahni/satvos-tally-connector/internal/tally"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustJSON marshals v to json.RawMessage, panicking on error.
func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// apiOK returns a SATVOS API success envelope with the given data.
func apiOK(t *testing.T, data interface{}) []byte {
	t.Helper()
	resp := cloud.APIResponse{Success: true}
	if data != nil {
		resp.Data = mustJSON(t, data)
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

// tallyCompanyXML returns a minimal Tally company info XML response.
func tallyCompanyXML(name string) string {
	return `<ENVELOPE><RESULT>` + name + `</RESULT></ENVELOPE>`
}

// tallyMasterXML returns a minimal Tally ledger+stock item+godown+unit+cost center response.
func tallyMasterXML(masterType string) string {
	switch masterType {
	case "Ledger":
		return `<ENVELOPE><BODY><DATA><COLLECTION>
<LEDGER><NAME>Cash</NAME><PARENT>Cash-in-Hand</PARENT><GSTIN></GSTIN><LEDSTATENAME>Maharashtra</LEDSTATENAME><TAXTYPE></TAXTYPE><RATEOFTAXCALCULATION>0</RATEOFTAXCALCULATION><ISREVENUE>No</ISREVENUE></LEDGER>
</COLLECTION></DATA></BODY></ENVELOPE>`
	case "StockItem":
		return `<ENVELOPE><BODY><DATA><COLLECTION>
<STOCKITEM><NAME>Widget</NAME><PARENT>Goods</PARENT><GSTDETAILS.LIST><HSNCODE>8471</HSNCODE></GSTDETAILS.LIST><BASEUNITS>Nos</BASEUNITS></STOCKITEM>
</COLLECTION></DATA></BODY></ENVELOPE>`
	case "Godown":
		return `<ENVELOPE><BODY><DATA><COLLECTION>
<GODOWN><NAME>Main</NAME><PARENT></PARENT></GODOWN>
</COLLECTION></DATA></BODY></ENVELOPE>`
	case "Unit":
		return `<ENVELOPE><BODY><DATA><COLLECTION>
<UNIT><SYMBOL>Nos</SYMBOL><FORMALNAME>Numbers</FORMALNAME></UNIT>
</COLLECTION></DATA></BODY></ENVELOPE>`
	case "CostCentre":
		return `<ENVELOPE><BODY><DATA><COLLECTION>
<COSTCENTRE><NAME>Head Office</NAME><PARENT></PARENT></COSTCENTRE>
</COLLECTION></DATA></BODY></ENVELOPE>`
	default:
		return `<ENVELOPE><BODY><DATA><COLLECTION></COLLECTION></DATA></BODY></ENVELOPE>`
	}
}

// tallyImportSuccessXML returns a successful import response.
func tallyImportSuccessXML() string {
	return `<RESPONSE><CREATED>1</CREATED><ALTERED>0</ALTERED></RESPONSE>`
}

// tallyImportErrorXML returns an import error response.
func tallyImportErrorXML(errMsg string) string {
	return `<RESPONSE><CREATED>0</CREATED><ALTERED>0</ALTERED><LINEERROR>` + errMsg + `</LINEERROR></RESPONSE>`
}

func testConfig() *config.Config {
	return &config.Config{
		Sync: config.SyncConfig{
			IntervalSeconds: 30,
			BatchSize:       50,
			RetryAttempts:   3,
		},
	}
}

func TestEngine_RunCycle_TallyUnavailable(t *testing.T) {
	var heartbeatCalled atomic.Int32
	var mastersCalled atomic.Int32

	// Mock SATVOS cloud server.
	cloudServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sync/v1/heartbeat":
			heartbeatCalled.Add(1)

			body, _ := io.ReadAll(r.Body)
			var req cloud.HeartbeatRequest
			require.NoError(t, json.Unmarshal(body, &req))
			assert.False(t, req.TallyConnected, "heartbeat should indicate tally not connected")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		case "/api/v1/sync/v1/masters":
			mastersCalled.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))
		}
	}))
	defer cloudServer.Close()

	// Mock Tally server that refuses connections (use a server that is already closed).
	tallyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return non-XML to make GetCompanyInfo fail.
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	tallyServer.Close() // Close immediately to simulate Tally being unavailable.

	localStore, err := store.New(t.TempDir())
	require.NoError(t, err)

	cloudClient := cloud.NewClient(cloudServer.URL, "test-key")
	tallyClient := tally.NewClientWithHTTPClient(tallyServer.URL, tallyServer.Client())

	eng := NewEngine(testConfig(), cloudClient, tallyClient, localStore, "1.0.0")
	eng.runCycle(context.Background())

	assert.Equal(t, int32(1), heartbeatCalled.Load(), "heartbeat should be called once")
	assert.Equal(t, int32(0), mastersCalled.Load(), "masters should NOT be pushed when tally is unavailable")
}

func TestEngine_RunCycle_FullCycle(t *testing.T) {
	var heartbeatCalled atomic.Int32
	var mastersCalled atomic.Int32
	var outboundCalled atomic.Int32
	var ackCalled atomic.Int32

	voucherDef := map[string]interface{}{
		"document_id":     "doc-001",
		"voucher_type":    "Purchase",
		"voucher_date":    "2024-01-15",
		"party_ledger":    "Acme Corp",
		"purchase_ledger": "Purchase@18%Gst",
		"tax_entries":     []map[string]interface{}{{"ledger_name": "Input CGST @9%", "amount": 900}},
		"inventory_items": []interface{}{},
		"total_amount":    10900,
		"narration":       "Test purchase",
		"remote_id":       "tenant-doc-001",
	}

	// Mock SATVOS cloud server.
	cloudServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sync/v1/heartbeat":
			heartbeatCalled.Add(1)

			body, _ := io.ReadAll(r.Body)
			var req cloud.HeartbeatRequest
			require.NoError(t, json.Unmarshal(body, &req))
			assert.True(t, req.TallyConnected, "heartbeat should indicate tally connected")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		case "/api/v1/sync/v1/masters":
			mastersCalled.Add(1)

			body, _ := io.ReadAll(r.Body)
			var payload cloud.MasterPayload
			require.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, 1, len(payload.Ledgers), "expected 1 ledger")
			assert.Equal(t, "Cash", payload.Ledgers[0].Name)
			assert.Equal(t, 1, len(payload.StockItems), "expected 1 stock item")
			assert.Equal(t, 1, len(payload.Godowns), "expected 1 godown")
			assert.Equal(t, 1, len(payload.Units), "expected 1 unit")
			assert.Equal(t, 1, len(payload.CostCentres), "expected 1 cost center") //nolint:misspell // Tally uses British spelling

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		case "/api/v1/sync/v1/outbound":
			outboundCalled.Add(1)

			resp := cloud.OutboundResponse{
				Items: []cloud.OutboundItem{
					{
						DocumentID:  "doc-001",
						VoucherDef:  mustJSON(t, voucherDef),
						SyncEventID: "evt-001",
					},
				},
				NextCursor: "cursor-next",
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, resp))

		case "/api/v1/sync/v1/ack":
			ackCalled.Add(1)

			body, _ := io.ReadAll(r.Body)
			var req cloud.AckRequest
			require.NoError(t, json.Unmarshal(body, &req))
			assert.Equal(t, 1, len(req.Results), "expected 1 ack result")
			assert.True(t, req.Results[0].Success, "ack result should be successful")
			assert.Equal(t, "evt-001", req.Results[0].SyncEventID)
			assert.Equal(t, "doc-001", req.Results[0].DocumentID)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))
		}
	}))
	defer cloudServer.Close()

	// Track which Tally requests we've seen to return appropriate responses.
	var companyInfoCalls atomic.Int32

	// Mock Tally server.
	tallyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		w.Header().Set("Content-Type", "text/xml; charset=utf-8")

		// Route based on request content.
		switch {
		case strings.Contains(bodyStr, "$$CurrentCompany"):
			companyInfoCalls.Add(1)
			_, _ = w.Write([]byte(tallyCompanyXML("Test Corp")))

		case strings.Contains(bodyStr, "LedgerList"):
			_, _ = w.Write([]byte(tallyMasterXML("Ledger")))

		case strings.Contains(bodyStr, "StockItemList"):
			_, _ = w.Write([]byte(tallyMasterXML("StockItem")))

		case strings.Contains(bodyStr, "GodownList"):
			_, _ = w.Write([]byte(tallyMasterXML("Godown")))

		case strings.Contains(bodyStr, "UnitList"):
			_, _ = w.Write([]byte(tallyMasterXML("Unit")))

		case strings.Contains(bodyStr, "CostCentreList"):
			_, _ = w.Write([]byte(tallyMasterXML("CostCentre")))

		case strings.Contains(bodyStr, "<TALLYREQUEST>Import</TALLYREQUEST>"):
			_, _ = w.Write([]byte(tallyImportSuccessXML()))

		default:
			_, _ = w.Write([]byte(tallyCompanyXML("Test Corp")))
		}
	}))
	defer tallyServer.Close()

	localStore, err := store.New(t.TempDir())
	require.NoError(t, err)

	cloudClient := cloud.NewClient(cloudServer.URL, "test-key")
	tallyClient := tally.NewClientWithHTTPClient(tallyServer.URL, tallyServer.Client())

	eng := NewEngine(testConfig(), cloudClient, tallyClient, localStore, "1.0.0")
	eng.runCycle(context.Background())

	assert.Equal(t, int32(1), heartbeatCalled.Load(), "heartbeat should be called")
	assert.Equal(t, int32(1), mastersCalled.Load(), "masters should be pushed")
	assert.Equal(t, int32(1), outboundCalled.Load(), "outbound should be pulled")
	assert.Equal(t, int32(1), ackCalled.Load(), "ack should be sent")

	// Verify state was updated.
	state := localStore.Get()
	assert.Equal(t, "Test Corp", state.TallyCompany)
	assert.Equal(t, "cursor-next", state.OutboundCursor)
}

func TestEngine_RunCycle_OutboundImportError(t *testing.T) {
	var ackCalled atomic.Int32

	voucherDef := map[string]interface{}{
		"document_id":     "doc-001",
		"voucher_type":    "Purchase",
		"voucher_date":    "2024-01-15",
		"party_ledger":    "Acme Corp",
		"purchase_ledger": "Purchase@18%Gst",
		"tax_entries":     []interface{}{},
		"inventory_items": []interface{}{},
		"total_amount":    10000,
		"narration":       "Test",
		"remote_id":       "remote-001",
	}

	// Mock SATVOS cloud server.
	cloudServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sync/v1/heartbeat":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		case "/api/v1/sync/v1/masters":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		case "/api/v1/sync/v1/outbound":
			resp := cloud.OutboundResponse{
				Items: []cloud.OutboundItem{
					{
						DocumentID:  "doc-001",
						VoucherDef:  mustJSON(t, voucherDef),
						SyncEventID: "evt-001",
					},
				},
				NextCursor: "cursor-next",
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, resp))

		case "/api/v1/sync/v1/ack":
			ackCalled.Add(1)

			body, _ := io.ReadAll(r.Body)
			var req cloud.AckRequest
			require.NoError(t, json.Unmarshal(body, &req))
			require.Equal(t, 1, len(req.Results))
			assert.False(t, req.Results[0].Success, "ack should indicate failure")
			assert.Contains(t, req.Results[0].ErrorMessage, "Ledger not found")
			assert.Equal(t, "evt-001", req.Results[0].SyncEventID)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(apiOK(t, nil))
		}
	}))
	defer cloudServer.Close()

	// Mock Tally server — import fails.
	tallyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		w.Header().Set("Content-Type", "text/xml; charset=utf-8")

		switch {
		case strings.Contains(bodyStr, "$$CurrentCompany"):
			_, _ = w.Write([]byte(tallyCompanyXML("Test Corp")))

		case strings.Contains(bodyStr, "List"):
			// Master requests — return empty collections.
			_, _ = w.Write([]byte(`<ENVELOPE><BODY><DATA><COLLECTION></COLLECTION></DATA></BODY></ENVELOPE>`))

		case strings.Contains(bodyStr, "<TALLYREQUEST>Import</TALLYREQUEST>"):
			_, _ = w.Write([]byte(tallyImportErrorXML("Ledger not found")))

		default:
			_, _ = w.Write([]byte(tallyCompanyXML("Test Corp")))
		}
	}))
	defer tallyServer.Close()

	localStore, err := store.New(t.TempDir())
	require.NoError(t, err)

	cloudClient := cloud.NewClient(cloudServer.URL, "test-key")
	tallyClient := tally.NewClientWithHTTPClient(tallyServer.URL, tallyServer.Client())

	eng := NewEngine(testConfig(), cloudClient, tallyClient, localStore, "1.0.0")
	eng.runCycle(context.Background())

	assert.Equal(t, int32(1), ackCalled.Load(), "ack should be sent with failure result")
}

func TestEngine_Stop(t *testing.T) {
	// Mock SATVOS cloud server — just accept everything.
	cloudServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiOK(t, nil))
	}))
	defer cloudServer.Close()

	// Mock Tally — unavailable (closed server).
	tallyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	tallyServer.Close()

	localStore, err := store.New(t.TempDir())
	require.NoError(t, err)

	cfg := testConfig()
	cfg.Sync.IntervalSeconds = 60 // Long interval so we don't get extra ticks.

	cloudClient := cloud.NewClient(cloudServer.URL, "test-key")
	tallyClient := tally.NewClientWithHTTPClient(tallyServer.URL, tallyServer.Client())

	eng := NewEngine(cfg, cloudClient, tallyClient, localStore, "1.0.0")

	done := make(chan error, 1)
	go func() {
		done <- eng.Start(context.Background())
	}()

	// Give the engine time to run its initial cycle and enter the select loop.
	time.Sleep(100 * time.Millisecond)

	eng.Stop()

	select {
	case err := <-done:
		assert.NoError(t, err, "engine should stop cleanly without error")
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not stop within timeout")
	}
}

func TestEngine_GetStatus(t *testing.T) {
	// Mock Tally server — returns company info.
	tallyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(tallyCompanyXML("Status Corp")))
	}))
	defer tallyServer.Close()

	// Mock SATVOS cloud server (not used by GetStatus, but needed for engine construction).
	cloudServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiOK(t, nil))
	}))
	defer cloudServer.Close()

	localStore, err := store.New(t.TempDir())
	require.NoError(t, err)

	// Pre-populate state.
	err = localStore.Update(func(s *store.State) {
		s.TallyCompany = "Status Corp"
		s.TallyPort = 9000
		s.AgentID = "agent-xyz"
	})
	require.NoError(t, err)

	cloudClient := cloud.NewClient(cloudServer.URL, "test-key")
	tallyClient := tally.NewClientWithHTTPClient(tallyServer.URL, tallyServer.Client())

	eng := NewEngine(testConfig(), cloudClient, tallyClient, localStore, "1.0.0")

	status := eng.GetStatus()
	assert.True(t, status.TallyConnected, "tally should be connected")
	assert.True(t, status.TallyReachable, "tally should be reachable")
	assert.Equal(t, "Status Corp", status.TallyCompany)
	assert.Equal(t, 9000, status.TallyPort)
	assert.Equal(t, "agent-xyz", status.AgentID)
	assert.Empty(t, status.TallyError)
}

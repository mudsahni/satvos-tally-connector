package tally

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// tallyCompanyInfoResponse is a valid Tally company info XML response.
const tallyCompanyInfoResponse = `<ENVELOPE>
<RESULT>Test Company Pvt Ltd</RESULT>
</ENVELOPE>`

// extractPort parses the port from an httptest.Server's listener address.
func extractPort(t *testing.T, ts *httptest.Server) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to extract port from test server: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port number: %v", err)
	}
	return port
}

func TestDiscoverWithPorts_FindsTally(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(tallyCompanyInfoResponse))
	}))
	defer ts.Close()

	port := extractPort(t, ts)

	// Scan a single-port range that matches the test server.
	got, err := DiscoverWithPorts(context.Background(), "127.0.0.1", port, port)
	if err != nil {
		t.Fatalf("expected discovery to succeed, got error: %v", err)
	}
	if got != port {
		t.Errorf("expected port %d, got %d", port, got)
	}
}

func TestDiscoverWithPorts_FindsTallyInRange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(tallyCompanyInfoResponse))
	}))
	defer ts.Close()

	port := extractPort(t, ts)

	// Scan a range that includes ports before the test server.
	// Non-listening ports will fail quickly, then the test server port succeeds.
	startPort := port - 2
	if startPort < 1 {
		startPort = 1
	}
	endPort := port + 2

	got, err := DiscoverWithPorts(context.Background(), "127.0.0.1", startPort, endPort)
	if err != nil {
		t.Fatalf("expected discovery to succeed, got error: %v", err)
	}
	if got != port {
		t.Errorf("expected port %d, got %d", port, got)
	}
}

func TestDiscoverWithPorts_NoTallyFound(t *testing.T) {
	// Scan a range of ports where nothing is listening.
	// Use high ephemeral ports that are unlikely to be in use.
	_, err := DiscoverWithPorts(context.Background(), "127.0.0.1", 19900, 19902)
	if err == nil {
		t.Fatal("expected error when no Tally instance is running, got nil")
	}
}

func TestDiscoverWithPorts_SkipsNonTallyServer(t *testing.T) {
	// Server that returns invalid XML (not a Tally instance).
	nonTally := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Not Tally</body></html>"))
	}))
	defer nonTally.Close()

	// Real Tally mock.
	tallyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(tallyCompanyInfoResponse))
	}))
	defer tallyServer.Close()

	nonTallyPort := extractPort(t, nonTally)
	tallyPort := extractPort(t, tallyServer)

	// Determine scan range that covers both servers.
	startPort := nonTallyPort
	endPort := tallyPort
	if tallyPort < nonTallyPort {
		startPort = tallyPort
		endPort = nonTallyPort
	}

	got, err := DiscoverWithPorts(context.Background(), "127.0.0.1", startPort, endPort)
	if err != nil {
		t.Fatalf("expected discovery to succeed, got error: %v", err)
	}
	if got != tallyPort {
		t.Errorf("expected Tally port %d, got %d", tallyPort, got)
	}
}

func TestDiscoverWithPorts_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Scan ports that are unlikely to be listening. The context timeout
	// should cause the function to return promptly rather than hanging.
	_, err := DiscoverWithPorts(ctx, "127.0.0.1", 19900, 19910)
	if err == nil {
		t.Fatal("expected error when context is cancelled/timed out, got nil")
	}
}

func TestDiscoverWithPorts_EmptyCompanyName(t *testing.T) {
	// Server that returns an empty company name (invalid Tally response).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<ENVELOPE><RESULT></RESULT></ENVELOPE>`))
	}))
	defer ts.Close()

	port := extractPort(t, ts)

	_, err := DiscoverWithPorts(context.Background(), "127.0.0.1", port, port)
	if err == nil {
		t.Fatal("expected error when Tally returns empty company name, got nil")
	}
}

func TestDiscover_UsesDefaultPortRange(t *testing.T) {
	// Verify Discover delegates to DiscoverWithPorts with default constants.
	// Since no Tally is running on 9000-9010 in the test environment,
	// we just verify it returns an error (confirming it scanned those ports).
	_, err := Discover(context.Background(), "127.0.0.1")
	if err == nil {
		// It's possible (but unlikely) that something is actually running
		// on one of these ports. If so, this test is still valid.
		t.Log("Discover found a service on ports 9000-9010 (unexpected in test env)")
		return
	}
	// Verify the error message mentions the default port range.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "9000") || !strings.Contains(errMsg, "9010") {
		t.Errorf("expected error to mention ports 9000-9010, got: %s", errMsg)
	}
}

func TestDiscoverWithPorts_SinglePortRange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(tallyCompanyInfoResponse))
	}))
	defer ts.Close()

	port := extractPort(t, ts)

	// startPort == endPort should scan exactly one port.
	got, err := DiscoverWithPorts(context.Background(), "127.0.0.1", port, port)
	if err != nil {
		t.Fatalf("expected discovery to succeed for single port, got error: %v", err)
	}
	if got != port {
		t.Errorf("expected port %d, got %d", port, got)
	}
}

func TestDiscoverWithPorts_ServerError(t *testing.T) {
	// Server that returns 500 Internal Server Error.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	port := extractPort(t, ts)

	_, err := DiscoverWithPorts(context.Background(), "127.0.0.1", port, port)
	if err == nil {
		t.Fatal("expected error when server returns 500, got nil")
	}
}

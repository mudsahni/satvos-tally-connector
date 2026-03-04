package tally

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Client communicates with Tally Prime via XML over HTTP.
// Tally exposes an XML server on localhost (default port 9000).
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Tally client pointing at the given host and port.
func NewClient(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithHTTPClient creates a Tally client with a custom http.Client.
// Useful for testing with httptest servers.
func NewClientWithHTTPClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// SendRequest posts raw XML to Tally and returns the response body.
func (c *Client) SendRequest(ctx context.Context, xmlBody []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(xmlBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to tally: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tally response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tally returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// IsAvailable returns true if Tally is reachable and responds with company info.
func (c *Client) IsAvailable(ctx context.Context) bool {
	info, err := c.GetCompanyInfo(ctx)
	if err != nil {
		log.Printf("[tally] availability check failed (%s): %v", c.baseURL, err)
		return false
	}
	log.Printf("[tally] available — company: %s", info.Name)
	return true
}

// CheckStatus returns a richer status for the UI: reachable, company open, error.
func (c *Client) CheckStatus(ctx context.Context) (reachable bool, company, errMsg string) {
	info, err := c.GetCompanyInfo(ctx)
	if err == nil {
		return true, info.Name, ""
	}

	// If the error mentions "no company is open", Tally is reachable but idle.
	errStr := err.Error()
	if strings.Contains(errStr, "no company is open") {
		return true, "", "Tally is running but no company is open"
	}

	return false, "", errStr
}

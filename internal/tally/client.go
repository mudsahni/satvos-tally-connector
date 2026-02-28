package tally

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tally response: %w", err)
	}
	return body, nil
}

// IsAvailable returns true if Tally is reachable and responds with company info.
func (c *Client) IsAvailable(ctx context.Context) bool {
	_, err := c.GetCompanyInfo(ctx)
	return err == nil
}

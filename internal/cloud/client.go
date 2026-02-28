package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the HTTPS client for the SATVOS Sync API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new SATVOS Sync API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// do executes an HTTP request against the SATVOS API and decodes the response.
func (c *Client) do(ctx context.Context, method, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if jsonErr := json.Unmarshal(respBody, &apiResp); jsonErr == nil && apiResp.Error != nil {
			return fmt.Errorf("API error %d: %s - %s", resp.StatusCode, apiResp.Error.Code, apiResp.Error.Message)
		}
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		var apiResp APIResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return fmt.Errorf("parsing response envelope: %w", err)
		}
		if !apiResp.Success {
			if apiResp.Error != nil {
				return fmt.Errorf("API error: %s - %s", apiResp.Error.Code, apiResp.Error.Message)
			}
			return fmt.Errorf("API returned success=false")
		}
		if err := json.Unmarshal(apiResp.Data, result); err != nil {
			return fmt.Errorf("parsing response data: %w", err)
		}
	}

	return nil
}

// Register registers this agent with the SATVOS server.
func (c *Client) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	var resp RegisterResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/sync/v1/register", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Heartbeat sends a heartbeat to the SATVOS server.
func (c *Client) Heartbeat(ctx context.Context, req HeartbeatRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v1/sync/v1/heartbeat", req, nil)
}

// PushMasters uploads master data (ledgers, stock items, etc.) to the SATVOS server.
func (c *Client) PushMasters(ctx context.Context, payload MasterPayload) error {
	return c.do(ctx, http.MethodPost, "/api/v1/sync/v1/masters", payload, nil)
}

// PullOutbound fetches outbound items (documents to push to Tally) from the SATVOS server.
func (c *Client) PullOutbound(ctx context.Context, cursor string, limit int) (*OutboundResponse, error) {
	path := fmt.Sprintf("/api/v1/sync/v1/outbound?cursor=%s&limit=%d", cursor, limit)
	var resp OutboundResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Ack acknowledges processing results for outbound items.
func (c *Client) Ack(ctx context.Context, req AckRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v1/sync/v1/ack", req, nil)
}

// PushInbound uploads inbound vouchers from Tally to the SATVOS server.
func (c *Client) PushInbound(ctx context.Context, req InboundRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v1/sync/v1/inbound", req, nil)
}

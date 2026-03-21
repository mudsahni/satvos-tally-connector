package tally

import (
	"context"
	"encoding/xml"
	"fmt"
)

// ImportResult holds the result of a voucher import into Tally.
type ImportResult struct {
	Success    bool
	Created    int
	Altered    int
	LastVchID  string
	LastMID    string
	Errors     []string
}

// ImportVoucher imports a voucher XML string into Tally.
func (c *Client) ImportVoucher(ctx context.Context, voucherXML, companyName string) (*ImportResult, error) {
	reqXML := BuildVoucherImportRequest(voucherXML, companyName)
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseImportResponse(resp)
}

// ImportMaster imports master data XML (ledgers, groups, stock items) into Tally.
// Uses DUPIGNORECOMBINE to skip entries that already exist.
func (c *Client) ImportMaster(ctx context.Context, masterXML, companyName string) (*ImportResult, error) {
	reqXML := BuildMasterImportRequest(masterXML, companyName)
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseImportResponse(resp)
}

// ParseImportResponse parses Tally's import result XML.
// Tally returns an envelope with CREATED/ALTERED counts and LINEERROR for failures.
func ParseImportResponse(data []byte) (*ImportResult, error) {
	// Tally import responses have two possible structures:
	// Flat: <RESPONSE><CREATED>1</CREATED>...</RESPONSE>
	// Envelope: <ENVELOPE><BODY><DATA><IMPORTRESULT><CREATED>1</CREATED>...</IMPORTRESULT></DATA></BODY></ENVELOPE>
	type importResponse struct {
		XMLName   xml.Name `xml:"RESPONSE"`
		Created   int      `xml:"CREATED"`
		Altered   int      `xml:"ALTERED"`
		LastVchID string   `xml:"LASTVCHID"`
		LastMID   string   `xml:"LASTMID"`
		LineError string   `xml:"LINEERROR"`
	}

	// Try structured parsing first.
	var resp importResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		// If structured parsing fails, try envelope wrapper.
		type envelopeImportResponse struct {
			Created   int    `xml:"CREATED"`
			Altered   int    `xml:"ALTERED"`
			LastVchID string `xml:"LASTVCHID"`
			LastMID   string `xml:"LASTMID"`
			LineError string `xml:"LINEERROR"`
		}
		type envelope struct {
			XMLName  xml.Name               `xml:"ENVELOPE"`
			Response envelopeImportResponse `xml:"BODY>DATA>IMPORTRESULT"`
		}
		var env envelope
		if envErr := xml.Unmarshal(data, &env); envErr != nil {
			return nil, fmt.Errorf("parsing import response: %w (raw: %s)", err, string(data))
		}
		resp = importResponse{
			Created:   env.Response.Created,
			Altered:   env.Response.Altered,
			LastVchID: env.Response.LastVchID,
			LastMID:   env.Response.LastMID,
			LineError: env.Response.LineError,
		}
	}

	result := &ImportResult{
		Created:   resp.Created,
		Altered:   resp.Altered,
		LastVchID: resp.LastVchID,
		LastMID:   resp.LastMID,
	}

	if resp.LineError != "" {
		result.Success = false
		result.Errors = append(result.Errors, resp.LineError)
		return result, nil
	}

	// Success only if at least one record was created or altered.
	if resp.Created > 0 || resp.Altered > 0 {
		result.Success = true
	} else {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf(
			"Tally processed the request but created 0 and altered 0 records (raw response: %s)",
			truncate(string(data), 500),
		))
	}

	return result, nil
}


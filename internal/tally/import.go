package tally

import (
	"context"
	"encoding/xml"
	"fmt"
)

// ImportResult holds the result of a voucher import into Tally.
type ImportResult struct {
	Success bool
	Created int
	Altered int
	Errors  []string
}

// ImportVoucher imports a voucher XML string into Tally.
func (c *Client) ImportVoucher(ctx context.Context, voucherXML string) (*ImportResult, error) {
	reqXML := BuildVoucherImportRequest(voucherXML)
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseImportResponse(resp)
}

// ParseImportResponse parses Tally's import result XML.
// Tally returns an envelope with CREATED/ALTERED counts and LINEERROR for failures.
func ParseImportResponse(data []byte) (*ImportResult, error) {
	// Tally import responses have this structure:
	// <RESPONSE>
	//   <CREATED>1</CREATED>
	//   <ALTERED>0</ALTERED>
	//   <LASTVCHID>...</LASTVCHID>
	//   <LASTMID>...</LASTMID>
	// </RESPONSE>
	// Or on error:
	// <RESPONSE>
	//   <CREATED>0</CREATED>
	//   <ALTERED>0</ALTERED>
	//   <LINEERROR>error description</LINEERROR>
	// </RESPONSE>
	type importResponse struct {
		XMLName   xml.Name `xml:"RESPONSE"`
		Created   int      `xml:"CREATED"`
		Altered   int      `xml:"ALTERED"`
		LineError string   `xml:"LINEERROR"`
	}

	// Try structured parsing first.
	var resp importResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		// If structured parsing fails, try envelope wrapper.
		type envelope struct {
			XMLName  xml.Name       `xml:"ENVELOPE"`
			Response importResponse `xml:"BODY>DATA>IMPORTRESULT"`
		}
		var env envelope
		if envErr := xml.Unmarshal(data, &env); envErr != nil {
			return nil, fmt.Errorf("parsing import response: %w", err)
		}
		resp = env.Response
	}

	result := &ImportResult{
		Created: resp.Created,
		Altered: resp.Altered,
		Success: true,
	}

	if resp.LineError != "" {
		result.Success = false
		result.Errors = append(result.Errors, resp.LineError)
	}

	// If nothing was created or altered and no explicit error, still mark success
	// based on whether there was actually something processed.
	if resp.Created == 0 && resp.Altered == 0 && resp.LineError == "" {
		// Could be an empty import -- still technically successful.
		result.Success = true
	}

	return result, nil
}

package tally

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"strings"
)

// ImportResult holds the result of a voucher import into Tally.
type ImportResult struct {
	Success         bool
	Created         int
	Altered         int
	Exceptions      int
	LastVchID       string
	LastMID         string
	Errors          []string
	IsZeroCountOnly bool // true when the only "error" is zero created/altered counts (no LINEERROR or EXCEPTIONS)
}

// ImportVoucher imports a voucher XML string into Tally.
func (c *Client) ImportVoucher(ctx context.Context, voucherXML, companyName string) (*ImportResult, error) {
	reqXML := BuildVoucherImportRequest(voucherXML, companyName)
	log.Printf("[tally] sending voucher import XML (%d bytes): %s", len(reqXML), truncate(string(reqXML), 2000))
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	log.Printf("[tally] voucher import response (%d bytes): %s", len(resp), string(resp))
	return ParseImportResponse(resp)
}

// ImportMaster imports master data XML (ledgers, groups, stock items) into Tally.
// Uses DUPIGNORECOMBINE to skip entries that already exist.
func (c *Client) ImportMaster(ctx context.Context, masterXML, companyName string) (*ImportResult, error) {
	reqXML := BuildMasterImportRequest(masterXML, companyName)
	log.Printf("[tally] sending master import XML (%d bytes)", len(reqXML))
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	log.Printf("[tally] master import response: %s", truncate(string(resp), 1000))
	return ParseImportResponse(resp)
}

// ParseImportResponse parses Tally's import result XML.
func ParseImportResponse(data []byte) (*ImportResult, error) {
	// Try envelope format first (most common for Tally Prime):
	// <ENVELOPE><BODY><DATA><IMPORTRESULT>...</IMPORTRESULT></DATA></BODY></ENVELOPE>
	type envelopeResult struct {
		Created    int    `xml:"CREATED"`
		Altered    int    `xml:"ALTERED"`
		Deleted    int    `xml:"DELETED"`
		Exceptions int    `xml:"EXCEPTIONS"`
		Errors     int    `xml:"ERRORS"`
		Canceled   int    `xml:"CANCELLED"` //nolint:misspell // Tally uses British spelling
		Ignored    int    `xml:"IGNORED"`
		Combined   int    `xml:"COMBINED"`
		LastVchID  string `xml:"LASTVCHID"`
		LastMID    string `xml:"LASTMID"`
		LineError  string `xml:"LINEERROR"`
	}
	type envelope struct {
		XMLName  xml.Name       `xml:"ENVELOPE"`
		Status   int            `xml:"HEADER>STATUS"`
		Result   envelopeResult `xml:"BODY>DATA>IMPORTRESULT"`
	}

	var env envelope
	if err := xml.Unmarshal(data, &env); err == nil && env.Status > 0 {
		return buildResult(env.Result.Created, env.Result.Altered, env.Result.Exceptions,
			env.Result.Errors, env.Result.LastVchID, env.Result.LastMID,
			env.Result.LineError, data), nil
	}

	// Fallback: flat <RESPONSE> format
	type flatResponse struct {
		XMLName    xml.Name `xml:"RESPONSE"`
		Created    int      `xml:"CREATED"`
		Altered    int      `xml:"ALTERED"`
		Exceptions int      `xml:"EXCEPTIONS"`
		LastVchID  string   `xml:"LASTVCHID"`
		LastMID    string   `xml:"LASTMID"`
		LineError  string   `xml:"LINEERROR"`
	}

	var flat flatResponse
	if err := xml.Unmarshal(data, &flat); err == nil {
		return buildResult(flat.Created, flat.Altered, flat.Exceptions,
			0, flat.LastVchID, flat.LastMID,
			flat.LineError, data), nil
	}

	// If both fail, return the raw response as error
	return &ImportResult{
		Success: false,
		Errors:  []string{fmt.Sprintf("failed to parse Tally response: %s", truncate(string(data), 1000))},
	}, nil
}

func buildResult(created, altered, exceptions, errCount int, lastVchID, lastMID, lineError string, rawData []byte) *ImportResult {
	result := &ImportResult{
		Created:    created,
		Altered:    altered,
		Exceptions: exceptions,
		LastVchID:  lastVchID,
		LastMID:    lastMID,
	}

	// Collect all error indicators
	var errMsgs []string

	if lineError != "" {
		errMsgs = append(errMsgs, lineError)
	}

	if exceptions > 0 {
		// Extract exception details from raw XML — look for common patterns
		rawStr := string(rawData)
		details := extractExceptionDetails(rawStr)
		if details != "" {
			errMsgs = append(errMsgs, fmt.Sprintf("Tally exception: %s", details))
		} else {
			errMsgs = append(errMsgs, fmt.Sprintf("Tally reported %d exception(s) but no details available", exceptions))
		}
	}

	if errCount > 0 && lineError == "" {
		errMsgs = append(errMsgs, fmt.Sprintf("Tally reported %d error(s)", errCount))
	}

	// Determine success
	switch {
	case len(errMsgs) > 0:
		result.Success = false
		result.Errors = errMsgs
	case created > 0 || altered > 0:
		result.Success = true
	default:
		result.Success = false
		result.IsZeroCountOnly = true
		result.Errors = []string{fmt.Sprintf(
			"Tally created 0 and altered 0 records (exceptions=%d, errors=%d, raw: %s)",
			exceptions, errCount, truncate(string(rawData), 500),
		)}
	}

	return result
}

// extractExceptionDetails looks for common exception patterns in Tally XML responses.
func extractExceptionDetails(rawXML string) string {
	// Look for LINEERROR tags that might be nested in unexpected places
	patterns := []string{"<LINEERROR>", "<ERRORMSG>", "<EXCEPTIONMSG>", "<ERRORDESC>"}
	for _, p := range patterns {
		endTag := strings.Replace(p, "<", "</", 1)
		start := strings.Index(rawXML, p)
		if start >= 0 {
			start += len(p)
			end := strings.Index(rawXML[start:], endTag)
			if end >= 0 {
				return strings.TrimSpace(rawXML[start : start+end])
			}
		}
	}
	return ""
}

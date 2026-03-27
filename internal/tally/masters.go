package tally

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
)

// xmlEsc escapes XML special characters.
func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// deducteeTypeFromPAN derives the TDS deductee type from the 4th character of a PAN.
func deducteeTypeFromPAN(pan string) string {
	if len(pan) < 4 {
		return ""
	}
	switch pan[3] {
	case 'C', 'c':
		return "Company"
	case 'F', 'f':
		return "Firm"
	case 'T', 't':
		return "Trust"
	case 'H', 'h':
		return "HUF"
	case 'A', 'a':
		return "AOP/BOI"
	case 'P', 'p':
		return "Individual"
	default:
		return "Individual"
	}
}

// LedgerDef describes a ledger to create in Tally.
type LedgerDef struct {
	Name        string
	ParentGroup string
	Address     string
	PAN         string
	GSTIN       string
	State       string
}

var ledgerTemplate = template.Must(template.New("ledger").Funcs(template.FuncMap{
	"xmlEscape":           xmlEsc,
	"deducteeTypeFromPAN": deducteeTypeFromPAN,
}).Parse(ledgerXMLTemplate))

const ledgerXMLTemplate = `<LEDGER NAME="{{.Name | xmlEscape}}" ACTION="Create">
<NAME>{{.Name | xmlEscape}}</NAME>
<PARENT>{{.ParentGroup | xmlEscape}}</PARENT>
{{- if .Address}}
<ADDRESS.LIST TYPE="String">
<ADDRESS>{{.Address | xmlEscape}}</ADDRESS>
</ADDRESS.LIST>
{{- end}}
{{- if .PAN}}
<INCOMETAXNUMBER>{{.PAN | xmlEscape}}</INCOMETAXNUMBER>
<ISTDSAPPLICABLE>Yes</ISTDSAPPLICABLE>
<TDSDEDUCTEETYPE>{{deducteeTypeFromPAN .PAN}}</TDSDEDUCTEETYPE>
{{- end}}
{{- if .GSTIN}}
<PARTYGSTIN>{{.GSTIN | xmlEscape}}</PARTYGSTIN>
<GSTREGISTRATIONTYPE>Regular</GSTREGISTRATIONTYPE>
{{- end}}
{{- if .State}}
<LEDSTATENAME>{{.State | xmlEscape}}</LEDSTATENAME>
{{- end}}
</LEDGER>`

// BuildLedgerXML creates XML for a single Tally ledger.
func BuildLedgerXML(def LedgerDef) string {
	var buf bytes.Buffer
	_ = ledgerTemplate.Execute(&buf, def)
	return buf.String()
}

// EnsureLedgersExist creates any ledgers in Tally that don't already exist.
// Uses DUPIGNORECOMBINE so existing ledgers are silently skipped.
func (c *Client) EnsureLedgersExist(ctx context.Context, companyName string, ledgers []LedgerDef) error {
	if len(ledgers) == 0 {
		return nil
	}

	var xmlParts []string
	for _, l := range ledgers {
		xmlParts = append(xmlParts, BuildLedgerXML(l))
	}
	combinedXML := strings.Join(xmlParts, "\n")

	result, err := c.ImportMaster(ctx, combinedXML, companyName)
	if err != nil {
		return fmt.Errorf("creating ledgers in Tally: %w", err)
	}

	// With DUPIGNORECOMBINE, existing ledgers are skipped.
	// Only report errors from LINEERROR, not from counts.
	for _, e := range result.Errors {
		if strings.Contains(e, "Tally created 0") || strings.Contains(e, "Tally processed") {
			continue // These are count-based messages, not real errors for master import
		}
		return fmt.Errorf("tally ledger creation errors: %s", e)
	}

	return nil
}

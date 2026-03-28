package tally

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/mudsahni/satvos-tally-connector/internal/xmlutil"
)


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
	GSTRegType  string // e.g. "Regular", "Composition", "Unregistered"; defaults to "Regular"
}

var ledgerTemplate = template.Must(template.New("ledger").Funcs(template.FuncMap{
	"xmlEscape":           xmlutil.Escape,
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
<TDSDEDUCTEETYPE>{{deducteeTypeFromPAN .PAN | xmlEscape}}</TDSDEDUCTEETYPE>
{{- end}}
{{- if .GSTIN}}
<PARTYGSTIN>{{.GSTIN | xmlEscape}}</PARTYGSTIN>
<GSTREGISTRATIONTYPE>{{.GSTRegType | xmlEscape}}</GSTREGISTRATIONTYPE>
{{- end}}
{{- if .State}}
<LEDSTATENAME>{{.State | xmlEscape}}</LEDSTATENAME>
{{- end}}
</LEDGER>`

// BuildLedgerXML creates XML for a single Tally ledger.
func BuildLedgerXML(def *LedgerDef) (string, error) {
	// Default GSTRegType if not set.
	if def.GSTRegType == "" {
		def.GSTRegType = "Regular"
	}
	var buf bytes.Buffer
	if err := ledgerTemplate.Execute(&buf, def); err != nil {
		return "", fmt.Errorf("executing ledger template: %w", err)
	}
	return buf.String(), nil
}

// EnsureLedgersExist creates any ledgers in Tally that don't already exist.
// Uses DUPIGNORECOMBINE so existing ledgers are silently skipped.
func (c *Client) EnsureLedgersExist(ctx context.Context, companyName string, ledgers []LedgerDef) error {
	if len(ledgers) == 0 {
		return nil
	}

	var xmlParts []string
	for i := range ledgers {
		part, err := BuildLedgerXML(&ledgers[i])
		if err != nil {
			return fmt.Errorf("building ledger XML for %q: %w", ledgers[i].Name, err)
		}
		xmlParts = append(xmlParts, part)
	}
	combinedXML := strings.Join(xmlParts, "\n")

	result, err := c.ImportMaster(ctx, combinedXML, companyName)
	if err != nil {
		return fmt.Errorf("creating ledgers in Tally: %w", err)
	}

	// With DUPIGNORECOMBINE, existing ledgers are skipped, so CREATED=0 ALTERED=0
	// is expected. Only report real errors (LINEERROR, EXCEPTIONS).
	if result.IsZeroCountOnly {
		return nil
	}
	for _, e := range result.Errors {
		return fmt.Errorf("tally ledger creation errors: %s", e)
	}

	return nil
}

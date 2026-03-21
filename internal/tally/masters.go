package tally

import (
	"context"
	"fmt"
	"strings"
)

// xmlEsc escapes XML special characters.
func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// BuildLedgerXML creates XML for a single Tally ledger.
// parentGroup should be a standard Tally group like "Sundry Creditors",
// "Purchase Accounts", "Duties & Taxes", etc.
func BuildLedgerXML(name, parentGroup string) string {
	return fmt.Sprintf(`<LEDGER NAME="%s" ACTION="Create">
<NAME>%s</NAME>
<PARENT>%s</PARENT>
</LEDGER>`, xmlEsc(name), xmlEsc(name), xmlEsc(parentGroup))
}

// EnsureLedgersExist creates any ledgers in Tally that don't already exist.
// Uses DUPIGNORECOMBINE so existing ledgers are silently skipped.
// Each entry is a {name, parentGroup} pair.
func (c *Client) EnsureLedgersExist(ctx context.Context, companyName string, ledgers []LedgerDef) error {
	if len(ledgers) == 0 {
		return nil
	}

	var xmlParts []string
	for _, l := range ledgers {
		xmlParts = append(xmlParts, BuildLedgerXML(l.Name, l.ParentGroup))
	}
	combinedXML := strings.Join(xmlParts, "\n")

	result, err := c.ImportMaster(ctx, combinedXML, companyName)
	if err != nil {
		return fmt.Errorf("creating ledgers in Tally: %w", err)
	}

	// With DUPIGNORECOMBINE, existing ledgers are ignored (Created=0 is fine).
	// Only real errors matter here.
	if len(result.Errors) > 0 {
		return fmt.Errorf("Tally ledger creation errors: %s", strings.Join(result.Errors, "; "))
	}

	return nil
}

// LedgerDef describes a ledger to create in Tally.
type LedgerDef struct {
	Name        string
	ParentGroup string
}

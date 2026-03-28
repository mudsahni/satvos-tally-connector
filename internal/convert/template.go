package convert

import (
	"text/template"

	"github.com/mudsahni/satvos-tally-connector/internal/xmlutil"
)

var voucherTemplate = template.Must(template.New("voucher").Funcs(template.FuncMap{
	"xmlEscape": xmlEscape,
	"neg":       func(f float64) float64 { return -f },
}).Parse(voucherXMLTemplate))

func xmlEscape(s string) string {
	return xmlutil.Escape(s)
}

const voucherXMLTemplate = `<VOUCHER REMOTEID="{{.RemoteID | xmlEscape}}" VCHTYPE="{{.VoucherTypeName | xmlEscape}}" ACTION="Create">
<DATE>{{.TallyDate}}</DATE>
<VOUCHERTYPENAME>{{.VoucherTypeName | xmlEscape}}</VOUCHERTYPENAME>
<VOUCHERNUMBER></VOUCHERNUMBER>
{{- if .SupplierInvoiceNo}}
<REFERENCE>{{.SupplierInvoiceNo | xmlEscape}}</REFERENCE>
{{- end}}
{{- if .SupplierInvoiceDate}}
<REFERENCEDATE>{{.SupplierInvoiceDate}}</REFERENCEDATE>
{{- end}}
<ISINVOICE>{{.IsInvoice}}</ISINVOICE>
<PERSISTEDVIEW>{{.PersistedView}}</PERSISTEDVIEW>
<NARRATION>{{.Narration | xmlEscape}}</NARRATION>
<PARTYLEDGERNAME>{{.PartyLedger | xmlEscape}}</PARTYLEDGERNAME>
<ALLLEDGERENTRIES.LIST>
<LEDGERNAME>{{.PartyLedger | xmlEscape}}</LEDGERNAME>
<ISDEEMEDPOSITIVE>No</ISDEEMEDPOSITIVE>
<AMOUNT>{{printf "%.2f" .TotalAmount}}</AMOUNT>
{{- if .SupplierInvoiceNo}}
<BILLALLOCATIONS.LIST>
<NAME>{{.SupplierInvoiceNo | xmlEscape}}</NAME>
<BILLTYPE>New Ref</BILLTYPE>
<AMOUNT>{{printf "%.2f" .TotalAmount}}</AMOUNT>
</BILLALLOCATIONS.LIST>
{{- end}}
</ALLLEDGERENTRIES.LIST>
<ALLLEDGERENTRIES.LIST>
<LEDGERNAME>{{.PurchaseLedger | xmlEscape}}</LEDGERNAME>
<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>
<AMOUNT>{{printf "%.2f" (neg .PurchaseAmount)}}</AMOUNT>
</ALLLEDGERENTRIES.LIST>
{{- if ne .VoucherMode "journal"}}
{{- range .TaxEntries}}
<ALLLEDGERENTRIES.LIST>
<LEDGERNAME>{{.LedgerName | xmlEscape}}</LEDGERNAME>
<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>
<AMOUNT>{{printf "%.2f" (neg .Amount)}}</AMOUNT>
</ALLLEDGERENTRIES.LIST>
{{- end}}
{{- end}}
{{- if eq .VoucherMode "item_invoice"}}
{{- range .InventoryItems}}
<ALLINVENTORYENTRIES.LIST>
<STOCKITEMNAME>{{.StockItem | xmlEscape}}</STOCKITEMNAME>
<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>
<RATE>{{printf "%.2f" .Rate}}/{{.UOM | xmlEscape}}</RATE>
<AMOUNT>{{printf "%.2f" (neg .Amount)}}</AMOUNT>
<ACTUALQTY>{{printf "%.2f" .Quantity}} {{.UOM | xmlEscape}}</ACTUALQTY>
<BILLEDQTY>{{printf "%.2f" .Quantity}} {{.UOM | xmlEscape}}</BILLEDQTY>
{{- if .Godown}}
<BATCHALLOCATIONS.LIST>
<GODOWNNAME>{{.Godown | xmlEscape}}</GODOWNNAME>
<AMOUNT>{{printf "%.2f" (neg .Amount)}}</AMOUNT>
<ACTUALQTY>{{printf "%.2f" .Quantity}} {{.UOM | xmlEscape}}</ACTUALQTY>
<BILLEDQTY>{{printf "%.2f" .Quantity}} {{.UOM | xmlEscape}}</BILLEDQTY>
</BATCHALLOCATIONS.LIST>
{{- end}}
</ALLINVENTORYENTRIES.LIST>
{{- end}}
{{- end}}
</VOUCHER>`

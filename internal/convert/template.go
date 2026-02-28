package convert

import (
	"strings"
	"text/template"
)

var voucherTemplate = template.Must(template.New("voucher").Funcs(template.FuncMap{
	"xmlEscape": xmlEscape,
	"neg":       func(f float64) float64 { return -f },
}).Parse(voucherXMLTemplate))

var xmlReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")

func xmlEscape(s string) string {
	return xmlReplacer.Replace(s)
}

const voucherXMLTemplate = `<VOUCHER REMOTEID="{{.RemoteID | xmlEscape}}" VCHTYPE="{{.VoucherType | xmlEscape}}" ACTION="Create">
<DATE>{{.TallyDate}}</DATE>
<VOUCHERTYPENAME>{{.VoucherType | xmlEscape}}</VOUCHERTYPENAME>
<VOUCHERNUMBER></VOUCHERNUMBER>
<NARRATION>{{.Narration | xmlEscape}}</NARRATION>
<PARTYLEDGERNAME>{{.PartyLedger | xmlEscape}}</PARTYLEDGERNAME>
<ALLLEDGERENTRIES.LIST>
<LEDGERNAME>{{.PartyLedger | xmlEscape}}</LEDGERNAME>
<ISDEEMEDPOSITIVE>No</ISDEEMEDPOSITIVE>
<AMOUNT>{{printf "%.2f" .TotalAmount}}</AMOUNT>
</ALLLEDGERENTRIES.LIST>
<ALLLEDGERENTRIES.LIST>
<LEDGERNAME>{{.PurchaseLedger | xmlEscape}}</LEDGERNAME>
<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>
<AMOUNT>{{printf "%.2f" (neg .PurchaseAmount)}}</AMOUNT>
</ALLLEDGERENTRIES.LIST>
{{- range .TaxEntries}}
<ALLLEDGERENTRIES.LIST>
<LEDGERNAME>{{.LedgerName | xmlEscape}}</LEDGERNAME>
<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>
<AMOUNT>{{printf "%.2f" (neg .Amount)}}</AMOUNT>
</ALLLEDGERENTRIES.LIST>
{{- end}}
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
</VOUCHER>`

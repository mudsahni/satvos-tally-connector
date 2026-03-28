package tally

import (
	"fmt"

	"github.com/mudsahni/satvos-tally-connector/internal/xmlutil"
)

// BuildCompanyInfoRequest creates an XML request to get the current company info.
func BuildCompanyInfoRequest() []byte {
	return []byte(`<ENVELOPE>
<HEADER>
<VERSION>1</VERSION>
<TALLYREQUEST>Export</TALLYREQUEST>
<TYPE>Function</TYPE>
<ID>$$CurrentCompany</ID>
</HEADER>
<BODY>
<DESC>
<STATICVARIABLES>
<SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT>
</STATICVARIABLES>
</DESC>
</BODY>
</ENVELOPE>`)
}

// BuildMasterExportRequest creates an XML request to export master data.
// masterType can be: "Ledger", "StockItem", "Godown", "Unit", "CostCentre".
func BuildMasterExportRequest(masterType string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE>
<HEADER>
<VERSION>1</VERSION>
<TALLYREQUEST>Export</TALLYREQUEST>
<TYPE>Collection</TYPE>
<ID>%sList</ID>
</HEADER>
<BODY>
<DESC>
<STATICVARIABLES>
<SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT>
</STATICVARIABLES>
<TDL>
<TDLMESSAGE>
<COLLECTION NAME="%sList" ISMODIFY="No">
<TYPE>%s</TYPE>
<FETCH>*</FETCH>
</COLLECTION>
</TDLMESSAGE>
</TDL>
</DESC>
</BODY>
</ENVELOPE>`, masterType, masterType, masterType))
}

// BuildVoucherImportRequest wraps voucher XML for import into Tally.
func BuildVoucherImportRequest(voucherXML, companyName string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE>
<HEADER>
<VERSION>1</VERSION>
<TALLYREQUEST>Import</TALLYREQUEST>
<TYPE>Data</TYPE>
<ID>Vouchers</ID>
</HEADER>
<BODY>
<DESC>
<STATICVARIABLES>
<SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY>
</STATICVARIABLES>
</DESC>
<DATA>
<TALLYMESSAGE>
%s
</TALLYMESSAGE>
</DATA>
</BODY>
</ENVELOPE>`, xmlutil.Escape(companyName), voucherXML))
}

// BuildMasterImportRequest wraps master XML (ledgers, groups, stock items)
// for import into Tally. Uses DUPIGNORECOMBINE to skip existing entries.
func BuildMasterImportRequest(masterXML, companyName string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE>
<HEADER>
<VERSION>1</VERSION>
<TALLYREQUEST>Import</TALLYREQUEST>
<TYPE>Data</TYPE>
<ID>All Masters</ID>
</HEADER>
<BODY>
<DESC>
<STATICVARIABLES>
<SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY>
<IMPORTDUPS>@@DUPIGNORECOMBINE</IMPORTDUPS>
</STATICVARIABLES>
</DESC>
<DATA>
<TALLYMESSAGE>
%s
</TALLYMESSAGE>
</DATA>
</BODY>
</ENVELOPE>`, xmlutil.Escape(companyName), masterXML))
}

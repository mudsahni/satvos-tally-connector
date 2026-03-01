package tally

import (
	"encoding/xml"
	"fmt"
)

// CompanyInfo holds basic Tally company information.
type CompanyInfo struct {
	Name string
}

// ParseCompanyInfoResponse extracts the company name from Tally's response.
// Tally returns: <ENVELOPE><RESULT>CompanyName</RESULT></ENVELOPE>
func ParseCompanyInfoResponse(data []byte) (*CompanyInfo, error) {
	type envelope struct {
		XMLName xml.Name `xml:"ENVELOPE"`
		Result  string   `xml:"RESULT"`
	}
	var env envelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing company info: %w", err)
	}
	if env.Result == "" {
		return nil, fmt.Errorf("empty company name in response")
	}
	return &CompanyInfo{Name: env.Result}, nil
}

// TallyLedger represents a ledger from Tally's export response.
type TallyLedger struct {
	Name        string  `xml:"NAME"`
	Parent      string  `xml:"PARENT"`
	GSTNumber   string  `xml:"GSTIN"`
	LedgerState string  `xml:"LEDSTATENAME"`
	TaxType     string  `xml:"TAXTYPE"`
	RateOfTax   float64 `xml:"RATEOFTAXCALCULATION"`
	IsRevenue   string  `xml:"ISREVENUE"`
}

// TallyStockItem represents a stock item from Tally's export response.
type TallyStockItem struct {
	Name     string `xml:"NAME"`
	Parent   string `xml:"PARENT"`
	HSNCode  string `xml:"GSTDETAILS.LIST>HSNCODE"`
	BaseUnit string `xml:"BASEUNITS"`
}

// TallyGodown represents a godown (warehouse) from Tally's export response.
type TallyGodown struct {
	Name   string `xml:"NAME"`
	Parent string `xml:"PARENT"`
}

// TallyUnit represents a unit of measurement from Tally's export response.
type TallyUnit struct {
	Symbol     string `xml:"SYMBOL"`
	FormalName string `xml:"FORMALNAME"`
}

// TallyCostCentre represents a cost center from Tally's export response. //nolint:misspell // Tally uses British spelling
type TallyCostCentre struct {
	Name   string `xml:"NAME"`
	Parent string `xml:"PARENT"`
}

// ParseLedgerResponse parses a Tally ledger master export response.
func ParseLedgerResponse(data []byte) ([]TallyLedger, error) {
	type envelope struct {
		XMLName xml.Name      `xml:"ENVELOPE"`
		Ledgers []TallyLedger `xml:"BODY>DATA>COLLECTION>LEDGER"`
	}
	var env envelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing ledger response: %w", err)
	}
	return env.Ledgers, nil
}

// ParseStockItemResponse parses a Tally stock item master export response.
func ParseStockItemResponse(data []byte) ([]TallyStockItem, error) {
	type envelope struct {
		XMLName    xml.Name         `xml:"ENVELOPE"`
		StockItems []TallyStockItem `xml:"BODY>DATA>COLLECTION>STOCKITEM"`
	}
	var env envelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing stock item response: %w", err)
	}
	return env.StockItems, nil
}

// ParseGodownResponse parses a Tally godown master export response.
func ParseGodownResponse(data []byte) ([]TallyGodown, error) {
	type envelope struct {
		XMLName xml.Name      `xml:"ENVELOPE"`
		Godowns []TallyGodown `xml:"BODY>DATA>COLLECTION>GODOWN"`
	}
	var env envelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing godown response: %w", err)
	}
	return env.Godowns, nil
}

// ParseUnitResponse parses a Tally unit master export response.
func ParseUnitResponse(data []byte) ([]TallyUnit, error) {
	type envelope struct {
		XMLName xml.Name    `xml:"ENVELOPE"`
		Units   []TallyUnit `xml:"BODY>DATA>COLLECTION>UNIT"`
	}
	var env envelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing unit response: %w", err)
	}
	return env.Units, nil
}

// ParseCostCentreResponse parses a Tally cost center master export response. //nolint:misspell // Tally uses British spelling
func ParseCostCentreResponse(data []byte) ([]TallyCostCentre, error) {
	type envelope struct {
		XMLName     xml.Name          `xml:"ENVELOPE"`
		CostCentres []TallyCostCentre `xml:"BODY>DATA>COLLECTION>COSTCENTRE"`
	}
	var env envelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing cost center response: %w", err)
	}
	return env.CostCentres, nil
}

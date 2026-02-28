package tally

import "context"

// GetCompanyInfo fetches the currently active company name from Tally.
func (c *Client) GetCompanyInfo(ctx context.Context) (*CompanyInfo, error) {
	reqXML := BuildCompanyInfoRequest()
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseCompanyInfoResponse(resp)
}

// GetLedgers exports all ledger master data from Tally.
func (c *Client) GetLedgers(ctx context.Context) ([]TallyLedger, error) {
	reqXML := BuildMasterExportRequest("Ledger")
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseLedgerResponse(resp)
}

// GetStockItems exports all stock item master data from Tally.
func (c *Client) GetStockItems(ctx context.Context) ([]TallyStockItem, error) {
	reqXML := BuildMasterExportRequest("StockItem")
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseStockItemResponse(resp)
}

// GetGodowns exports all godown (warehouse) master data from Tally.
func (c *Client) GetGodowns(ctx context.Context) ([]TallyGodown, error) {
	reqXML := BuildMasterExportRequest("Godown")
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseGodownResponse(resp)
}

// GetUnits exports all unit of measurement master data from Tally.
func (c *Client) GetUnits(ctx context.Context) ([]TallyUnit, error) {
	reqXML := BuildMasterExportRequest("Unit")
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseUnitResponse(resp)
}

// GetCostCentres exports all cost centre master data from Tally.
func (c *Client) GetCostCentres(ctx context.Context) ([]TallyCostCentre, error) {
	reqXML := BuildMasterExportRequest("CostCentre")
	resp, err := c.SendRequest(ctx, reqXML)
	if err != nil {
		return nil, err
	}
	return ParseCostCentreResponse(resp)
}

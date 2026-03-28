package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mudsahni/satvos-tally-connector/internal/cloud"
	"github.com/mudsahni/satvos-tally-connector/internal/config"
	"github.com/mudsahni/satvos-tally-connector/internal/convert"
	"github.com/mudsahni/satvos-tally-connector/internal/store"
	"github.com/mudsahni/satvos-tally-connector/internal/tally"
)

// Engine is the main sync loop orchestrator.
// It periodically sends heartbeats, pushes Tally master data to the SATVOS cloud,
// and pulls outbound vouchers from the cloud to import into Tally.
type Engine struct {
	cfg         *config.Config
	cloudClient *cloud.Client
	tallyClient *tally.Client
	store       *store.LocalStore
	version     string
	stopCh      chan struct{}
	stopOnce    sync.Once
}

// NewEngine creates a new sync engine.
func NewEngine(cfg *config.Config, cloudClient *cloud.Client, tallyClient *tally.Client, localStore *store.LocalStore, version string) *Engine {
	return &Engine{
		cfg:         cfg,
		cloudClient: cloudClient,
		tallyClient: tallyClient,
		store:       localStore,
		version:     version,
		stopCh:      make(chan struct{}),
	}
}

// Start runs the sync loop. It executes one cycle immediately, then repeats
// on each tick until the context is canceled or Stop is called.
func (e *Engine) Start(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration(e.cfg.Sync.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	e.runCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-e.stopCh:
			return nil
		case <-ticker.C:
			e.runCycle(ctx)
		}
	}
}

// Stop signals the engine to shut down. It is safe to call multiple times.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() { close(e.stopCh) })
}

// TriggerSync runs a single sync cycle immediately (used by the UI).
func (e *Engine) TriggerSync(ctx context.Context) {
	e.runCycle(ctx)
}

// Status holds the current sync status for the UI.
type Status struct {
	TallyConnected bool   `json:"tally_connected"`
	TallyReachable bool   `json:"tally_reachable"`
	TallyCompany   string `json:"tally_company"`
	TallyPort      int    `json:"tally_port"`
	TallyError     string `json:"tally_error,omitempty"`
	AgentID        string `json:"agent_id"`
	LastSyncError  string `json:"last_sync_error,omitempty"`
}

// GetStatus returns the current sync status for the UI.
func (e *Engine) GetStatus() Status {
	state := e.store.Get()
	reachable, company, errMsg := e.tallyClient.CheckStatus(context.Background())
	if company != "" {
		// Update cached company name.
		_ = e.store.Update(func(s *store.State) { s.TallyCompany = company })
	} else {
		company = state.TallyCompany
	}
	return Status{
		TallyConnected: reachable && company != "",
		TallyReachable: reachable,
		TallyCompany:   company,
		TallyPort:      state.TallyPort,
		TallyError:     errMsg,
		AgentID:        state.AgentID,
	}
}

func (e *Engine) runCycle(ctx context.Context) {
	log.Println("[sync] starting sync cycle")

	tallyAvailable := e.tallyClient.IsAvailable(ctx)

	// 1. Heartbeat
	state := e.store.Get()
	if err := e.cloudClient.Heartbeat(ctx, cloud.HeartbeatRequest{
		TallyConnected: tallyAvailable,
		TallyCompany:   state.TallyCompany,
		TallyPort:      state.TallyPort,
		Version:        e.version,
	}); err != nil {
		log.Printf("[sync] heartbeat failed: %v", err)
	}

	if !tallyAvailable {
		log.Println("[sync] tally not available, skipping sync")
		return
	}

	// Update company info
	if info, err := e.tallyClient.GetCompanyInfo(ctx); err == nil {
		_ = e.store.Update(func(s *store.State) {
			s.TallyCompany = info.Name
		})
	}

	// 2. Push masters (Tally -> SATVOS)
	e.pushMasters(ctx)

	// 3. Pull outbound (SATVOS -> Tally)
	e.processOutbound(ctx)

	log.Println("[sync] sync cycle complete")
}

func (e *Engine) pushMasters(ctx context.Context) {
	payload := cloud.MasterPayload{}

	if ledgers, err := e.tallyClient.GetLedgers(ctx); err == nil {
		for _, l := range ledgers {
			payload.Ledgers = append(payload.Ledgers, cloud.MasterLedger{
				Name: l.Name, ParentGroup: l.Parent, GSTIN: l.GSTNumber,
				State: l.LedgerState, TaxType: l.TaxType, TaxRate: l.RateOfTax,
				IsRevenue: l.IsRevenue == "Yes",
			})
		}
	} else {
		log.Printf("[sync] failed to get ledgers: %v", err)
	}

	if items, err := e.tallyClient.GetStockItems(ctx); err == nil {
		for _, i := range items {
			payload.StockItems = append(payload.StockItems, cloud.MasterStockItem{
				Name: i.Name, ParentGroup: i.Parent, HSNCode: i.HSNCode, DefaultUOM: i.BaseUnit,
			})
		}
	} else {
		log.Printf("[sync] failed to get stock items: %v", err)
	}

	if godowns, err := e.tallyClient.GetGodowns(ctx); err == nil {
		for _, g := range godowns {
			payload.Godowns = append(payload.Godowns, cloud.MasterGodown{Name: g.Name, Parent: g.Parent})
		}
	} else {
		log.Printf("[sync] failed to get godowns: %v", err)
	}

	if units, err := e.tallyClient.GetUnits(ctx); err == nil {
		for _, u := range units {
			payload.Units = append(payload.Units, cloud.MasterUnit{Symbol: u.Symbol, FormalName: u.FormalName})
		}
	} else {
		log.Printf("[sync] failed to get units: %v", err)
	}

	if centers, err := e.tallyClient.GetCostCentres(ctx); err == nil { //nolint:misspell // Tally uses British spelling
		for _, c := range centers {
			payload.CostCentres = append(payload.CostCentres, cloud.MasterCostCentre{Name: c.Name, Parent: c.Parent})
		}
	} else {
		log.Printf("[sync] failed to get cost centers: %v", err)
	}

	if err := e.cloudClient.PushMasters(ctx, &payload); err != nil {
		log.Printf("[sync] failed to push masters: %v", err)
	}
}

func (e *Engine) processOutbound(ctx context.Context) {
	state := e.store.Get()
	companyName := state.TallyCompany

	resp, err := e.cloudClient.PullOutbound(ctx, state.OutboundCursor, e.cfg.Sync.BatchSize)
	if err != nil {
		log.Printf("[sync] failed to pull outbound: %v", err)
		return
	}

	log.Printf("[sync] outbound: received %d items", len(resp.Items))
	if len(resp.Items) == 0 {
		return
	}

	// Collect all unique ledgers referenced across all vouchers so we can
	// ensure they exist in Tally before importing any vouchers.
	ledgerSet := make(map[string]tally.LedgerDef)
	var parsedItems []struct {
		item cloud.OutboundItem
		vdef convert.VoucherDef
	}

	var ackResults []cloud.AckResult

	for _, item := range resp.Items {
		var vdef convert.VoucherDef
		if err := json.Unmarshal(item.VoucherDef, &vdef); err != nil {
			log.Printf("[sync] failed to parse voucher def for doc %s: %v", item.DocumentID, err)
			ackResults = append(ackResults, cloud.AckResult{
				SyncEventID: item.SyncEventID, DocumentID: item.DocumentID,
				Success: false, ErrorMessage: fmt.Sprintf("parse error: %v", err),
			})
			continue
		}

		// Track ledgers that need to exist.
		if vdef.PartyLedger != "" {
			ld := tally.LedgerDef{
				Name: vdef.PartyLedger, ParentGroup: "Sundry Creditors",
			}
			if vdef.PartyDetails != nil {
				ld.Address = vdef.PartyDetails.Address
				ld.PAN = vdef.PartyDetails.PAN
				ld.GSTIN = vdef.PartyDetails.GSTIN
				ld.State = vdef.PartyDetails.State
			}
			ledgerSet[vdef.PartyLedger] = ld
		}
		if vdef.PurchaseLedger != "" {
			ledgerSet[vdef.PurchaseLedger] = tally.LedgerDef{
				Name: vdef.PurchaseLedger, ParentGroup: "Purchase Accounts",
			}
		}
		for _, t := range vdef.TaxEntries {
			if t.LedgerName != "" {
				ledgerSet[t.LedgerName] = tally.LedgerDef{
					Name: t.LedgerName, ParentGroup: "Duties & Taxes",
				}
			}
		}

		parsedItems = append(parsedItems, struct {
			item cloud.OutboundItem
			vdef convert.VoucherDef
		}{item: item, vdef: vdef})
	}

	// Ensure all referenced ledgers exist in Tally (creates missing ones,
	// skips existing ones via DUPIGNORECOMBINE).
	if len(ledgerSet) > 0 {
		var ledgers []tally.LedgerDef
		for _, l := range ledgerSet {
			ledgers = append(ledgers, l)
		}
		if err := e.tallyClient.EnsureLedgersExist(ctx, companyName, ledgers); err != nil {
			log.Printf("[sync] warning: failed to pre-create ledgers: %v", err)
			// Continue anyway — some ledgers may already exist, and individual
			// voucher imports will report specific errors.
		}
	}

	// Now import each voucher.
	for i := range parsedItems {
		item := parsedItems[i].item
		vdef := parsedItems[i].vdef

		log.Printf("[sync] outbound item: doc=%s sync_event=%s", item.DocumentID, item.SyncEventID)

		xmlStr, err := convert.ToXML(&vdef)
		if err != nil {
			log.Printf("[sync] failed to convert to XML for doc %s: %v", item.DocumentID, err)
			ackResults = append(ackResults, cloud.AckResult{
				SyncEventID: item.SyncEventID, DocumentID: item.DocumentID,
				Success: false, ErrorMessage: fmt.Sprintf("conversion error: %v", err),
			})
			continue
		}

		result, err := e.tallyClient.ImportVoucher(ctx, xmlStr, companyName)
		if err != nil {
			log.Printf("[sync] failed to import voucher for doc %s: %v", item.DocumentID, err)
			ackResults = append(ackResults, cloud.AckResult{
				SyncEventID: item.SyncEventID, DocumentID: item.DocumentID,
				Success: false, ErrorMessage: fmt.Sprintf("import error: %v", err),
			})
			continue
		}

		if !result.Success {
			errMsg := "unknown import error"
			if len(result.Errors) > 0 {
				errMsg = result.Errors[0]
			}
			log.Printf("[sync] voucher import failed for doc %s: %s", item.DocumentID, errMsg)
			ackResults = append(ackResults, cloud.AckResult{
				SyncEventID: item.SyncEventID, DocumentID: item.DocumentID,
				Success: false, ErrorMessage: errMsg,
			})
			continue
		}

		log.Printf("[sync] voucher imported for doc %s (created=%d, vchid=%s)",
			item.DocumentID, result.Created, result.LastVchID)
		ackResults = append(ackResults, cloud.AckResult{
			SyncEventID:    item.SyncEventID,
			DocumentID:     item.DocumentID,
			Success:        true,
			TallyVoucherID: result.LastVchID,
		})
	}

	// Update cursor
	if resp.NextCursor != "" {
		_ = e.store.Update(func(s *store.State) {
			s.OutboundCursor = resp.NextCursor
		})
	}

	// Send ACKs
	if len(ackResults) > 0 {
		if err := e.cloudClient.Ack(ctx, cloud.AckRequest{Results: ackResults}); err != nil {
			log.Printf("[sync] failed to send ACKs: %v", err)
		}
	}
}

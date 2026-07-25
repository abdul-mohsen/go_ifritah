package config

// Plan identifiers are shared by feature definitions and plan-gating callers.
const (
	PlanSolo       = "solo"
	PlanGrowth     = "growth"
	PlanBusiness   = "business"
	PlanEnterprise = "enterprise"
)

// Feature identifiers are the stable keys used by plan-gating and plugins.
const (
	FeatureZATCAInvoicing       = "zatca_invoicing"
	FeatureZATCAMonitor         = "zatca_monitor"
	FeatureDashboardAdvanced    = "dashboard_advanced"
	FeatureDashboardBasic       = "dashboard_basic"
	FeatureCashVouchersApproval = "cash_vouchers_approval"
	FeatureSupplierLedger       = "supplier_ledger"
	FeatureMarginEngine         = "margin_engine"
	FeatureAutomotiveModule     = "automotive_module"
	FeaturePOSMode              = "pos_mode"
	FeatureStockTransfer        = "stock_transfer"
	FeatureUserManagement       = "user_management"
	FeatureDebitNotes           = "debit_notes"
	FeatureVATReturnReport      = "vat_return_report"
	FeatureZakatWorksheet       = "zakat_worksheet"
	FeatureChartOfAccounts      = "chart_of_accounts"
	FeatureJournalEntries       = "journal_entries"
	FeatureAVCOValuation        = "avco_valuation"
	FeaturePOWorkflow           = "po_workflow"
	FeatureBankReconciliation   = "bank_reconciliation"
)

// Feature describes a capability and the minimum plan required to use it.
type Feature struct {
	ID       string
	MinPlan  string
	AlwaysOn bool
	Replaces string
	PluginID string
}

// Plan contains the display and ordering metadata for a subscription plan.
type Plan struct {
	Level    int
	PriceSAR int
	LabelEN  string
	LabelAR  string
}

// FeatureCatalog is the single source of truth for plan-gated capabilities.
var FeatureCatalog = []Feature{
	{ID: FeatureZATCAInvoicing, MinPlan: PlanSolo, AlwaysOn: true},
	{ID: FeatureZATCAMonitor, MinPlan: PlanBusiness},
	{ID: FeatureDashboardAdvanced, MinPlan: PlanGrowth, Replaces: FeatureDashboardBasic},
	{ID: FeatureCashVouchersApproval, MinPlan: PlanGrowth},
	{ID: FeatureSupplierLedger, MinPlan: PlanGrowth},
	{ID: FeatureMarginEngine, MinPlan: PlanGrowth, PluginID: FeatureMarginEngine},
	{ID: FeatureAutomotiveModule, MinPlan: PlanGrowth, PluginID: "automotive"},
	{ID: FeaturePOSMode, MinPlan: PlanGrowth},
	{ID: FeatureStockTransfer, MinPlan: PlanGrowth},
	{ID: FeatureUserManagement, MinPlan: PlanBusiness},
	{ID: FeatureDebitNotes, MinPlan: PlanBusiness},
	{ID: FeatureVATReturnReport, MinPlan: PlanBusiness},
	{ID: FeatureZakatWorksheet, MinPlan: PlanBusiness},
	{ID: FeatureChartOfAccounts, MinPlan: PlanEnterprise},
	{ID: FeatureJournalEntries, MinPlan: PlanEnterprise},
	{ID: FeatureAVCOValuation, MinPlan: PlanEnterprise},
	{ID: FeaturePOWorkflow, MinPlan: PlanEnterprise},
	{ID: FeatureBankReconciliation, MinPlan: PlanEnterprise},
}

// PlanCatalog contains the plan levels, prices, and localized labels.
var PlanCatalog = map[string]Plan{
	PlanSolo:       {Level: 1, PriceSAR: 149, LabelEN: "Solo", LabelAR: "فردي"},
	PlanGrowth:     {Level: 2, PriceSAR: 349, LabelEN: "Growth", LabelAR: "نمو"},
	PlanBusiness:   {Level: 3, PriceSAR: 699, LabelEN: "Business", LabelAR: "أعمال"},
	PlanEnterprise: {Level: 4, PriceSAR: 1499, LabelEN: "Enterprise", LabelAR: "مؤسسات"},
}

// GetFeature returns the catalog entry for id, or nil when it is unknown.
func GetFeature(id string) *Feature {
	for i := range FeatureCatalog {
		if FeatureCatalog[i].ID == id {
			return &FeatureCatalog[i]
		}
	}
	return nil
}

// PlanLevel returns the ordering level for plan, or zero when it is unknown.
func PlanLevel(plan string) int {
	if p, ok := PlanCatalog[plan]; ok {
		return p.Level
	}
	return 0
}

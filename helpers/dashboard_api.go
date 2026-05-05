package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"afrita/config"
)

// ─── Wire structs match the JSON shape returned by the new ifritah-go
// /api/v2/dashboard, /api/v2/dashboard/analytics and
// /api/v2/dashboard/compare endpoints. Keep field tags exact. ────────

// DashboardAPIResponse — GET /api/v2/dashboard
type DashboardAPIResponse struct {
	Stats              DashboardAPIStats        `json:"stats"`
	StatusCounts       map[string]int64         `json:"status_counts"`
	Charts             DashboardAPICharts       `json:"charts"`
	RecentInvoices     []DashboardAPIInvoice    `json:"recent_invoices"`
	LowStockProducts   []DashboardAPILowStock   `json:"low_stock_products"`
	TopProducts        []DashboardAPITopProduct `json:"top_products"`
	MarginTiers        []DashboardAPITier       `json:"margin_tiers"`
	SupplierPerf       []DashboardAPISupplier   `json:"supplier_perf"`
	TopClients         []DashboardAPITopClient  `json:"top_clients"`
	TopCLV             []DashboardAPICLVClient  `json:"top_clv"`
	ClientDistribution DashboardAPIClientDist   `json:"client_distribution"`
	Filters            DashboardAPIFilters      `json:"filters"`
}

type DashboardAPIStats struct {
	TotalInvoices       int64  `json:"total_invoices"`
	TotalRevenue        string `json:"total_revenue"`
	TotalVAT            string `json:"total_vat"`
	TotalDiscount       string `json:"total_discount"`
	TotalProducts       int64  `json:"total_products"`
	TotalClients        int64  `json:"total_clients"`
	TotalSuppliers      int64  `json:"total_suppliers"`
	TotalStores         int64  `json:"total_stores"`
	TotalBranches       int64  `json:"total_branches"`
	PendingInvoices     int64  `json:"pending_invoices"`
	PendingAmount       string `json:"pending_amount"`
	TotalPurchases      string `json:"total_purchases"`
	TotalPurchaseBills  int64  `json:"total_purchase_bills"`
	TotalPurchaseVAT    string `json:"total_purchase_vat"`
	GrossProfit         string `json:"gross_profit"`
	GrossMargin         string `json:"gross_margin"`
	AvgInvoiceValue     string `json:"avg_invoice_value"`
	LowStockCount       int64  `json:"low_stock_count"`
	CreditNoteCount     int64  `json:"credit_note_count"`
	CreditNoteTotal     string `json:"credit_note_total"`
	InvTurnover         string `json:"inv_turnover"`
	FulfillmentRate     string `json:"fulfillment_rate"`
	ReturnRate          string `json:"return_rate"`
	TotalOrders         int64  `json:"total_orders"`
	PendingOrders       int64  `json:"pending_orders"`
	CompletedOrders     int64  `json:"completed_orders"`
	CancelledOrders     int64  `json:"cancelled_orders"`
	TotalOrdersAmount   string `json:"total_orders_amount"`
	AvgProcessingDays   string `json:"avg_processing_days"`
	ClientConcentration string `json:"client_concentration"`
}

type DashboardAPICharts struct {
	MonthLabels      []string                     `json:"month_labels"`
	MonthlyRevenue   []string                     `json:"monthly_revenue"`
	MonthlyPurchases []string                     `json:"monthly_purchases"`
	MonthlyProfit    []string                     `json:"monthly_profit"`
	YoYRevenue       []string                     `json:"yoy_revenue"`
	WeekdayRevenue   []DashboardAPIWeekdayRevenue `json:"weekday_revenue"`
	MonthlyReturns   []DashboardAPIMonthlyReturn  `json:"monthly_returns"`
}

type DashboardAPIInvoice struct {
	ID              int    `json:"id"`
	SequenceNumber  int64  `json:"sequence_number"`
	Total           string `json:"total"`
	State           int    `json:"state"`
	StateLabel      string `json:"state_label"`
	Date            string `json:"date"`
	IsCreditNote    bool   `json:"is_credit_note"`
	UserPhoneNumber any    `json:"user_phone_number"`
}

type DashboardAPILowStock struct {
	ID            int    `json:"id"`
	ArticleNumber string `json:"article_number"`
	Price         string `json:"price"`
	Quantity      string `json:"quantity"`
	CostPrice     string `json:"cost_price"`
	MinStock      int    `json:"min_stock"`
	StoreID       int    `json:"store_id"`
}

type DashboardAPITopProduct struct {
	ID            int    `json:"id"`
	ArticleNumber string `json:"article_number"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
}

type DashboardAPITier struct {
	Label    string `json:"label"`
	Count    int    `json:"count"`
	AvgPrice string `json:"avg_price"`
}

type DashboardAPISupplier struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	BillCount  int    `json:"bill_count"`
	TotalSpent string `json:"total_spent"`
	AvgTotal   string `json:"avg_total"`
}

type DashboardAPITopClient struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	InvoiceCount int    `json:"invoice_count"`
	Total        string `json:"total"`
	LastInvoice  string `json:"last_invoice"`
}

type DashboardAPICLVClient struct {
	ClientID   int    `json:"client_id"`
	Name       string `json:"name"`
	OrderCount int    `json:"order_count"`
	Value      string `json:"value"`
}

type DashboardAPIWeekdayRevenue struct {
	Day        int    `json:"day"`
	DayName    string `json:"day_name"`
	AvgRevenue string `json:"avg_revenue"`
}

type DashboardAPIMonthlyReturn struct {
	Month       string `json:"month"`
	Invoices    int    `json:"invoices"`
	CreditNotes int    `json:"credit_notes"`
	Rate        string `json:"rate"`
}

type DashboardAPIClientDist struct {
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
}

type DashboardAPIFilters struct {
	State     string `json:"state"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Months    int    `json:"months"`
}

// DashboardAnalyticsAPIResponse — GET /api/v2/dashboard/analytics
type DashboardAnalyticsAPIResponse struct {
	ARAging         []DashboardAPIAgingBucket   `json:"ar_aging"`
	APAging         []DashboardAPIAgingBucket   `json:"ap_aging"`
	CashFlow        []DashboardAPICashFlowMonth `json:"cash_flow"`
	PnL             DashboardAPIPnL             `json:"pnl"`
	KPITrends       DashboardAPIKPITrends       `json:"kpi_trends"`
	VATQuarterly    []DashboardAPIVATQuarter    `json:"vat_quarterly"`
	BalanceSheet    DashboardAPIBalanceSheet    `json:"balance_sheet"`
	OpEx            DashboardAPIOpEx            `json:"opex"`
	ZATCA           DashboardAPIZATCAStats      `json:"zatca"`
	PaymentTracking DashboardAPIPaymentTracking `json:"payment_tracking"`
	Liquidity       DashboardAPILiquidity       `json:"liquidity"`
}

type DashboardAPIAgingBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Total string `json:"total"`
}

type DashboardAPICashFlowMonth struct {
	Month   string `json:"month"`
	Inflow  string `json:"inflow"`
	Outflow string `json:"outflow"`
	Net     string `json:"net"`
}

type DashboardAPIPnL struct {
	Revenue      string   `json:"revenue"`
	COGS         string   `json:"cogs"`
	GrossProfit  string   `json:"gross_profit"`
	GrossMargin  string   `json:"gross_margin"`
	MonthLabels  []string `json:"month_labels"`
	MonthRevenue []string `json:"month_revenue"`
	MonthCOGS    []string `json:"month_cogs"`
	MonthProfit  []string `json:"month_profit"`
}

type DashboardAPIKPITrends struct {
	Invoices       DashboardAPITrend `json:"invoices"`
	Revenue        DashboardAPITrend `json:"revenue"`
	PurchasesTotal DashboardAPITrend `json:"purchases_total"`
	GrossProfit    DashboardAPITrend `json:"gross_profit"`
}

type DashboardAPITrend struct {
	Direction string `json:"direction"`
	Percent   string `json:"percent"`
	Arrow     string `json:"arrow"`
}

type DashboardAPIVATQuarter struct {
	Quarter   string `json:"quarter"`
	OutputVAT string `json:"output_vat"`
	InputVAT  string `json:"input_vat"`
	NetVAT    string `json:"net_vat"`
}

type DashboardAPIBalanceSheet struct {
	AsOf             string                   `json:"as_of"`
	TotalAssets      string                   `json:"total_assets"`
	TotalLiabilities string                   `json:"total_liabilities"`
	TotalEquity      string                   `json:"total_equity"`
	NetWorth         string                   `json:"net_worth"`
	Assets           []DashboardAPIAccountGrp `json:"assets"`
	Liabilities      []DashboardAPIAccountGrp `json:"liabilities"`
	Equity           []DashboardAPIAccountGrp `json:"equity"`
}

type DashboardAPIAccountGrp struct {
	Subtype string `json:"subtype"`
	Amount  string `json:"amount"`
}

type DashboardAPIOpEx struct {
	StartDate    string                        `json:"start_date"`
	EndDate      string                        `json:"end_date"`
	TotalOpEx    string                        `json:"total_opex"`
	OpExVAT      string                        `json:"opex_vat"`
	ExpenseCount int64                         `json:"expense_count"`
	NetIncome    string                        `json:"net_income"`
	OpExRatio    string                        `json:"opex_ratio"`
	ByCategory   []DashboardAPIExpenseCategory `json:"by_category"`
}

type DashboardAPIExpenseCategory struct {
	CategoryID   int    `json:"category_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	TotalAmount  string `json:"total_amount"`
	ExpenseCount int64  `json:"expense_count"`
}

type DashboardAPIZATCAStats struct {
	TotalSubmissions    int64  `json:"total_submissions"`
	PendingCount        int64  `json:"pending_count"`
	SubmittedCount      int64  `json:"submitted_count"`
	AcceptedCount       int64  `json:"accepted_count"`
	RejectedCount       int64  `json:"rejected_count"`
	WarningCount        int64  `json:"warning_count"`
	AcceptanceRate      string `json:"acceptance_rate"`
	AvgRetries          string `json:"avg_retries"`
	AvgClearanceSeconds string `json:"avg_clearance_seconds"`
}

type DashboardAPIPaymentTracking struct {
	AROutstandingCount int64  `json:"ar_outstanding_count"`
	AROutstandingTotal string `json:"ar_outstanding_total"`
	APOutstandingCount int64  `json:"ap_outstanding_count"`
	APOutstandingTotal string `json:"ap_outstanding_total"`
	PaymentsReceived   string `json:"payments_received"`
	PaymentsMade       string `json:"payments_made"`
	NetCashPosition    string `json:"net_cash_position"`
}

type DashboardAPILiquidity struct {
	CurrentAssets      string `json:"current_assets"`
	CurrentLiabilities string `json:"current_liabilities"`
	Inventory          string `json:"inventory"`
	CurrentRatio       string `json:"current_ratio"`
	QuickRatio         string `json:"quick_ratio"`
	DebtToEquity       string `json:"debt_to_equity"`
}

// DashboardCompareAPIResponse — GET /api/v2/dashboard/compare
type DashboardCompareAPIResponse struct {
	PeriodA DashboardAPIComparePeriod `json:"period_a"`
	PeriodB DashboardAPIComparePeriod `json:"period_b"`
}

type DashboardAPIComparePeriod struct {
	Invoices   int64  `json:"invoices"`
	Revenue    string `json:"revenue"`
	Purchases  string `json:"purchases"`
	Profit     string `json:"profit"`
	AvgInvoice string `json:"avg_invoice"`
	Pending    string `json:"pending"`
	Margin     string `json:"margin"`
	Issued     int64  `json:"issued"`
	Draft      int64  `json:"draft"`
}

// ─── fetcher ────────────────────────────────────────────────────────

func dashboardGet(token, path string, query url.Values, out any) error {
	target := config.BackendDomain + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	log.Printf("🔵 [API REQUEST] GET %s", target)

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("🔴 [API ERROR] GET %s → %d", target, resp.StatusCode)
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("backend status 401")
		}
		return fmt.Errorf("backend status %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

// FetchDashboardSummary calls GET /api/v2/dashboard with optional filters.
func FetchDashboardSummary(token, stateFilter, startDate, endDate string, months int) (*DashboardAPIResponse, error) {
	q := url.Values{}
	if stateFilter != "" {
		q.Set("state", stateFilter)
	}
	if startDate != "" {
		q.Set("start_date", startDate)
	}
	if endDate != "" {
		q.Set("end_date", endDate)
	}
	if months > 0 {
		q.Set("months", fmt.Sprintf("%d", months))
	}
	var out DashboardAPIResponse
	if err := dashboardGet(token, "/api/v2/dashboard", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FetchDashboardAnalytics calls GET /api/v2/dashboard/analytics.
func FetchDashboardAnalytics(token, startDate, endDate string, months int) (*DashboardAnalyticsAPIResponse, error) {
	q := url.Values{}
	if startDate != "" {
		q.Set("start_date", startDate)
	}
	if endDate != "" {
		q.Set("end_date", endDate)
	}
	if months > 0 {
		q.Set("months", fmt.Sprintf("%d", months))
	}
	var out DashboardAnalyticsAPIResponse
	if err := dashboardGet(token, "/api/v2/dashboard/analytics", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FetchDashboardCompareData calls GET /api/v2/dashboard/compare.
func FetchDashboardCompareData(token, aStart, aEnd, bStart, bEnd string) (*DashboardCompareAPIResponse, error) {
	q := url.Values{}
	q.Set("a_start", aStart)
	q.Set("a_end", aEnd)
	q.Set("b_start", bStart)
	q.Set("b_end", bEnd)
	var out DashboardCompareAPIResponse
	if err := dashboardGet(token, "/api/v2/dashboard/compare", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

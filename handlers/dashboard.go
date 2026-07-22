package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"afrita/resources"
)

// statPlaceholder is used when a stat cannot be loaded due to an API error.
const statPlaceholder = "\u2014"

var dashboardAgingKeys = []string{
	"aging.0_30_current",
	"aging.31_60_overdue",
	"aging.61_90_very_overdue",
	"aging.90_plus_critical",
}

func dashboardInvoiceStatus(lang string, state int) string {
	switch state {
	case 0:
		return resources.T(lang, "invoice_status.draft")
	case 1:
		return resources.T(lang, "status.processing")
	case 2:
		return resources.T(lang, "invoice_status.processed")
	case 3:
		return resources.T(lang, "invoice_status.issued")
	default:
		return ""
	}
}

func dashboardAgingLabel(lang string, idx int, fallback string) string {
	if idx >= 0 && idx < len(dashboardAgingKeys) {
		return resources.T(lang, dashboardAgingKeys[idx])
	}
	return fallback
}

// HandleDashboardTest renders the dashboard with mock data for testing.
func HandleDashboardTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	stats := map[string]interface{}{
		"invoices":             "156",
		"products":             "342",
		"clients":              "89",
		"revenue":              "2350000.00",
		"suppliers":            "42",
		"pending_orders":       "12",
		"purchases_total":      "1200000.00",
		"gross_profit":         "1150000.00",
		"low_stock_count":      "7",
		"revenue_before_vat":   "2043478.26",
		"purchases_before_vat": "1043478.26",
	}

	recentInvoices := []map[string]interface{}{
		{
			"id":              1,
			"sequence_number": "INV-001",
			"total":           "15,500",
			"date":            "2026-02-13",
			"status":          "صادرة",
			"status_class":    "badge-issued",
		},
		{
			"id":              2,
			"sequence_number": "INV-002",
			"total":           "8,750",
			"date":            "2026-02-12",
			"status":          "تمت المعالجة",
			"status_class":    "badge-issued",
		},
	}

	labelsJSON := `["يناير", "فبراير", "مارس", "أبريل", "مايو", "يونيو"]`
	revenueJSON := `[150000, 180000, 165000, 195000, 210000, 245000]`
	purchasesJSON := `[80000, 95000, 88000, 102000, 110000, 125000]`
	statusJSON := `[12, 5, 28, 45]`

	statusCountsTemplate := map[string]int{
		"draft":      12,
		"processing": 5,
		"processed":  28,
		"issued":     45,
	}

	data := map[string]interface{}{
		"title": "لوحة التحكم",
		"stats": stats,
		"kpi_trends": map[string]interface{}{
			"invoices":        map[string]interface{}{"Arrow": "↑", "Percent": "12.5%", "Class": "text-green-500"},
			"revenue":         map[string]interface{}{"Arrow": "↑", "Percent": "8.2%", "Class": "text-green-500"},
			"purchases_total": map[string]interface{}{"Arrow": "↓", "Percent": "3.1%", "Class": "text-red-500"},
			"gross_profit":    map[string]interface{}{"Arrow": "↑", "Percent": "15.0%", "Class": "text-green-500"},
		},
		"chart_labels":    template.JS(labelsJSON),
		"chart_revenue":   template.JS(revenueJSON),
		"chart_purchases": template.JS(purchasesJSON),
		"chart_status":    template.JS(statusJSON),
		"recent_invoices": recentInvoices,
		"status_counts":   statusCountsTemplate,
		"low_stock_products": []map[string]interface{}{
			{"id": 101, "price": "45.00", "quantity": "2"},
			{"id": 102, "price": "120.00", "quantity": "0"},
			{"id": 103, "price": "78.50", "quantity": "3"},
		},
		"state_filter": "",
		"start_date":   "",
		"end_date":     "",
		"user_role":    "admin",
		"version":      config.AppVersion,

		// Analytics — AR / AP Aging
		"ar_aging": []map[string]interface{}{
			{"Label": "0-30", "Class": "text-green-500", "Count": 12, "Total": 25000.00},
			{"Label": "31-60", "Class": "text-yellow-500", "Count": 8, "Total": 18000.00},
			{"Label": "61-90", "Class": "text-orange-500", "Count": 5, "Total": 8000.00},
			{"Label": "90+", "Class": "text-red-500", "Count": 3, "Total": 5000.00},
		},
		"ap_aging": []map[string]interface{}{
			{"Label": "0-30", "Class": "text-green-500", "Count": 10, "Total": 20000.00},
			{"Label": "31-60", "Class": "text-yellow-500", "Count": 6, "Total": 15000.00},
			{"Label": "61-90", "Class": "text-orange-500", "Count": 3, "Total": 6000.00},
			{"Label": "90+", "Class": "text-red-500", "Count": 2, "Total": 4000.00},
		},

		// Analytics — Cash Flow chart data
		"cf_inflow":  template.JS(`[150000, 180000, 165000, 195000, 210000, 245000]`),
		"cf_outflow": template.JS(`[80000, 95000, 88000, 102000, 110000, 125000]`),
		"cf_net":     template.JS(`[70000, 85000, 77000, 93000, 100000, 120000]`),

		// Analytics — P&L
		"pnl": map[string]interface{}{
			"Revenue":     2350000.00,
			"COGS":        1200000.00,
			"GrossProfit": 1150000.00,
			"GrossMargin": 48.9,
		},
		"pnl_revenue": template.JS(`[200000, 220000, 240000, 250000, 260000, 280000]`),
		"pnl_cogs":    template.JS(`[100000, 110000, 120000, 130000, 140000, 150000]`),
		"pnl_profit":  template.JS(`[100000, 110000, 120000, 120000, 120000, 130000]`),

		// Analytics — P1 KPIs
		"inv_turnover":     "2.50",
		"fulfillment_rate": "78.5",
		"return_rate": map[string]interface{}{
			"ReturnRate":    4.2,
			"CreditNotes":   8,
			"TotalInvoices": 190,
		},
		"avg_processing_time": "3.2",

		// Analytics — CLV table
		"top_clv": []map[string]interface{}{
			{"name": "شركة الفارس", "value": "125000.00"},
			{"name": "مؤسسة النجم", "value": "98000.00"},
			{"name": "شركة الأمل", "value": "75000.00"},
		},

		// Analytics — Margin tiers
		"margin_tiers": []map[string]interface{}{
			{"Label": "< 50 ر.س", "Class": "bg-green-500", "Count": 45, "AvgPrice": 32.0},
			{"Label": "50-200 ر.س", "Class": "bg-blue-500", "Count": 120, "AvgPrice": 125.0},
			{"Label": "200-500 ر.س", "Class": "bg-yellow-500", "Count": 95, "AvgPrice": 340.0},
			{"Label": "500+ ر.س", "Class": "bg-red-500", "Count": 82, "AvgPrice": 750.0},
		},

		// Analytics — Supplier performance
		"supplier_perf": []map[string]interface{}{
			{"Name": "شركة التوريدات", "BillCount": 24, "AvgTotal": 15000.0, "TotalSpent": 360000.0},
			{"Name": "مؤسسة المعدات", "BillCount": 18, "AvgTotal": 22000.0, "TotalSpent": 396000.0},
		},

		// Analytics — P1 charts
		"yoy_revenue":     template.JS(`[1800000, 2100000, 2350000]`),
		"moving_avg":      template.JS(`[180000, 185000, 190000, 192000, 198000, 210000]`),
		"weekday_revenue": template.JS(`[45000, 52000, 48000, 55000, 50000, 38000, 12000]`),
		"return_monthly":  template.JS(`[2, 1, 3, 2, 1, 2]`),

		// Analytics — additional chart data
		"top_prod_labels":    template.JS(`["قطعة A", "قطعة B", "قطعة C"]`),
		"top_prod_values":    template.JS(`[150, 120, 95]`),
		"client_dist_labels": template.JS(`["أفراد", "شركات", "حكومي"]`),
		"client_dist_values": template.JS(`[45, 35, 20]`),

		// Advanced metrics (test mock values)
		"avg_invoice_value":        "15064.10",
		"revenue_per_client":       "26404.49",
		"mom_growth":               template.JS(`[0, 20, -8.3, 18.2, 7.7, 16.7]`),
		"purchase_to_sales_ratio":  "51.1",
		"discount_rate":            "2.50",
		"effective_vat_rate":       "14.85",
		"draft_to_issued_rate":     "13.3",
		"rev_by_status":            template.JS(`[120000, 80000, 350000, 1800000]`),
		"inventory_value":          "456000.00",
		"out_of_stock_count":       5,
		"out_of_stock_rate":        "1.5",
		"inventory_to_sales_ratio": "1.9",
		"dio":                      "146.0",
		"ar_total":                 "56000.00",
		"ar_count":                 28,
		"ap_total":                 "45000.00",
		"ap_count":                 21,
		"net_cash_position":        "11000.00",
		"recv_turnover":            "41.96",
		"avg_collection_period":    "8.7",
		"ap_turnover_val":          "26.67",
		"avg_payment_period":       "13.7",
		"dso":                      "8.7",
		"dpo":                      "13.7",
		"ccc":                      "141.0",
		"cash_burn_rate":           "0.00",
		"client_concentration":     "65.3",
		"vat_quarterly": []map[string]interface{}{
			{"Quarter": "Q1/2026", "OutputVAT": 52500.0, "InputVAT": 27000.0, "NetVAT": 25500.0},
			{"Quarter": "Q4/2025", "OutputVAT": 48000.0, "InputVAT": 25000.0, "NetVAT": 23000.0},
		},
		"mock_balance_sheet": helpers.DashboardBalanceSheet{
			CurrentAssets:      1057500,
			CurrentLiabilities: 420000,
			TotalAssets:        1880000,
			TotalDebt:          300000,
			Equity:             1580000,
			Cash:               352500,
		},
		"mock_opex": helpers.DashboardOperatingExpenses{
			Rent:      117500,
			Salaries:  352500,
			Utilities: 47000,
			Marketing: 70500,
			Other:     117500,
			TotalOpEx: 705000,
			NetIncome: 445000,
			OpExRatio: 30.0,
		},
		"mock_zatca": helpers.DashboardZATCACompliance{
			UUIDCount:          156,
			QRCodeCount:        156,
			SubmittedCount:     143,
			RejectedCount:      3,
			ComplianceRate:     91.7,
			LastSubmissionDate: "2026-02-01",
		},
		"mock_payment": helpers.DashboardPaymentTrackingPanel{
			CollectionEfficiency: 78.5,
			OnTimeRate:           72.3,
			OverdueCount:         23,
			OverdueAmount:        282000,
			AvgDaysToPay:         23.5,
		},
		"mock_liquidity": helpers.DashboardLiquidityRatios{
			CurrentRatio: 2.52,
			QuickRatio:   1.43,
			DebtToEquity: 0.19,
		},
		"net_income":     "450000.00",
		"opex_ratio":     "29.8",
		"current_ratio":  "2.52",
		"quick_ratio":    "1.71",
		"debt_to_equity": "0.22",
	}

	if config.DashboardTestTemplate == nil {
		config.LoadTemplates()
	}
	if config.DashboardTestTemplate == nil {
		http.Error(w, "Template error: dashboard test template not available", http.StatusInternalServerError)
		return
	}
	tmpl, data := helpers.BindLangData(config.DashboardTestTemplate, r, data)
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// HandleDashboard renders the live dashboard backed by the consolidated
// /api/v2/dashboard and /api/v2/dashboard/analytics backend endpoints.
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	stateFilter := r.URL.Query().Get("state")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	var (
		summary                  *helpers.DashboardAPIResponse
		analytics                *helpers.DashboardAnalyticsAPIResponse
		summaryErr, analyticsErr error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		summary, summaryErr = helpers.FetchDashboardSummary(token, stateFilter, startDate, endDate, 6)
	}()
	go func() {
		defer wg.Done()
		analytics, analyticsErr = helpers.FetchDashboardAnalytics(token, startDate, endDate, 6)
	}()
	wg.Wait()

	// Check auth errors
	for _, err := range []error{summaryErr, analyticsErr} {
		if err != nil && helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
	}

	// Collect warning sections for partial API failures
	var dashboardWarnings []string
	if summaryErr != nil {
		log.Printf("dashboard summary error: %v", summaryErr)
		dashboardWarnings = append(dashboardWarnings, resources.T(lang, "dashboard.title"))
	}
	if analyticsErr != nil {
		log.Printf("dashboard analytics error: %v", analyticsErr)
		dashboardWarnings = append(dashboardWarnings, resources.T(lang, "dashboard.analytics"))
	}

	// Defensive: if both failed, render an empty shell so the page still loads.
	if summary == nil {
		summary = &helpers.DashboardAPIResponse{}
	}
	if analytics == nil {
		analytics = &helpers.DashboardAnalyticsAPIResponse{}
	}

	// ── Helper parsers (backend returns numerics as formatted strings) ─
	parseF := func(s string) float64 {
		if s == "" {
			return 0
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return v
	}
	parseFSlice := func(in []string) []float64 {
		out := make([]float64, len(in))
		for i, s := range in {
			out[i] = parseF(s)
		}
		return out
	}
	jsJSON := func(v interface{}) template.JS {
		b, _ := json.Marshal(v)
		return template.JS(b)
	}

	// ── Stats map ──────────────────────────────────────────────────
	s := summary.Stats
	totalRevenue := parseF(s.TotalRevenue)
	totalPurchases := parseF(s.TotalPurchases)
	outputVAT := parseF(s.TotalVAT)
	inputVAT := parseF(s.TotalPurchaseVAT)

	stats := map[string]string{
		"invoices":             fmt.Sprintf("%d", s.TotalInvoices),
		"products":             fmt.Sprintf("%d", s.TotalProducts),
		"clients":              fmt.Sprintf("%d", s.TotalClients),
		"suppliers":            fmt.Sprintf("%d", s.TotalSuppliers),
		"revenue":              s.TotalRevenue,
		"purchases_total":      s.TotalPurchases,
		"gross_profit":         s.GrossProfit,
		"low_stock_count":      fmt.Sprintf("%d", s.LowStockCount),
		"pending_orders":       fmt.Sprintf("%d", s.PendingOrders),
		"total_vat":            s.TotalVAT,
		"purchase_vat":         s.TotalPurchaseVAT,
		"net_vat_payable":      fmt.Sprintf("%.2f", outputVAT-inputVAT),
		"credit_note_count":    fmt.Sprintf("%d", s.CreditNoteCount),
		"credit_note_total":    s.CreditNoteTotal,
		"total_discount":       s.TotalDiscount,
		"revenue_before_vat":   fmt.Sprintf("%.2f", totalRevenue-outputVAT),
		"purchases_before_vat": fmt.Sprintf("%.2f", totalPurchases-inputVAT),
	}
	if summaryErr != nil {
		for _, k := range []string{
			"invoices", "products", "clients", "suppliers", "revenue",
			"purchases_total", "gross_profit", "low_stock_count", "pending_orders",
			"total_vat", "purchase_vat", "net_vat_payable",
			"credit_note_count", "credit_note_total", "total_discount",
			"revenue_before_vat", "purchases_before_vat",
		} {
			stats[k] = statPlaceholder
		}
	}

	// ── status_counts (template expects keys: draft/processing/processed/issued)
	stateBy := map[int]int{0: 0, 1: 0, 2: 0, 3: 0}
	for k, v := range summary.StatusCounts {
		// Backend may key by numeric string ("0","1","2","3") or label.
		switch k {
		case "0", "draft":
			stateBy[0] += int(v)
		case "1", "pending", "processing":
			stateBy[1] += int(v)
		case "2", "processed":
			stateBy[2] += int(v)
		case "3", "issued":
			stateBy[3] += int(v)
		}
	}
	statusCountsTemplate := map[string]int{
		"draft":      stateBy[0],
		"processing": stateBy[1],
		"processed":  stateBy[2],
		"issued":     stateBy[3],
	}

	// ── Recent invoices ────────────────────────────────────────────
	statusClassFor := func(state int) string {
		switch state {
		case 0:
			return "badge-gray"
		case 1:
			return "badge-yellow"
		case 2:
			return "badge-green"
		case 3:
			return "badge-blue"
		default:
			return "badge-gray"
		}
	}
	recentInvoices := make([]map[string]interface{}, 0, len(summary.RecentInvoices))
	for _, inv := range summary.RecentInvoices {
		status := dashboardInvoiceStatus(lang, inv.State)
		if status == "" {
			status = inv.StateLabel
		}
		recentInvoices = append(recentInvoices, map[string]interface{}{
			"id":              inv.ID,
			"sequence_number": inv.SequenceNumber,
			"total":           inv.Total,
			"date":            helpers.ToDisplayDate(inv.Date),
			"status":          status,
			"status_class":    statusClassFor(inv.State),
		})
	}

	// ── Low-stock products ─────────────────────────────────────────
	lowStockProducts := make([]map[string]interface{}, 0, len(summary.LowStockProducts))
	for _, p := range summary.LowStockProducts {
		lowStockProducts = append(lowStockProducts, map[string]interface{}{
			"id":       p.ID,
			"price":    p.Price,
			"quantity": p.Quantity,
		})
	}

	// ── Charts ────────────────────────────────────────────────────
	c := summary.Charts
	monthLabels := c.MonthLabels
	revenueSeries := parseFSlice(c.MonthlyRevenue)
	purchaseSeries := parseFSlice(c.MonthlyPurchases)
	chartLabelsJSON := jsJSON(monthLabels)
	chartRevenueJSON := jsJSON(revenueSeries)
	chartPurchasesJSON := jsJSON(purchaseSeries)
	chartStatusJSON := jsJSON([]int{stateBy[0], stateBy[1], stateBy[2], stateBy[3]})

	// ── KPI trends ────────────────────────────────────────────────
	cls := func(direction string) string {
		switch direction {
		case "up":
			return "text-green-500"
		case "down":
			return "text-red-500"
		default:
			return "text-gray-400"
		}
	}
	tr := func(t helpers.DashboardAPITrend) map[string]interface{} {
		return map[string]interface{}{"Arrow": t.Arrow, "Percent": t.Percent, "Class": cls(t.Direction)}
	}
	kpiTrends := map[string]interface{}{
		"invoices":        tr(analytics.KPITrends.Invoices),
		"revenue":         tr(analytics.KPITrends.Revenue),
		"purchases_total": tr(analytics.KPITrends.PurchasesTotal),
		"gross_profit":    tr(analytics.KPITrends.GrossProfit),
	}

	// ── AR / AP aging ─────────────────────────────────────────────
	mapBuckets := func(in []helpers.DashboardAPIAgingBucket) []map[string]interface{} {
		out := make([]map[string]interface{}, 0, len(in))
		for i, b := range in {
			out = append(out, map[string]interface{}{
				"Label": dashboardAgingLabel(lang, i, b.Label),
				"Count": b.Count,
				"Total": parseF(b.Total),
			})
		}
		return out
	}
	arAging := mapBuckets(analytics.ARAging)
	apAging := mapBuckets(analytics.APAging)

	// ── Cash flow ─────────────────────────────────────────────────
	cfInflow := make([]float64, 0, len(analytics.CashFlow))
	cfOutflow := make([]float64, 0, len(analytics.CashFlow))
	cfNet := make([]float64, 0, len(analytics.CashFlow))
	for _, m := range analytics.CashFlow {
		cfInflow = append(cfInflow, parseF(m.Inflow))
		cfOutflow = append(cfOutflow, parseF(m.Outflow))
		cfNet = append(cfNet, parseF(m.Net))
	}

	// ── P&L ───────────────────────────────────────────────────────
	pnl := analytics.PnL
	pnlMap := map[string]interface{}{
		"Revenue":     parseF(pnl.Revenue),
		"COGS":        parseF(pnl.COGS),
		"GrossProfit": parseF(pnl.GrossProfit),
		"GrossMargin": parseF(pnl.GrossMargin),
	}

	// ── Margin tiers / supplier perf / top clients reshape ───────
	marginTiers := make([]map[string]interface{}, 0, len(summary.MarginTiers))
	for _, t := range summary.MarginTiers {
		marginTiers = append(marginTiers, map[string]interface{}{
			"Label":    t.Label,
			"Count":    t.Count,
			"AvgPrice": parseF(t.AvgPrice),
		})
	}
	supplierPerf := make([]map[string]interface{}, 0, len(summary.SupplierPerf))
	for _, sp := range summary.SupplierPerf {
		supplierPerf = append(supplierPerf, map[string]interface{}{
			"Name":       sp.Name,
			"BillCount":  sp.BillCount,
			"TotalSpent": parseF(sp.TotalSpent),
			"AvgTotal":   parseF(sp.AvgTotal),
		})
	}
	topCLV := make([]map[string]interface{}, 0, len(summary.TopCLV))
	for _, cl := range summary.TopCLV {
		topCLV = append(topCLV, map[string]interface{}{
			"name":  cl.Name,
			"value": cl.Value,
		})
	}

	// ── Weekday revenue / monthly returns / yoy / moving avg ────
	weekdayRevenue := make([]map[string]interface{}, 0, len(c.WeekdayRevenue))
	for _, w := range c.WeekdayRevenue {
		weekdayRevenue = append(weekdayRevenue, map[string]interface{}{
			"DayName":    w.DayName,
			"AvgRevenue": parseF(w.AvgRevenue),
		})
	}
	returnMonthlyRates := make([]float64, 0, len(c.MonthlyReturns))
	for _, m := range c.MonthlyReturns {
		returnMonthlyRates = append(returnMonthlyRates, parseF(m.Rate))
	}
	yoyRevenue := parseFSlice(c.YoYRevenue)

	// 3-month moving average over revenue series
	movingAvg := make([]float64, len(revenueSeries))
	for i := range revenueSeries {
		start := i - 2
		if start < 0 {
			start = 0
		}
		var sum float64
		for j := start; j <= i; j++ {
			sum += revenueSeries[j]
		}
		movingAvg[i] = sum / float64(i-start+1)
	}

	// MoM growth (% vs previous month)
	momGrowth := make([]float64, len(revenueSeries))
	for i := 1; i < len(revenueSeries); i++ {
		if revenueSeries[i-1] > 0 {
			momGrowth[i] = (revenueSeries[i] - revenueSeries[i-1]) * 100 / revenueSeries[i-1]
		}
	}

	// ── Top products / client distribution ───────────────────────
	topProdLabels := make([]string, 0, len(summary.TopProducts))
	topProdValues := make([]float64, 0, len(summary.TopProducts))
	for _, p := range summary.TopProducts {
		topProdLabels = append(topProdLabels, p.ArticleNumber)
		topProdValues = append(topProdValues, parseF(p.Quantity))
	}
	clientDistLabels := []string{
		resources.T(lang, "dashboard.active_clients"),
		resources.T(lang, "dashboard.inactive_clients"),
	}
	clientDistValues := []int64{summary.ClientDistribution.Active, summary.ClientDistribution.Inactive}

	// ── Build base data map ──────────────────────────────────────
	data := map[string]interface{}{
		"title":              resources.T(lang, "dashboard.title"),
		"dashboard_warnings": dashboardWarnings,
		"stats":              stats,
		"kpi_trends":         kpiTrends,
		"chart_labels":       chartLabelsJSON,
		"chart_revenue":      chartRevenueJSON,
		"chart_purchases":    chartPurchasesJSON,
		"chart_status":       chartStatusJSON,
		"recent_invoices":    recentInvoices,
		"status_counts":      statusCountsTemplate,
		"low_stock_products": lowStockProducts,
		"state_filter":       stateFilter,
		"start_date":         startDate,
		"end_date":           endDate,
		"user_role":          helpers.GetUserRole(r),
		"version":            config.AppVersion,

		// Aging
		"ar_aging": arAging,
		"ap_aging": apAging,

		// Cash flow
		"cf_inflow":  jsJSON(cfInflow),
		"cf_outflow": jsJSON(cfOutflow),
		"cf_net":     jsJSON(cfNet),

		// P&L
		"pnl":         pnlMap,
		"pnl_revenue": jsJSON(parseFSlice(pnl.MonthRevenue)),
		"pnl_cogs":    jsJSON(parseFSlice(pnl.MonthCOGS)),
		"pnl_profit":  jsJSON(parseFSlice(pnl.MonthProfit)),

		// KPI scalars
		"inv_turnover":     s.InvTurnover,
		"fulfillment_rate": s.FulfillmentRate,
		"return_rate": map[string]interface{}{
			"ReturnRate":    parseF(s.ReturnRate),
			"CreditNotes":   s.CreditNoteCount,
			"TotalInvoices": s.TotalInvoices,
		},
		"avg_processing_time": s.AvgProcessingDays,

		// Tables
		"top_clv":       topCLV,
		"margin_tiers":  marginTiers,
		"supplier_perf": supplierPerf,

		// Charts
		"yoy_revenue":     jsJSON(yoyRevenue),
		"moving_avg":      jsJSON(movingAvg),
		"weekday_revenue": jsJSON(weekdayRevenue),
		"return_monthly":  jsJSON(returnMonthlyRates),

		// Distributions
		"top_prod_labels":    jsJSON(topProdLabels),
		"top_prod_values":    jsJSON(topProdValues),
		"client_dist_labels": jsJSON(clientDistLabels),
		"client_dist_values": jsJSON(clientDistValues),
	}

	// ── Advanced metrics derived from totals ─────────────────────
	avgInvoiceValue := parseF(s.AvgInvoiceValue)
	revenuePerClient := 0.0
	if s.TotalClients > 0 {
		revenuePerClient = totalRevenue / float64(s.TotalClients)
	}
	purchaseToSalesRatio := 0.0
	if totalRevenue > 0 {
		purchaseToSalesRatio = totalPurchases * 100 / totalRevenue
	}
	totalDiscount := parseF(s.TotalDiscount)
	discountRate := 0.0
	if totalRevenue > 0 {
		discountRate = totalDiscount * 100 / totalRevenue
	}
	effectiveVATRate := 0.0
	if totalRevenue > 0 {
		effectiveVATRate = outputVAT * 100 / totalRevenue
	}
	draftToIssuedRate := 0.0
	if stateBy[3] > 0 {
		draftToIssuedRate = float64(stateBy[0]) * 100 / float64(stateBy[3])
	}
	revByStatusJSON := jsJSON([]float64{0, 0, 0, totalRevenue}) // backend not yet exposing per-status revenue split

	// Inventory
	inventoryValue := 0.0
	for _, t := range marginTiers {
		if cnt, ok := t["Count"].(int); ok {
			if avg, ok2 := t["AvgPrice"].(float64); ok2 {
				inventoryValue += float64(cnt) * avg
			}
		}
	}
	turnoverRatio := parseF(s.InvTurnover)
	dio := 0.0
	if turnoverRatio > 0 {
		dio = 365 / turnoverRatio
	}
	monthlyAvgRev := totalRevenue / 12
	inventoryToSalesRatio := 0.0
	if monthlyAvgRev > 0 {
		inventoryToSalesRatio = inventoryValue * 100 / monthlyAvgRev
	}
	outOfStockCount := 0
	for _, p := range summary.LowStockProducts {
		if parseF(p.Quantity) <= 0 {
			outOfStockCount++
		}
	}
	outOfStockRate := 0.0
	if s.TotalProducts > 0 {
		outOfStockRate = float64(outOfStockCount) * 100 / float64(s.TotalProducts)
	}

	// AR/AP totals from analytics (sum of buckets)
	arTotal, arCount := 0.0, 0
	for _, b := range analytics.ARAging {
		arTotal += parseF(b.Total)
		arCount += b.Count
	}
	apTotal, apCount := 0.0, 0
	for _, b := range analytics.APAging {
		apTotal += parseF(b.Total)
		apCount += b.Count
	}

	pt := analytics.PaymentTracking
	netCashPosition := parseF(pt.NetCashPosition)
	if netCashPosition == 0 {
		netCashPosition = arTotal - apTotal
	}
	recvTurnover := 0.0
	if arTotal > 0 {
		recvTurnover = totalRevenue / arTotal
	}
	avgCollectionPeriod := 0.0
	if recvTurnover > 0 {
		avgCollectionPeriod = 365 / recvTurnover
	}
	apTurnoverVal := 0.0
	if apTotal > 0 {
		apTurnoverVal = totalPurchases / apTotal
	}
	avgPaymentPeriod := 0.0
	if apTurnoverVal > 0 {
		avgPaymentPeriod = 365 / apTurnoverVal
	}
	dso := 0.0
	if totalRevenue > 0 {
		dso = arTotal * 365 / totalRevenue
	}
	dpo := 0.0
	if totalPurchases > 0 {
		dpo = apTotal * 365 / totalPurchases
	}
	cashConversionCycle := dio + dso - dpo

	cashBurnRate := 0.0
	if len(cfNet) > 0 {
		var sumNet float64
		for _, n := range cfNet {
			sumNet += n
		}
		cashBurnRate = sumNet / float64(len(cfNet))
	}

	// VAT quarterly
	vatQuarterly := make([]map[string]interface{}, 0, len(analytics.VATQuarterly))
	for _, q := range analytics.VATQuarterly {
		vatQuarterly = append(vatQuarterly, map[string]interface{}{
			"Quarter":   q.Quarter,
			"OutputVAT": parseF(q.OutputVAT),
			"InputVAT":  parseF(q.InputVAT),
			"NetVAT":    parseF(q.NetVAT),
		})
	}

	// ── Panel structs populated from analytics endpoint ─────────
	mockBS := helpers.DashboardBalanceSheet{
		TotalAssets: parseF(analytics.BalanceSheet.TotalAssets),
		TotalDebt:   parseF(analytics.BalanceSheet.TotalLiabilities),
		Equity:      parseF(analytics.BalanceSheet.TotalEquity),
	}

	mockOpEx := helpers.DashboardOperatingExpenses{
		TotalOpEx: parseF(analytics.OpEx.TotalOpEx),
		NetIncome: parseF(analytics.OpEx.NetIncome),
		OpExRatio: parseF(analytics.OpEx.OpExRatio),
	}
	for _, cat := range analytics.OpEx.ByCategory {
		amt := parseF(cat.TotalAmount)
		switch cat.Code {
		case "RENT":
			mockOpEx.Rent += amt
		case "SALARIES", "PAYROLL":
			mockOpEx.Salaries += amt
		case "UTILITIES":
			mockOpEx.Utilities += amt
		case "MARKETING":
			mockOpEx.Marketing += amt
		default:
			mockOpEx.Other += amt
		}
	}

	mockZATCA := helpers.DashboardZATCACompliance{
		SubmittedCount: int(analytics.ZATCA.SubmittedCount + analytics.ZATCA.AcceptedCount),
		RejectedCount:  int(analytics.ZATCA.RejectedCount),
		ComplianceRate: parseF(analytics.ZATCA.AcceptanceRate),
	}

	mockPayment := helpers.DashboardPaymentTrackingPanel{
		OverdueCount:  int(pt.AROutstandingCount),
		OverdueAmount: parseF(pt.AROutstandingTotal),
	}

	mockLiquidity := helpers.DashboardLiquidityRatios{
		CurrentRatio: parseF(analytics.Liquidity.CurrentRatio),
		QuickRatio:   parseF(analytics.Liquidity.QuickRatio),
		DebtToEquity: parseF(analytics.Liquidity.DebtToEquity),
	}

	clientConcentration := parseF(s.ClientConcentration)

	// ── Add advanced/derived keys ────────────────────────────────
	data["avg_invoice_value"] = fmt.Sprintf("%.2f", avgInvoiceValue)
	data["revenue_per_client"] = fmt.Sprintf("%.2f", revenuePerClient)
	data["mom_growth"] = jsJSON(momGrowth)
	data["purchase_to_sales_ratio"] = fmt.Sprintf("%.1f", purchaseToSalesRatio)
	data["discount_rate"] = fmt.Sprintf("%.2f", discountRate)
	data["effective_vat_rate"] = fmt.Sprintf("%.2f", effectiveVATRate)
	data["draft_to_issued_rate"] = fmt.Sprintf("%.1f", draftToIssuedRate)
	data["rev_by_status"] = revByStatusJSON

	data["inventory_value"] = fmt.Sprintf("%.2f", inventoryValue)
	data["out_of_stock_count"] = outOfStockCount
	data["out_of_stock_rate"] = fmt.Sprintf("%.1f", outOfStockRate)
	data["inventory_to_sales_ratio"] = fmt.Sprintf("%.1f", inventoryToSalesRatio)
	data["dio"] = fmt.Sprintf("%.1f", dio)

	data["ar_total"] = fmt.Sprintf("%.2f", arTotal)
	data["ar_count"] = arCount
	data["ap_total"] = fmt.Sprintf("%.2f", apTotal)
	data["ap_count"] = apCount
	data["net_cash_position"] = fmt.Sprintf("%.2f", netCashPosition)
	data["recv_turnover"] = fmt.Sprintf("%.2f", recvTurnover)
	data["avg_collection_period"] = fmt.Sprintf("%.1f", avgCollectionPeriod)
	data["ap_turnover_val"] = fmt.Sprintf("%.2f", apTurnoverVal)
	data["avg_payment_period"] = fmt.Sprintf("%.1f", avgPaymentPeriod)
	data["dso"] = fmt.Sprintf("%.1f", dso)
	data["dpo"] = fmt.Sprintf("%.1f", dpo)
	data["ccc"] = fmt.Sprintf("%.1f", cashConversionCycle)
	data["cash_burn_rate"] = fmt.Sprintf("%.2f", cashBurnRate)
	data["client_concentration"] = fmt.Sprintf("%.1f", clientConcentration)
	data["vat_quarterly"] = vatQuarterly

	data["mock_balance_sheet"] = mockBS
	data["mock_opex"] = mockOpEx
	data["mock_zatca"] = mockZATCA
	data["mock_payment"] = mockPayment
	data["mock_liquidity"] = mockLiquidity
	data["net_income"] = fmt.Sprintf("%.2f", mockOpEx.NetIncome)
	data["opex_ratio"] = fmt.Sprintf("%.1f", mockOpEx.OpExRatio)
	data["current_ratio"] = fmt.Sprintf("%.2f", mockLiquidity.CurrentRatio)
	data["quick_ratio"] = fmt.Sprintf("%.2f", mockLiquidity.QuickRatio)
	data["debt_to_equity"] = fmt.Sprintf("%.2f", mockLiquidity.DebtToEquity)

	if config.DashboardTemplate == nil {
		config.LoadTemplates()
	}
	if config.DashboardTemplate == nil {
		http.Error(w, "Template error: dashboard template not available", http.StatusInternalServerError)
		return
	}
	tmpl, data := helpers.BindLangData(config.DashboardTemplate, r, data)
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// HandleDashboardCompare returns an HTMX partial comparing two periods.
func HandleDashboardCompare(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)
	alignClass := "text-right"
	if lang == resources.LangEn {
		alignClass = "text-left"
	}

	aStart := html.EscapeString(r.URL.Query().Get("a_start"))
	aEnd := html.EscapeString(r.URL.Query().Get("a_end"))
	bStart := html.EscapeString(r.URL.Query().Get("b_start"))
	bEnd := html.EscapeString(r.URL.Query().Get("b_end"))

	if aStart == "" || aEnd == "" || bStart == "" || bEnd == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="text-center text-gray-400 text-sm py-4">%s</div>`, resources.T(lang, "compare.select_periods"))
		return
	}

	resp, err := helpers.FetchDashboardCompareData(token, aStart, aEnd, bStart, bEnd)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		log.Printf("dashboard compare error: %v", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="text-center text-red-500 text-sm py-4">%s</div>`, resources.T(lang, "compare.load_error"))
		return
	}

	a := resp.PeriodA
	b := resp.PeriodB

	type row struct {
		Label string
		A     string
		B     string
	}
	rows := []row{
		{resources.T(lang, "compare.invoice_count"), fmt.Sprintf("%d", a.Invoices), fmt.Sprintf("%d", b.Invoices)},
		{resources.T(lang, "compare.revenue"), a.Revenue, b.Revenue},
		{resources.T(lang, "compare.purchases"), a.Purchases, b.Purchases},
		{resources.T(lang, "compare.profit"), a.Profit, b.Profit},
		{resources.T(lang, "compare.avg_invoice"), a.AvgInvoice, b.AvgInvoice},
		{resources.T(lang, "compare.pending"), a.Pending, b.Pending},
		{resources.T(lang, "compare.margin_pct"), a.Margin + "%", b.Margin + "%"},
		{resources.T(lang, "compare.issued_invoices"), fmt.Sprintf("%d", a.Issued), fmt.Sprintf("%d", b.Issued)},
		{resources.T(lang, "compare.drafts"), fmt.Sprintf("%d", a.Draft), fmt.Sprintf("%d", b.Draft)},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<table class="w-full text-sm border-collapse"><thead><tr class="bg-gray-50 dark:bg-gray-700">`)
	fmt.Fprintf(w, `<th class="p-2 %s font-semibold text-gray-600 dark:text-gray-300 border-b dark:border-gray-600">%s</th>`, alignClass, resources.T(lang, "compare.indicator"))
	fmt.Fprintf(w, `<th class="p-2 text-center font-semibold text-blue-600 border-b dark:border-gray-600">%s<br><span class="text-xs font-normal text-gray-400">%s → %s</span></th>`, resources.T(lang, "compare.period_a"), aStart, aEnd)
	fmt.Fprintf(w, `<th class="p-2 text-center font-semibold text-purple-600 border-b dark:border-gray-600">%s<br><span class="text-xs font-normal text-gray-400">%s → %s</span></th>`, resources.T(lang, "compare.period_b"), bStart, bEnd)
	fmt.Fprint(w, `</tr></thead><tbody>`)
	for i, rw := range rows {
		bg := ""
		if i%2 == 0 {
			bg = ` class="bg-gray-50/50 dark:bg-gray-800/30"`
		}
		fmt.Fprintf(w, `<tr%s><td class="p-2 %s text-gray-700 dark:text-gray-300 border-b dark:border-gray-700">%s</td>`, bg, alignClass, rw.Label)
		fmt.Fprintf(w, `<td class="p-2 text-center font-mono border-b dark:border-gray-700">%s</td>`, rw.A)
		fmt.Fprintf(w, `<td class="p-2 text-center font-mono border-b dark:border-gray-700">%s</td></tr>`, rw.B)
	}
	fmt.Fprint(w, `</tbody></table>`)
}

// HandleDashboardExportPDF renders a print-ready HTML page of dashboard KPIs.
func HandleDashboardExportPDF(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)
	dir := "rtl"
	textAlign := "right"
	printButtonSide := "left"
	if lang == resources.LangEn {
		dir = "ltr"
		textAlign = "left"
		printButtonSide = "right"
	}

	// ── Fetch all data in parallel (same as HandleDashboard) ───
	var (
		invoices      []models.Invoice
		products      []models.Product
		clients       []models.Client
		suppliers     []models.Supplier
		orders        []map[string]interface{}
		purchaseBills []models.Invoice
	)
	var (
		invErr, prodErr, clientErr, suppErr, orderErr, pbErr error
	)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() { defer wg.Done(); invoices, invErr = helpers.FetchAllInvoicesUnpaginated(token) }()
	go func() { defer wg.Done(); products, prodErr = helpers.FetchProducts(token) }()
	go func() { defer wg.Done(); clients, clientErr = helpers.FetchClients(token) }()
	go func() { defer wg.Done(); suppliers, suppErr = helpers.FetchSuppliers(token) }()
	go func() { defer wg.Done(); orders, orderErr = helpers.FetchOrders(token) }()
	go func() {
		defer wg.Done()
		purchaseBills, pbErr = helpers.FetchPurchaseBillsAll(token, 1, "", "")
	}()
	wg.Wait()

	for _, err := range []error{invErr, prodErr, clientErr, suppErr, orderErr, pbErr} {
		if err != nil && helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
	}

	// ── Core stats ─────────────────────────────────────────────
	var totalRevenue, totalPurchases float64
	for _, inv := range invoices {
		totalRevenue += inv.Total
	}
	for _, pb := range purchaseBills {
		totalPurchases += pb.Total
	}
	grossProfit := totalRevenue - totalPurchases
	avgInvoice := 0.0
	if len(invoices) > 0 {
		avgInvoice = totalRevenue / float64(len(invoices))
	}
	var pendingAmount float64
	for _, inv := range invoices {
		if inv.State == 0 || inv.State == 1 {
			pendingAmount += inv.Total
		}
	}
	statusCounts := map[int]int{0: 0, 1: 0, 2: 0, 3: 0}
	for _, inv := range invoices {
		statusCounts[inv.State]++
	}

	lowStockCount := 0
	if prodErr == nil {
		for _, p := range products {
			if helpers.ParseIntValue(p.Quantity) <= 5 {
				lowStockCount++
			}
		}
	}

	pendingOrders := 0
	if orderErr == nil {
		pendingOrders = helpers.ComputePendingOrders(orders)
	}

	turnoverRatio, _, _ := helpers.ComputeInventoryTurnover(products, totalPurchases)
	agingBuckets := helpers.ComputeAgingAnalysis(invoices)
	apAgingBuckets := helpers.ComputeAPAging(purchaseBills)
	clvMap := helpers.ComputeCLV(orders)
	topClients := []map[string]interface{}{}
	if orderErr == nil {
		topClients = helpers.ComputeTopClients(orders, 10)
		for i, tc := range topClients {
			name, _ := tc["name"].(string)
			if v, ok := clvMap[name]; ok && v > 0 {
				topClients[i]["clv"] = fmt.Sprintf("%.2f", v)
			} else {
				topClients[i]["clv"] = statPlaceholder
			}
		}
	}

	profitMargin := 0.0
	if totalRevenue > 0 {
		profitMargin = grossProfit * 100 / totalRevenue
	}

	// ── VAT computation ────────────────────────────────────────
	var outputVAT, inputVAT, totalDiscount float64
	for _, inv := range invoices {
		outputVAT += inv.TotalVAT
		totalDiscount += inv.Discount
	}
	for _, pb := range purchaseBills {
		inputVAT += pb.TotalVAT
	}
	netVATPayable := outputVAT - inputVAT

	// ── Advanced metrics ───────────────────────────────────────
	avgInvoiceValue := helpers.ComputeAverageInvoiceValue(invoices)
	revenuePerClient := helpers.ComputeRevenuePerClient(totalRevenue, len(clients))
	purchaseToSalesRatio := helpers.ComputePurchaseToSalesRatio(totalPurchases, totalRevenue)
	discountRate := helpers.ComputeDiscountRate(totalDiscount, totalRevenue)
	effectiveVATRate := helpers.ComputeEffectiveVATRate(outputVAT, totalRevenue)
	draftToIssuedRate := helpers.ComputeDraftToIssuedRate(statusCounts)

	inventoryValue := helpers.ComputeInventoryValue(products)
	outOfStockCount, outOfStockRate := helpers.ComputeOutOfStock(products)
	inventoryToSalesRatio := helpers.ComputeInventoryToSalesRatio(inventoryValue, totalRevenue/12)
	dio := helpers.ComputeDIO(turnoverRatio)

	arTotal, arCount := helpers.ComputeTotalAROutstanding(agingBuckets)
	apTotal, apCount := helpers.ComputeTotalAPOutstanding(apAgingBuckets)
	netCashPosition := helpers.ComputeNetCashPosition(arTotal, apTotal)
	recvTurnover := helpers.ComputeReceivablesTurnover(totalRevenue, arTotal)
	avgCollectionPeriod := helpers.ComputeAvgCollectionPeriod(recvTurnover)
	apTurnoverVal := helpers.ComputeAPTurnover(totalPurchases, apTotal)
	avgPaymentPeriod := helpers.ComputeAvgPaymentPeriod(apTurnoverVal)
	dso := helpers.ComputeDSO(arTotal, totalRevenue, 365)
	dpo := helpers.ComputeDPO(apTotal, totalPurchases, 365)
	cashConversionCycle := helpers.ComputeCashConversionCycle(dio, dso, dpo)

	clientValues := make([]float64, 0, len(clvMap))
	for _, v := range clvMap {
		clientValues = append(clientValues, v)
	}
	clientConcentration := helpers.ComputeConcentrationRisk(clientValues, 3)

	vatQuarterly := helpers.ComputeVATQuarterly(invoices, purchaseBills)
	for i := range agingBuckets {
		agingBuckets[i].Label = dashboardAgingLabel(lang, i, agingBuckets[i].Label)
	}
	for i := range apAgingBuckets {
		apAgingBuckets[i].Label = dashboardAgingLabel(lang, i, apAgingBuckets[i].Label)
	}

	// ── Render print-friendly HTML ─────────────────────────────
	nowStr := time.Now().Format("2006-01-02 15:04")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="%s" dir="%s">
<head>
<meta charset="UTF-8">
<title>%s — %s</title>
<style>
  @media print { @page { size: A4 landscape; margin: 10mm; } body { -webkit-print-color-adjust: exact; print-color-adjust: exact; } .no-print { display:none; } }
  * { margin:0; padding:0; box-sizing:border-box; }
  body { font-family: 'Segoe UI', Tahoma, sans-serif; direction: %s; color: #1e293b; padding: 20px; font-size: 12px; background: #fff; }
  h1 { font-size: 20px; margin-bottom: 4px; }
  .sub { color: #64748b; font-size: 11px; margin-bottom: 16px; }
  table { width: 100%%; border-collapse: collapse; margin-bottom: 16px; }
  th, td { border: 1px solid #cbd5e1; padding: 6px 10px; text-align: %s; }
  th { background: #f1f5f9; font-weight: 600; font-size: 11px; }
  td { font-size: 12px; }
  .section { margin-bottom: 18px; }
  .section-title { font-size: 14px; font-weight: 700; margin-bottom: 6px; border-bottom: 2px solid #3b82f6; padding-bottom: 4px; color: #1e40af; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .badge { display:inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
  .bg-green { background: #dcfce7; color: #166534; } .bg-yellow { background: #fef9c3; color: #854d0e; }
  .bg-orange { background: #ffedd5; color: #9a3412; } .bg-red { background: #fee2e2; color: #991b1b; }
  .print-btn { position: fixed; bottom: 20px; %s: 20px; background: #3b82f6; color: #fff; border: none; padding: 10px 24px; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 600; box-shadow: 0 2px 8px rgba(0,0,0,.15); }
  .print-btn:hover { background: #2563eb; }
</style>
</head>
<body>
<h1>📊 %s</h1>
<p class="sub">%s: %s &nbsp;|&nbsp; %s: %s</p>

<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr>
      <th>%s</th><th>%s</th><th>%s</th><th>%s</th>
    </tr>
    <tr>
      <td>%s</td><td><b>%d</b></td>
      <td>%s</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.2f</b></td>
      <td>%s</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.2f</b></td>
      <td>%s</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>%s</td><td>%d</td>
      <td>%s</td><td>%d</td>
    </tr>
    <tr>
      <td>%s</td><td>%d</td>
      <td>%s</td><td>%d</td>
    </tr>
    <tr>
      <td>%s</td><td>%d</td>
      <td>%s</td><td><b>%.1f%%</b></td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.2f</b></td>
      <td>&nbsp;</td><td>&nbsp;</td>
    </tr>
  </table>
</div>

<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>
    <tr><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>
  </table>
</div>
`,
		lang, dir,
		resources.T(lang, "report.dashboard_title"), nowStr,
		dir, textAlign, printButtonSide,
		resources.T(lang, "report.dashboard_title"),
		resources.T(lang, "report.date_version"), nowStr, resources.T(lang, "report.version"), config.AppVersion,
		resources.T(lang, "report.key_indicators"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.total_invoices"), len(invoices),
		resources.T(lang, "report.total_revenue"), totalRevenue,
		resources.T(lang, "report.total_purchases"), totalPurchases,
		resources.T(lang, "report.net_profit"), grossProfit,
		resources.T(lang, "report.avg_invoice"), avgInvoice,
		resources.T(lang, "report.pending_amount"), pendingAmount,
		resources.T(lang, "report.products"), len(products),
		resources.T(lang, "report.clients"), len(clients),
		resources.T(lang, "report.suppliers"), len(suppliers),
		resources.T(lang, "dashboard.kpi.pending_orders"), pendingOrders,
		resources.T(lang, "report.low_stock"), lowStockCount,
		resources.T(lang, "report.profit_margin"), profitMargin,
		resources.T(lang, "report.inventory_turnover"), turnoverRatio,
		resources.T(lang, "report.invoice_status"),
		resources.T(lang, "report.draft"), resources.T(lang, "report.processing"),
		resources.T(lang, "report.processed"), resources.T(lang, "report.issued"),
		statusCounts[0], statusCounts[1], statusCounts[2], statusCounts[3],
	)

	// ── Aging analysis ─────────────────────────────────────────
	fmt.Fprintf(w, `<div class="grid2"><div class="section"><div class="section-title">%s</div><table><tr><th>%s</th><th>%s</th><th>%s</th></tr>`,
		resources.T(lang, "report.aging_analysis"),
		resources.T(lang, "report.category"),
		resources.T(lang, "report.count"),
		resources.T(lang, "report.amount"),
	)
	for _, b := range agingBuckets {
		cls := ""
		switch {
		case b.Count == 0:
			cls = ""
		case b.Total > 50000:
			cls = "bg-red"
		case b.Total > 20000:
			cls = "bg-orange"
		default:
			cls = "bg-green"
		}
		fmt.Fprintf(w, `<tr><td>%s</td><td>%d</td><td><span class="badge %s">%.2f</span></td></tr>`, b.Label, b.Count, cls, b.Total)
	}
	fmt.Fprintf(w, `</table></div>`)

	// ── Top clients ────────────────────────────────────────────
	fmt.Fprintf(w, `<div class="section"><div class="section-title">%s</div><table><tr><th>#</th><th>%s</th><th>%s</th><th>%s</th><th>CLV</th></tr>`,
		resources.T(lang, "report.top_clients"),
		resources.T(lang, "report.client"),
		resources.T(lang, "nav.orders"),
		resources.T(lang, "report.total"),
	)
	for _, tc := range topClients {
		fmt.Fprintf(w, `<tr><td>%v</td><td>%v</td><td>%v</td><td>%v</td><td>%v</td></tr>`,
			tc["order"], tc["name"], tc["count"], tc["total"], tc["clv"])
	}
	fmt.Fprintf(w, `</table></div></div>`)

	// ── VAT Section ────────────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>
    <tr>
      <td>%s</td><td><b>%.2f</b></td>
      <td>%s</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>%s</td><td><b style="color:#1e40af">%.2f</b></td>
      <td>%s</td><td><b>%.2f%%</b></td>
    </tr>
  </table>
</div>`,
		resources.T(lang, "report.vat_title"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.output_vat"), outputVAT,
		resources.T(lang, "report.input_vat"), inputVAT,
		resources.T(lang, "report.net_vat"), netVATPayable,
		resources.T(lang, "dashboard.kpi.effective_vat"), effectiveVATRate,
	)

	// ── VAT Quarterly Table ────────────────────────────────────
	if len(vatQuarterly) > 0 {
		fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>`,
			resources.T(lang, "dashboard.section.vat_quarterly"),
			resources.T(lang, "dashboard.vat.quarter"),
			resources.T(lang, "dashboard.vat.output_vat"),
			resources.T(lang, "dashboard.vat.input_vat"),
			resources.T(lang, "dashboard.vat.net_vat"),
		)
		for _, q := range vatQuarterly {
			fmt.Fprintf(w, `<tr><td>%s</td><td style="color:#166534">%.2f</td><td style="color:#991b1b">%.2f</td><td><b style="color:#1e40af">%.2f</b></td></tr>`,
				q.Quarter, q.OutputVAT, q.InputVAT, q.NetVAT)
		}
		fmt.Fprintf(w, `</table></div>`)
	}

	// ── Advanced Profitability ─────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>
    <tr>
      <td>%s</td><td><b>%.2f</b></td>
      <td>%s</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.1f%%</b></td>
      <td>%s</td><td><b>%.2f%%</b></td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.1f%%</b></td>
      <td>&nbsp;</td><td>&nbsp;</td>
    </tr>
  </table>
</div>`,
		resources.T(lang, "dashboard.section.advanced_profitability"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "dashboard.kpi.avg_invoice"), avgInvoiceValue,
		resources.T(lang, "dashboard.kpi.revenue_per_client"), revenuePerClient,
		resources.T(lang, "dashboard.kpi.purchase_to_sales"), purchaseToSalesRatio,
		resources.T(lang, "dashboard.kpi.discount_rate"), discountRate,
		resources.T(lang, "dashboard.kpi.draft_to_issued"), draftToIssuedRate,
	)

	// ── Receivables & Payables ─────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>
    <tr>
      <td>%s</td><td><b>%.2f</b> (%d %s)</td>
      <td>%s</td><td><b>%.2f</b> (%d %s)</td>
    </tr>
    <tr>
      <td>%s</td><td><b style="color:#1e40af">%.2f</b></td>
      <td>%s</td><td><b>%.1f</b> %s</td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.1f</b> %s</td>
      <td>%s</td><td><b>%.1f</b> %s</td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.2f</b>x</td>
      <td>%s</td><td><b>%.1f</b> %s</td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.2f</b>x</td>
      <td>%s</td><td><b>%.1f</b> %s</td>
    </tr>
  </table>
</div>`,
		resources.T(lang, "dashboard.section.receivables_payables"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "dashboard.kpi.ar_total"), arTotal, arCount, resources.T(lang, "report.invoices"),
		resources.T(lang, "dashboard.kpi.ap_total"), apTotal, apCount, resources.T(lang, "report.invoices"),
		resources.T(lang, "dashboard.kpi.net_cash_position"), netCashPosition,
		resources.T(lang, "dashboard.kpi.dso"), dso, resources.T(lang, "ui.day"),
		resources.T(lang, "dashboard.kpi.dpo"), dpo, resources.T(lang, "ui.day"),
		resources.T(lang, "dashboard.kpi.ccc"), cashConversionCycle, resources.T(lang, "ui.day"),
		resources.T(lang, "dashboard.kpi.recv_turnover"), recvTurnover,
		resources.T(lang, "dashboard.kpi.avg_collection"), avgCollectionPeriod, resources.T(lang, "ui.day"),
		resources.T(lang, "dashboard.kpi.ap_turnover"), apTurnoverVal,
		resources.T(lang, "dashboard.kpi.avg_payment"), avgPaymentPeriod, resources.T(lang, "ui.day"),
	)

	// ── AP Aging ───────────────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table><tr><th>%s</th><th>%s</th><th>%s</th></tr>`,
		resources.T(lang, "dashboard.section.ap_aging"),
		resources.T(lang, "report.category"),
		resources.T(lang, "report.count"),
		resources.T(lang, "report.amount"),
	)
	for _, b := range apAgingBuckets {
		cls := ""
		switch {
		case b.Count == 0:
			cls = ""
		case b.Total > 50000:
			cls = "bg-red"
		case b.Total > 20000:
			cls = "bg-orange"
		default:
			cls = "bg-green"
		}
		fmt.Fprintf(w, `<tr><td>%s</td><td>%d</td><td><span class="badge %s">%.2f</span></td></tr>`, b.Label, b.Count, cls, b.Total)
	}
	fmt.Fprintf(w, `</table></div>`)

	// ── Advanced Inventory ─────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>
    <tr>
      <td>%s</td><td><b>%.2f</b></td>
      <td>%s</td><td><b>%d</b> (%.1f%%)</td>
    </tr>
    <tr>
      <td>%s</td><td><b>%.1f</b>x</td>
      <td>%s</td><td><b>%.1f</b> %s</td>
    </tr>
  </table>
</div>`,
		resources.T(lang, "dashboard.section.advanced_inventory"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "report.indicator"), resources.T(lang, "report.value"),
		resources.T(lang, "dashboard.kpi.inventory_value"), inventoryValue,
		resources.T(lang, "dashboard.kpi.out_of_stock"), outOfStockCount, outOfStockRate,
		resources.T(lang, "dashboard.kpi.inv_to_sales"), inventoryToSalesRatio,
		resources.T(lang, "dashboard.kpi.dio"), dio, resources.T(lang, "ui.day"),
	)

	// ── Concentration Risk ─────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">%s</div>
  <table>
    <tr><th>%s</th><th>%s</th></tr>
    <tr><td>%s</td><td><b>%.1f%%</b></td></tr>
  </table>
</div>`,
		resources.T(lang, "dashboard.kpi.client_concentration"),
		resources.T(lang, "report.indicator"),
		resources.T(lang, "report.value"),
		resources.T(lang, "dashboard.kpi.client_concentration"),
		clientConcentration,
	)

	// ── Footer & print trigger ─────────────────────────────────
	fmt.Fprintf(w, `
<button class="print-btn no-print" onclick="window.print()">🖨️ %s</button>
<script>
  // Auto-open print dialog after page loads
  window.addEventListener('load', function() {
    setTimeout(function() { window.print(); }, 500);
  });
</script>
</body></html>`, resources.T(lang, "report.print_save_pdf"))
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// statPlaceholder is used when a stat cannot be loaded due to an API error.
const statPlaceholder = "\u2014"

// HandleDashboardTest renders the dashboard with mock data for testing.
func HandleDashboardTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	stats := map[string]interface{}{
		"invoices":        "156",
		"products":        "342",
		"clients":         "89",
		"revenue":         "2350000.00",
		"suppliers":       "42",
		"pending_orders":  "12",
		"purchases_total": "1200000.00",
		"gross_profit":    "1150000.00",
		"low_stock_count": "7",
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
		"title":              "لوحة التحكم",
		"stats":              stats,
		"kpi_trends": map[string]interface{}{
			"invoices":        map[string]interface{}{"Arrow": "↑", "Percent": "12.5%", "Class": "text-green-500"},
			"revenue":         map[string]interface{}{"Arrow": "↑", "Percent": "8.2%", "Class": "text-green-500"},
			"purchases_total": map[string]interface{}{"Arrow": "↓", "Percent": "3.1%", "Class": "text-red-500"},
			"gross_profit":    map[string]interface{}{"Arrow": "↑", "Percent": "15.0%", "Class": "text-green-500"},
		},
		"chart_labels":       template.JS(labelsJSON),
		"chart_revenue":      template.JS(revenueJSON),
		"chart_purchases":    template.JS(purchasesJSON),
		"chart_status":       template.JS(statusJSON),
		"recent_invoices":    recentInvoices,
		"status_counts":      statusCountsTemplate,
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
		"inv_turnover":    "2.50",
		"fulfillment_rate": "78.5",
		"return_rate": map[string]interface{}{
			"ReturnRate":    4.2,
			"CreditNotes":  8,
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
		"avg_invoice_value":       "15064.10",
		"revenue_per_client":      "26404.49",
		"mom_growth":              template.JS(`[0, 20, -8.3, 18.2, 7.7, 16.7]`),
		"purchase_to_sales_ratio": "51.1",
		"discount_rate":           "2.50",
		"effective_vat_rate":      "14.85",
		"draft_to_issued_rate":    "13.3",
		"rev_by_status":           template.JS(`[120000, 80000, 350000, 1800000]`),
		"inventory_value":         "456000.00",
		"out_of_stock_count":      5,
		"out_of_stock_rate":       "1.5",
		"inventory_to_sales_ratio": "1.9",
		"dio":                     "146.0",
		"ar_total":                "56000.00",
		"ar_count":                28,
		"ap_total":                "45000.00",
		"ap_count":                21,
		"net_cash_position":       "11000.00",
		"recv_turnover":           "41.96",
		"avg_collection_period":   "8.7",
		"ap_turnover_val":         "26.67",
		"avg_payment_period":      "13.7",
		"dso":                     "8.7",
		"dpo":                     "13.7",
		"ccc":                     "141.0",
		"cash_burn_rate":          "0.00",
		"client_concentration":    "65.3",
		"vat_quarterly": []map[string]interface{}{
			{"Quarter": "Q1/2026", "OutputVAT": 52500.0, "InputVAT": 27000.0, "NetVAT": 25500.0},
			{"Quarter": "Q4/2025", "OutputVAT": 48000.0, "InputVAT": 25000.0, "NetVAT": 23000.0},
		},
		"mock_balance_sheet": helpers.GetMockBalanceSheet(2350000, 1200000),
		"mock_opex":          helpers.GetMockOperatingExpenses(2350000, 1150000),
		"mock_zatca":         helpers.GetMockZATCACompliance(156),
		"mock_payment":       helpers.GetMockPaymentTracking(156, 2350000),
		"mock_liquidity":     helpers.GetMockLiquidityRatios(helpers.GetMockBalanceSheet(2350000, 1200000), 456000),
		"net_income":         "450000.00",
		"opex_ratio":         "29.8",
		"current_ratio":      "2.52",
		"quick_ratio":        "1.71",
		"debt_to_equity":     "0.22",
	}

	if config.DashboardTestTemplate == nil {
		config.LoadTemplates()
	}
	if config.DashboardTestTemplate == nil {
		http.Error(w, "Template error: dashboard test template not available", http.StatusInternalServerError)
		return
	}
	if err := config.DashboardTestTemplate.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// HandleDashboard renders the live dashboard.
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stateFilter := r.URL.Query().Get("state")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	var (
		invoices      []models.Invoice
		products      []models.Product
		clients       []models.Client
		suppliers     []models.Supplier
		orders        []map[string]interface{}
		purchaseBills []models.Invoice
		invErr        error
		prodErr       error
		clientErr     error
		suppErr       error
		orderErr      error
		pbErr         error
	)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() { defer wg.Done(); invoices, invErr = helpers.FetchAllInvoicesUnpaginated(token) }()
	go func() { defer wg.Done(); products, prodErr = helpers.FetchProducts(token) }()
	go func() { defer wg.Done(); clients, clientErr = helpers.FetchClients(token) }()
	go func() { defer wg.Done(); suppliers, suppErr = helpers.FetchSuppliers(token) }()
	go func() { defer wg.Done(); orders, orderErr = helpers.FetchOrders(token) }()
	go func() { defer wg.Done(); purchaseBills, pbErr = helpers.FetchPurchaseBillsAll(token, 1, "") }()
	wg.Wait()

	// Check auth errors
	for _, err := range []error{invErr, prodErr, clientErr, suppErr, orderErr, pbErr} {
		if err != nil && helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
	}

	// Collect warning sections for partial API failures
	var dashboardWarnings []string
	if invErr != nil {
		dashboardWarnings = append(dashboardWarnings, "الفواتير")
	}
	if prodErr != nil {
		dashboardWarnings = append(dashboardWarnings, "المنتجات")
	}
	if clientErr != nil {
		dashboardWarnings = append(dashboardWarnings, "العملاء")
	}
	if suppErr != nil {
		dashboardWarnings = append(dashboardWarnings, "الموردين")
	}
	if orderErr != nil {
		dashboardWarnings = append(dashboardWarnings, "الطلبات")
	}
	if pbErr != nil {
		dashboardWarnings = append(dashboardWarnings, "فواتير الشراء")
	}

	// ── Filter invoices ────────────────────────────────────────────
	filteredInvoices := invoices
	if stateFilter != "" {
		stateValue := helpers.ParseIntValue(stateFilter)
		filtered := make([]models.Invoice, 0)
		for _, inv := range invoices {
			if inv.State == stateValue {
				filtered = append(filtered, inv)
			}
		}
		filteredInvoices = filtered
	}

	if startDate != "" || endDate != "" {
		start := helpers.ParseFilterDate(startDate, false)
		end := helpers.ParseFilterDate(endDate, true)
		if start != nil || end != nil {
			filtered := make([]models.Invoice, 0)
			for _, inv := range filteredInvoices {
				if inv.EffectiveDate.Time == "" {
					continue
				}
				parsed, err := time.Parse(time.RFC3339, inv.EffectiveDate.Time)
				if err != nil {
					parsed, err = time.Parse(time.RFC3339Nano, inv.EffectiveDate.Time)
					if err != nil {
						continue
					}
				}
				if start != nil && parsed.Before(*start) {
					continue
				}
				if end != nil && parsed.After(*end) {
					continue
				}
				filtered = append(filtered, inv)
			}
			filteredInvoices = filtered
		}
	}

	// ── Compute core stats ─────────────────────────────────────────
	var totalRevenue float64
	var totalRevenueBeforeVAT float64
	var totalPurchases float64
	var totalPurchasesBeforeVAT float64
	var outputVAT float64
	var inputVAT float64
	var totalDiscount float64
	var creditNoteCount int
	var creditNoteTotal float64
	for _, inv := range filteredInvoices {
		totalRevenue += inv.Total
		totalRevenueBeforeVAT += inv.TotalBeforeVAT
		outputVAT += inv.TotalVAT
		totalDiscount += inv.Discount
		if inv.CreditState == 1 {
			creditNoteCount++
			creditNoteTotal += inv.Total
		}
	}
	for _, pb := range purchaseBills {
		totalPurchases += pb.Total
		totalPurchasesBeforeVAT += pb.TotalBeforeVAT
		inputVAT += pb.TotalVAT
	}
	grossProfit := totalRevenue - totalPurchases

	statusCounts := map[int]int{0: 0, 1: 0, 2: 0, 3: 0}
	for _, inv := range filteredInvoices {
		statusCounts[inv.State]++
	}

	// ── Monthly revenue (last 6 months) ────────────────────────────
	now := time.Now()
	monthLabels := make([]string, 0, 6)
	revenueByMonth := make(map[string]float64)
	purchasesByMonth := make(map[string]float64)
	for i := 5; i >= 0; i-- {
		m := now.AddDate(0, -i, 0)
		monthLabels = append(monthLabels, m.Format("01/2006"))
	}
	for _, inv := range filteredInvoices {
		if inv.EffectiveDate.Time == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, inv.EffectiveDate.Time)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, inv.EffectiveDate.Time)
			if err != nil {
				continue
			}
		}
		revenueByMonth[parsed.Format("01/2006")] += inv.Total
	}
	for _, pb := range purchaseBills {
		if pb.EffectiveDate.Time == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, pb.EffectiveDate.Time)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, pb.EffectiveDate.Time)
			if err != nil {
				continue
			}
		}
		purchasesByMonth[parsed.Format("01/2006")] += pb.Total
	}

	revenueSeries := make([]float64, 0, 6)
	purchaseSeries := make([]float64, 0, 6)
	for _, label := range monthLabels {
		revenueSeries = append(revenueSeries, revenueByMonth[label])
		purchaseSeries = append(purchaseSeries, purchasesByMonth[label])
	}

	// ── Low-stock products ─────────────────────────────────────────
	lowStockProducts := make([]map[string]interface{}, 0)
	if prodErr == nil {
		for _, p := range products {
			if helpers.ParseIntValue(p.Quantity) <= 5 {
				lowStockProducts = append(lowStockProducts, map[string]interface{}{
					"id":       p.ID,
					"price":    p.Price,
					"quantity": p.Quantity,
				})
			}
		}
		sort.Slice(lowStockProducts, func(i, j int) bool {
			qi := helpers.ParseIntValue(fmt.Sprint(lowStockProducts[i]["quantity"]))
			qj := helpers.ParseIntValue(fmt.Sprint(lowStockProducts[j]["quantity"]))
			return qi < qj
		})
		if len(lowStockProducts) > 10 {
			lowStockProducts = lowStockProducts[:10]
		}
	}

	pendingOrders := 0
	if orderErr == nil {
		pendingOrders = helpers.ComputePendingOrders(orders)
	}

	stats := map[string]string{
		"invoices":          fmt.Sprintf("%d", len(filteredInvoices)),
		"products":          fmt.Sprintf("%d", len(products)),
		"clients":           fmt.Sprintf("%d", len(clients)),
		"revenue":           fmt.Sprintf("%.2f", totalRevenue),
		"suppliers":         fmt.Sprintf("%d", len(suppliers)),
		"pending_orders":    fmt.Sprintf("%d", pendingOrders),
		"purchases_total":   fmt.Sprintf("%.2f", totalPurchases),
		"gross_profit":      fmt.Sprintf("%.2f", grossProfit),
		"low_stock_count":   fmt.Sprintf("%d", len(lowStockProducts)),
		"total_vat":         fmt.Sprintf("%.2f", outputVAT),
		"purchase_vat":      fmt.Sprintf("%.2f", inputVAT),
		"net_vat_payable":   fmt.Sprintf("%.2f", outputVAT-inputVAT),
		"credit_note_count":       fmt.Sprintf("%d", creditNoteCount),
		"credit_note_total":       fmt.Sprintf("%.2f", creditNoteTotal),
		"total_discount":          fmt.Sprintf("%.2f", totalDiscount),
		"revenue_before_vat":      fmt.Sprintf("%.2f", totalRevenueBeforeVAT),
		"purchases_before_vat":    fmt.Sprintf("%.2f", totalPurchasesBeforeVAT),
	}
	if invErr != nil {
		stats["invoices"] = statPlaceholder
		stats["revenue"] = statPlaceholder
		stats["total_vat"] = statPlaceholder
		stats["net_vat_payable"] = statPlaceholder
		stats["credit_note_count"] = statPlaceholder
		stats["credit_note_total"] = statPlaceholder
		stats["total_discount"] = statPlaceholder
		stats["revenue_before_vat"] = statPlaceholder
	}
	if prodErr != nil {
		stats["products"] = statPlaceholder
		stats["low_stock_count"] = statPlaceholder
	}
	if clientErr != nil {
		stats["clients"] = statPlaceholder
	}
	if suppErr != nil {
		stats["suppliers"] = statPlaceholder
	}
	if orderErr != nil {
		stats["pending_orders"] = statPlaceholder
	}
	if pbErr != nil {
		stats["purchases_total"] = statPlaceholder
		stats["gross_profit"] = statPlaceholder
		stats["purchase_vat"] = statPlaceholder
		stats["net_vat_payable"] = statPlaceholder
		stats["purchases_before_vat"] = statPlaceholder
	}

	recentInvoices := make([]map[string]interface{}, 0)
	for i, inv := range filteredInvoices {
		if i >= 10 {
			break
		}
		status, statusClass := helpers.InvoiceStatus(inv)
		status = helpers.TranslateInvoiceStatus(status)
		recentInvoices = append(recentInvoices, map[string]interface{}{
			"id":              inv.ID,
			"sequence_number": inv.SequenceNumber,
			"total":           fmt.Sprintf("%.2f", inv.Total),
			"date":            helpers.FormatInvoiceDate(inv.EffectiveDate.Time),
			"status":          status,
			"status_class":    statusClass,
		})
	}

	statusCountsTemplate := map[string]int{
		"draft":      statusCounts[0],
		"processing": statusCounts[1],
		"processed":  statusCounts[2],
		"issued":     statusCounts[3],
	}

	labelsJSON, _ := json.Marshal(monthLabels)
	revenueJSON, _ := json.Marshal(revenueSeries)
	purchasesJSON, _ := json.Marshal(purchaseSeries)
	statusJSON, _ := json.Marshal([]int{
		statusCounts[0], statusCounts[1], statusCounts[2], statusCounts[3],
	})

	// Build KPI trends from real data
	kpiTrends := map[string]interface{}{}
	realTrends := helpers.ComputeKPITrends(invoices, purchaseBills, products, clients, orders, startDate, endDate)
	for k, v := range realTrends {
		kpiTrends[k] = map[string]interface{}{"Arrow": v.Arrow, "Percent": v.Percent, "Class": v.Class}
	}

	// ── Analytics computations ─────────────────────────────────────
	// AR / AP Aging
	arAging := helpers.ComputeAgingAnalysis(filteredInvoices)
	apAging := helpers.ComputeAPAging(purchaseBills)

	// Cash flow
	cfPoints := helpers.ComputeCashFlow(filteredInvoices, purchaseBills, monthLabels)
	cfInflow := make([]float64, len(cfPoints))
	cfOutflow := make([]float64, len(cfPoints))
	cfNet := make([]float64, len(cfPoints))
	for i, p := range cfPoints {
		cfInflow[i] = p.Inflow
		cfOutflow[i] = p.Outflow
		cfNet[i] = p.Net
	}
	cfInflowJSON, _ := json.Marshal(cfInflow)
	cfOutflowJSON, _ := json.Marshal(cfOutflow)
	cfNetJSON, _ := json.Marshal(cfNet)

	// P&L
	pnl := helpers.ComputePnL(filteredInvoices, purchaseBills, monthLabels)
	pnlRevenueJSON, _ := json.Marshal(pnl.MonthRevenue)
	pnlCOGSJSON, _ := json.Marshal(pnl.MonthCOGS)
	pnlProfitJSON, _ := json.Marshal(pnl.MonthProfit)

	// Inventory turnover
	turnoverRatio, _, _ := helpers.ComputeInventoryTurnover(products, totalPurchases)

	// Fulfillment rate
	fulfillmentRate := helpers.ComputeFulfillmentRate(orders, filteredInvoices)

	// Return rate
	returnRateData := helpers.ComputeReturnRate(filteredInvoices, monthLabels)
	returnMonthlyJSON, _ := json.Marshal(returnRateData.MonthlyRates)

	// Processing time
	avgProcessingTime := helpers.ComputeAvgProcessingTime(orders, filteredInvoices)

	// CLV — top clients
	clvMap := helpers.ComputeCLV(orders)
	topCLV := make([]map[string]interface{}, 0)
	if orderErr == nil {
		topClients := helpers.ComputeTopClients(orders, 10)
		for _, tc := range topClients {
			name, _ := tc["name"].(string)
			value := 0.0
			if v, ok := clvMap[name]; ok {
				value = v
			}
			topCLV = append(topCLV, map[string]interface{}{
				"name":  name,
				"value": fmt.Sprintf("%.2f", value),
			})
		}
	}

	// Margin tiers
	marginTiers := helpers.ComputeGrossMarginByTier(products)

	// Supplier performance
	supplierPerf := helpers.ComputeSupplierPerformance(suppliers, purchaseBills, 10)

	// YoY revenue
	yoyRevenue := helpers.ComputeYoYRevenue(invoices, monthLabels)
	yoyJSON, _ := json.Marshal(yoyRevenue)

	// Moving average (3-month window on revenue)
	movingAvg := helpers.ComputeMovingAverage(revenueSeries, 3)
	movingAvgJSON, _ := json.Marshal(movingAvg)

	// Weekday revenue
	weekdayRevenue := helpers.ComputeWeekdayRevenue(filteredInvoices)
	weekdayJSON, _ := json.Marshal(weekdayRevenue)

	// Top products
	topProdLabels, topProdValues := helpers.ComputeTopProducts(products, 5)
	topProdLabelsJSON, _ := json.Marshal(topProdLabels)
	topProdValuesJSON, _ := json.Marshal(topProdValues)

	// Client distribution
	clientDistLabels, clientDistValues := helpers.ComputeClientsDistribution(clients, orders)
	clientDistLabelsJSON, _ := json.Marshal(clientDistLabels)
	clientDistValuesJSON, _ := json.Marshal(clientDistValues)

	data := map[string]interface{}{
		"title":              "لوحة التحكم",
		"dashboard_warnings": dashboardWarnings,
		"stats":              stats,
		"kpi_trends":         kpiTrends,
		"chart_labels":       template.JS(labelsJSON),
		"chart_revenue":      template.JS(revenueJSON),
		"chart_purchases":    template.JS(purchasesJSON),
		"chart_status":       template.JS(statusJSON),
		"recent_invoices":    recentInvoices,
		"status_counts":      statusCountsTemplate,
		"low_stock_products": lowStockProducts,
		"state_filter":       stateFilter,
		"start_date":         startDate,
		"end_date":           endDate,
		"user_role":          helpers.GetUserRole(r),
		"version":            config.AppVersion,

		// Analytics — Aging
		"ar_aging": arAging,
		"ap_aging": apAging,

		// Analytics — Cash Flow
		"cf_inflow":  template.JS(cfInflowJSON),
		"cf_outflow": template.JS(cfOutflowJSON),
		"cf_net":     template.JS(cfNetJSON),

		// Analytics — P&L
		"pnl": map[string]interface{}{
			"Revenue":     pnl.Revenue,
			"COGS":        pnl.COGS,
			"GrossProfit": pnl.GrossProfit,
			"GrossMargin": pnl.GrossMargin,
		},
		"pnl_revenue": template.JS(pnlRevenueJSON),
		"pnl_cogs":    template.JS(pnlCOGSJSON),
		"pnl_profit":  template.JS(pnlProfitJSON),

		// Analytics — KPIs
		"inv_turnover":    fmt.Sprintf("%.2f", turnoverRatio),
		"fulfillment_rate": fmt.Sprintf("%.1f", fulfillmentRate),
		"return_rate": map[string]interface{}{
			"ReturnRate":    returnRateData.ReturnRate,
			"CreditNotes":  returnRateData.CreditNotes,
			"TotalInvoices": returnRateData.TotalInvoices,
		},
		"avg_processing_time": fmt.Sprintf("%.1f", avgProcessingTime),

		// Analytics — Tables
		"top_clv":       topCLV,
		"margin_tiers":  marginTiers,
		"supplier_perf": supplierPerf,

		// Analytics — Charts
		"yoy_revenue":     template.JS(yoyJSON),
		"moving_avg":      template.JS(movingAvgJSON),
		"weekday_revenue": template.JS(weekdayJSON),
		"return_monthly":  template.JS(returnMonthlyJSON),

		// Analytics — Product & Client distribution
		"top_prod_labels":    template.JS(topProdLabelsJSON),
		"top_prod_values":    template.JS(topProdValuesJSON),
		"client_dist_labels": template.JS(clientDistLabelsJSON),
		"client_dist_values": template.JS(clientDistValuesJSON),
	}

	// ── Advanced Metrics (Score ≥ 8) ───────────────────────────────
	// Profitability & Revenue
	avgInvoiceValue := helpers.ComputeAverageInvoiceValue(filteredInvoices)
	revenuePerClient := helpers.ComputeRevenuePerClient(totalRevenue, len(clients))
	momGrowth := helpers.ComputeMoMGrowth(revenueSeries)
	momGrowthJSON, _ := json.Marshal(momGrowth)
	purchaseToSalesRatio := helpers.ComputePurchaseToSalesRatio(totalPurchases, totalRevenue)
	discountRate := helpers.ComputeDiscountRate(totalDiscount, totalRevenue)
	effectiveVATRate := helpers.ComputeEffectiveVATRate(outputVAT, totalRevenue)
	draftToIssuedRate := helpers.ComputeDraftToIssuedRate(statusCounts)
	revenueByStatus := helpers.ComputeRevenueByStatus(filteredInvoices)

	// Inventory
	inventoryValue := helpers.ComputeInventoryValue(products)
	outOfStockCount, outOfStockRate := helpers.ComputeOutOfStock(products)
	currentMonthRevenue := 0.0
	if len(revenueSeries) > 0 {
		currentMonthRevenue = revenueSeries[len(revenueSeries)-1]
	}
	inventoryToSalesRatio := helpers.ComputeInventoryToSalesRatio(inventoryValue, currentMonthRevenue)
	dio := helpers.ComputeDIO(turnoverRatio)

	// AR/AP & Liquidity
	arTotal, arCount := helpers.ComputeTotalAROutstanding(arAging)
	apTotal, apCount := helpers.ComputeTotalAPOutstanding(apAging)
	netCashPosition := helpers.ComputeNetCashPosition(arTotal, apTotal)
	recvTurnover := helpers.ComputeReceivablesTurnover(totalRevenue, arTotal)
	avgCollectionPeriod := helpers.ComputeAvgCollectionPeriod(recvTurnover)
	apTurnoverVal := helpers.ComputeAPTurnover(totalPurchases, apTotal)
	avgPaymentPeriod := helpers.ComputeAvgPaymentPeriod(apTurnoverVal)
	dso := helpers.ComputeDSO(arTotal, totalRevenue, 365)
	dpo := helpers.ComputeDPO(apTotal, totalPurchases, 365)
	cashConversionCycle := helpers.ComputeCashConversionCycle(dio, dso, dpo)
	cashBurnRate := helpers.ComputeCashBurnRate(cfPoints)

	// Concentration risk from client values
	clientValues := make([]float64, 0, len(clvMap))
	for _, v := range clvMap {
		clientValues = append(clientValues, v)
	}
	clientConcentration := helpers.ComputeConcentrationRisk(clientValues, 3)

	// VAT quarterly
	vatQuarterly := helpers.ComputeVATQuarterly(filteredInvoices, purchaseBills)

	// Revenue by status for chart
	revDraft := revenueByStatus[0]
	revProcessing := revenueByStatus[1]
	revProcessed := revenueByStatus[2]
	revIssued := revenueByStatus[3]
	revByStatusJSON, _ := json.Marshal([]float64{revDraft, revProcessing, revProcessed, revIssued})

	// Mock data for metrics requiring backend data not yet available
	mockBS := helpers.GetMockBalanceSheet(totalRevenue, totalPurchases)
	mockOpEx := helpers.GetMockOperatingExpenses(totalRevenue, grossProfit)
	mockZATCA := helpers.GetMockZATCACompliance(len(filteredInvoices))
	mockPayment := helpers.GetMockPaymentTracking(len(filteredInvoices), totalRevenue)
	mockLiquidity := helpers.GetMockLiquidityRatios(mockBS, inventoryValue)

	// Add advanced metrics to data map
	data["avg_invoice_value"] = fmt.Sprintf("%.2f", avgInvoiceValue)
	data["revenue_per_client"] = fmt.Sprintf("%.2f", revenuePerClient)
	data["mom_growth"] = template.JS(momGrowthJSON)
	data["purchase_to_sales_ratio"] = fmt.Sprintf("%.1f", purchaseToSalesRatio)
	data["discount_rate"] = fmt.Sprintf("%.2f", discountRate)
	data["effective_vat_rate"] = fmt.Sprintf("%.2f", effectiveVATRate)
	data["draft_to_issued_rate"] = fmt.Sprintf("%.1f", draftToIssuedRate)
	data["rev_by_status"] = template.JS(revByStatusJSON)

	// Inventory metrics
	data["inventory_value"] = fmt.Sprintf("%.2f", inventoryValue)
	data["out_of_stock_count"] = outOfStockCount
	data["out_of_stock_rate"] = fmt.Sprintf("%.1f", outOfStockRate)
	data["inventory_to_sales_ratio"] = fmt.Sprintf("%.1f", inventoryToSalesRatio)
	data["dio"] = fmt.Sprintf("%.1f", dio)

	// AR/AP metrics
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

	// Mock-based metrics
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
	if err := config.DashboardTemplate.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// comparePeriodStats filters invoices & purchase bills for a date range and returns KPIs.
func comparePeriodStats(invoices, purchaseBills []models.Invoice, start, end time.Time) map[string]string {
	var filtered []models.Invoice
	var filteredPB []models.Invoice
	for _, inv := range invoices {
		if inv.EffectiveDate.Time == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, inv.EffectiveDate.Time)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, inv.EffectiveDate.Time)
			if err != nil {
				continue
			}
		}
		if !parsed.Before(start) && !parsed.After(end) {
			filtered = append(filtered, inv)
		}
	}
	for _, pb := range purchaseBills {
		if pb.EffectiveDate.Time == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, pb.EffectiveDate.Time)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, pb.EffectiveDate.Time)
			if err != nil {
				continue
			}
		}
		if !parsed.Before(start) && !parsed.After(end) {
			filteredPB = append(filteredPB, pb)
		}
	}

	var revenue, purchases, pending float64
	statusCounts := map[int]int{0: 0, 1: 0, 2: 0, 3: 0}
	for _, inv := range filtered {
		revenue += inv.Total
		statusCounts[inv.State]++
		if inv.State == 0 || inv.State == 1 {
			pending += inv.Total
		}
	}
	for _, pb := range filteredPB {
		purchases += pb.Total
	}
	profit := revenue - purchases
	avg := 0.0
	if len(filtered) > 0 {
		avg = revenue / float64(len(filtered))
	}
	margin := 0.0
	if revenue > 0 {
		margin = profit * 100 / revenue
	}
	return map[string]string{
		"invoices":    fmt.Sprintf("%d", len(filtered)),
		"revenue":     fmt.Sprintf("%.2f", revenue),
		"purchases":   fmt.Sprintf("%.2f", purchases),
		"profit":      fmt.Sprintf("%.2f", profit),
		"avg_invoice": fmt.Sprintf("%.2f", avg),
		"pending":     fmt.Sprintf("%.2f", pending),
		"margin":      fmt.Sprintf("%.1f", margin),
		"issued":      fmt.Sprintf("%d", statusCounts[3]),
		"draft":       fmt.Sprintf("%d", statusCounts[0]),
	}
}

// HandleDashboardCompare returns an HTMX partial comparing two periods.
func HandleDashboardCompare(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	aStart := r.URL.Query().Get("a_start")
	aEnd := r.URL.Query().Get("a_end")
	bStart := r.URL.Query().Get("b_start")
	bEnd := r.URL.Query().Get("b_end")

	if aStart == "" || aEnd == "" || bStart == "" || bEnd == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<div class="text-center text-gray-400 text-sm py-4">يرجى تحديد فترتين للمقارنة</div>`)
		return
	}

	parseDate := func(s string) time.Time {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}
		}
		return t
	}

	periodAStart := parseDate(aStart)
	periodAEnd := parseDate(aEnd).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	periodBStart := parseDate(bStart)
	periodBEnd := parseDate(bEnd).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// Fetch invoices & purchase bills
	var invoices, purchaseBills []models.Invoice
	var invErr, pbErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); invoices, invErr = helpers.FetchAllInvoicesUnpaginated(token) }()
	go func() { defer wg.Done(); purchaseBills, pbErr = helpers.FetchPurchaseBillsAll(token, 1, "") }()
	wg.Wait()

	for _, err := range []error{invErr, pbErr} {
		if err != nil && helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
	}

	a := comparePeriodStats(invoices, purchaseBills, periodAStart, periodAEnd)
	b := comparePeriodStats(invoices, purchaseBills, periodBStart, periodBEnd)

	type row struct {
		Label string
		A     string
		B     string
	}
	rows := []row{
		{"عدد الفواتير", a["invoices"], b["invoices"]},
		{"الإيرادات", a["revenue"], b["revenue"]},
		{"المشتريات", a["purchases"], b["purchases"]},
		{"الربح", a["profit"], b["profit"]},
		{"متوسط الفاتورة", a["avg_invoice"], b["avg_invoice"]},
		{"المعلقات", a["pending"], b["pending"]},
		{"هامش الربح %", a["margin"] + "%", b["margin"] + "%"},
		{"فواتير صادرة", a["issued"], b["issued"]},
		{"مسودات", a["draft"], b["draft"]},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<table class="w-full text-sm border-collapse"><thead><tr class="bg-gray-50 dark:bg-gray-700">`)
	fmt.Fprintf(w, `<th class="p-2 text-right font-semibold text-gray-600 dark:text-gray-300 border-b dark:border-gray-600">المؤشر</th>`)
	fmt.Fprintf(w, `<th class="p-2 text-center font-semibold text-blue-600 border-b dark:border-gray-600">الفترة أ<br><span class="text-xs font-normal text-gray-400">%s → %s</span></th>`, aStart, aEnd)
	fmt.Fprintf(w, `<th class="p-2 text-center font-semibold text-purple-600 border-b dark:border-gray-600">الفترة ب<br><span class="text-xs font-normal text-gray-400">%s → %s</span></th>`, bStart, bEnd)
	fmt.Fprint(w, `</tr></thead><tbody>`)
	for i, rw := range rows {
		bg := ""
		if i%2 == 0 {
			bg = ` class="bg-gray-50/50 dark:bg-gray-800/30"`
		}
		fmt.Fprintf(w, `<tr%s><td class="p-2 text-right text-gray-700 dark:text-gray-300 border-b dark:border-gray-700">%s</td>`, bg, rw.Label)
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
	go func() { defer wg.Done(); purchaseBills, pbErr = helpers.FetchPurchaseBillsAll(token, 1, "") }()
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

	// ── Render print-friendly HTML ─────────────────────────────
	nowStr := time.Now().Format("2006-01-02 15:04")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
<meta charset="UTF-8">
<title>تقرير لوحة التحكم — %s</title>
<style>
  @media print { @page { size: A4 landscape; margin: 10mm; } body { -webkit-print-color-adjust: exact; print-color-adjust: exact; } .no-print { display:none; } }
  * { margin:0; padding:0; box-sizing:border-box; }
  body { font-family: 'Segoe UI', Tahoma, sans-serif; direction: rtl; color: #1e293b; padding: 20px; font-size: 12px; background: #fff; }
  h1 { font-size: 20px; margin-bottom: 4px; }
  .sub { color: #64748b; font-size: 11px; margin-bottom: 16px; }
  table { width: 100%%; border-collapse: collapse; margin-bottom: 16px; }
  th, td { border: 1px solid #cbd5e1; padding: 6px 10px; text-align: right; }
  th { background: #f1f5f9; font-weight: 600; font-size: 11px; }
  td { font-size: 12px; }
  .section { margin-bottom: 18px; }
  .section-title { font-size: 14px; font-weight: 700; margin-bottom: 6px; border-bottom: 2px solid #3b82f6; padding-bottom: 4px; color: #1e40af; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .badge { display:inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
  .bg-green { background: #dcfce7; color: #166534; } .bg-yellow { background: #fef9c3; color: #854d0e; }
  .bg-orange { background: #ffedd5; color: #9a3412; } .bg-red { background: #fee2e2; color: #991b1b; }
  .print-btn { position: fixed; bottom: 20px; left: 20px; background: #3b82f6; color: #fff; border: none; padding: 10px 24px; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 600; box-shadow: 0 2px 8px rgba(0,0,0,.15); }
  .print-btn:hover { background: #2563eb; }
</style>
</head>
<body>
<h1>📊 تقرير لوحة التحكم</h1>
<p class="sub">تاريخ التقرير: %s &nbsp;|&nbsp; إصدار: %s</p>

<div class="section">
  <div class="section-title">المؤشرات الأساسية</div>
  <table>
    <tr>
      <th>المؤشر</th><th>القيمة</th><th>المؤشر</th><th>القيمة</th>
    </tr>
    <tr>
      <td>إجمالي الفواتير</td><td><b>%d</b></td>
      <td>إجمالي الإيرادات</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>إجمالي المشتريات</td><td><b>%.2f</b></td>
      <td>صافي الربح</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>متوسط الفاتورة</td><td><b>%.2f</b></td>
      <td>المبالغ المعلقة</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>المنتجات</td><td>%d</td>
      <td>العملاء</td><td>%d</td>
    </tr>
    <tr>
      <td>الموردين</td><td>%d</td>
      <td>الطلبات المعلقة</td><td>%d</td>
    </tr>
    <tr>
      <td>منتجات منخفضة المخزون</td><td>%d</td>
      <td>هامش الربح</td><td><b>%.1f%%</b></td>
    </tr>
    <tr>
      <td>معدل دوران المخزون</td><td><b>%.2f</b></td>
      <td>&nbsp;</td><td>&nbsp;</td>
    </tr>
  </table>
</div>

<div class="section">
  <div class="section-title">حالة الفواتير</div>
  <table>
    <tr><th>مسودة</th><th>قيد المعالجة</th><th>تمت المعالجة</th><th>صادرة</th></tr>
    <tr><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>
  </table>
</div>
`,
		nowStr, nowStr, config.AppVersion,
		len(invoices), totalRevenue,
		totalPurchases, grossProfit,
		avgInvoice, pendingAmount,
		len(products), len(clients),
		len(suppliers), pendingOrders,
		lowStockCount, profitMargin,
		turnoverRatio,
		statusCounts[0], statusCounts[1], statusCounts[2], statusCounts[3],
	)

	// ── Aging analysis ─────────────────────────────────────────
	fmt.Fprintf(w, `<div class="grid2"><div class="section"><div class="section-title">تحليل أعمار المستحقات</div><table><tr><th>الفئة</th><th>العدد</th><th>المبلغ</th></tr>`)
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
	fmt.Fprintf(w, `<div class="section"><div class="section-title">أفضل العملاء</div><table><tr><th>#</th><th>العميل</th><th>الطلبات</th><th>الإجمالي</th><th>القيمة الدائمة</th></tr>`)
	for _, tc := range topClients {
		fmt.Fprintf(w, `<tr><td>%v</td><td>%v</td><td>%v</td><td>%v</td><td>%v</td></tr>`,
			tc["order"], tc["name"], tc["count"], tc["total"], tc["clv"])
	}
	fmt.Fprintf(w, `</table></div></div>`)

	// ── VAT Section ────────────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">ضريبة القيمة المضافة — هيئة الزكاة (ZATCA)</div>
  <table>
    <tr><th>المؤشر</th><th>القيمة</th><th>المؤشر</th><th>القيمة</th></tr>
    <tr>
      <td>ضريبة المخرجات (مبيعات)</td><td><b>%.2f</b></td>
      <td>ضريبة المدخلات (مشتريات)</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>صافي الضريبة المستحقة</td><td><b style="color:#1e40af">%.2f</b></td>
      <td>معدل الضريبة الفعلي</td><td><b>%.2f%%</b></td>
    </tr>
  </table>
</div>`, outputVAT, inputVAT, netVATPayable, effectiveVATRate)

	// ── VAT Quarterly Table ────────────────────────────────────
	if len(vatQuarterly) > 0 {
		fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">الضريبة الربع سنوية</div>
  <table>
    <tr><th>الربع</th><th>ضريبة المخرجات</th><th>ضريبة المدخلات</th><th>صافي الضريبة</th></tr>`)
		for _, q := range vatQuarterly {
			fmt.Fprintf(w, `<tr><td>%s</td><td style="color:#166534">%.2f</td><td style="color:#991b1b">%.2f</td><td><b style="color:#1e40af">%.2f</b></td></tr>`,
				q.Quarter, q.OutputVAT, q.InputVAT, q.NetVAT)
		}
		fmt.Fprintf(w, `</table></div>`)
	}

	// ── Advanced Profitability ─────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">مؤشرات الربحية المتقدمة</div>
  <table>
    <tr><th>المؤشر</th><th>القيمة</th><th>المؤشر</th><th>القيمة</th></tr>
    <tr>
      <td>متوسط قيمة الفاتورة</td><td><b>%.2f</b></td>
      <td>الإيراد لكل عميل</td><td><b>%.2f</b></td>
    </tr>
    <tr>
      <td>نسبة المشتريات إلى المبيعات</td><td><b>%.1f%%</b></td>
      <td>نسبة الخصومات</td><td><b>%.2f%%</b></td>
    </tr>
    <tr>
      <td>نسبة المسودات إلى الصادرة</td><td><b>%.1f%%</b></td>
      <td>&nbsp;</td><td>&nbsp;</td>
    </tr>
  </table>
</div>`, avgInvoiceValue, revenuePerClient, purchaseToSalesRatio, discountRate, draftToIssuedRate)

	// ── Receivables & Payables ─────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">مؤشرات الذمم والتحصيل</div>
  <table>
    <tr><th>المؤشر</th><th>القيمة</th><th>المؤشر</th><th>القيمة</th></tr>
    <tr>
      <td>إجمالي المستحقات (AR)</td><td><b>%.2f</b> (%d فاتورة)</td>
      <td>إجمالي المطلوبات (AP)</td><td><b>%.2f</b> (%d فاتورة)</td>
    </tr>
    <tr>
      <td>صافي المركز النقدي</td><td><b style="color:#1e40af">%.2f</b></td>
      <td>أيام تحصيل المبيعات (DSO)</td><td><b>%.1f</b> يوم</td>
    </tr>
    <tr>
      <td>أيام سداد المشتريات (DPO)</td><td><b>%.1f</b> يوم</td>
      <td>دورة التحويل النقدي (CCC)</td><td><b>%.1f</b> يوم</td>
    </tr>
    <tr>
      <td>معدل دوران المستحقات</td><td><b>%.2f</b>x</td>
      <td>متوسط فترة التحصيل</td><td><b>%.1f</b> يوم</td>
    </tr>
    <tr>
      <td>معدل دوران المطلوبات</td><td><b>%.2f</b>x</td>
      <td>متوسط فترة السداد</td><td><b>%.1f</b> يوم</td>
    </tr>
  </table>
</div>`, arTotal, arCount, apTotal, apCount, netCashPosition, dso, dpo, cashConversionCycle,
		recvTurnover, avgCollectionPeriod, apTurnoverVal, avgPaymentPeriod)

	// ── AP Aging ───────────────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">تحليل أعمار المطلوبات (AP)</div>
  <table><tr><th>الفئة</th><th>العدد</th><th>المبلغ</th></tr>`)
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
  <div class="section-title">مؤشرات المخزون المتقدمة</div>
  <table>
    <tr><th>المؤشر</th><th>القيمة</th><th>المؤشر</th><th>القيمة</th></tr>
    <tr>
      <td>قيمة المخزون</td><td><b>%.2f</b></td>
      <td>نفاد المخزون</td><td><b>%d</b> (%.1f%%)</td>
    </tr>
    <tr>
      <td>نسبة المخزون إلى المبيعات</td><td><b>%.1f</b>x</td>
      <td>أيام المخزون (DIO)</td><td><b>%.1f</b> يوم</td>
    </tr>
  </table>
</div>`, inventoryValue, outOfStockCount, outOfStockRate, inventoryToSalesRatio, dio)

	// ── Concentration Risk ─────────────────────────────────────
	fmt.Fprintf(w, `
<div class="section">
  <div class="section-title">مؤشرات المخاطر</div>
  <table>
    <tr><th>المؤشر</th><th>القيمة</th></tr>
    <tr><td>تركز العملاء (أعلى 3)</td><td><b>%.1f%%</b></td></tr>
  </table>
</div>`, clientConcentration)

	// ── Footer & print trigger ─────────────────────────────────
	fmt.Fprintf(w, `
<button class="print-btn no-print" onclick="window.print()">🖨️ طباعة / حفظ PDF</button>
<script>
  // Auto-open print dialog after page loads
  window.addEventListener('load', function() {
    setTimeout(function() { window.print(); }, 500);
  });
</script>
</body></html>`)
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

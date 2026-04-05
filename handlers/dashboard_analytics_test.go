package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDashboardAnalyticsSections verifies the Sprint 7 P0 analytics sections render.
func TestDashboardAnalyticsSections(t *testing.T) {
	req, err := http.NewRequest("GET", "/dashboard-test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleDashboardTest)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("Dashboard returned %v, expected 200", status)
	}

	body := rr.Body.String()

	// AC1: KPI trend arrows should appear on the cards
	trendChecks := []string{
		"↑",   // trend arrow up
		"12.5%", // trend percent on invoices
	}
	for _, check := range trendChecks {
		if !strings.Contains(body, check) {
			t.Errorf("KPI trend display missing: %s", check)
		}
	}

	// AC2: AR Aging section
	arChecks := []string{
		"تقادم الذمم المدينة",
		"0-30",
		"31-60",
		"61-90",
		"90&#43;", // "90+" is HTML-escaped by Go templates
	}
	for _, check := range arChecks {
		if !strings.Contains(body, check) {
			t.Errorf("AR Aging section missing: %s", check)
		}
	}

	// AC3: AP Aging section
	apChecks := []string{
		"تقادم الذمم الدائنة",
	}
	for _, check := range apChecks {
		if !strings.Contains(body, check) {
			t.Errorf("AP Aging section missing: %s", check)
		}
	}

	// AC4: Cash Flow chart
	cfChecks := []string{
		"التدفق النقدي",
		"cashFlowChart",
		"التدفق الداخل",
		"التدفق الخارج",
		"صافي التدفق",
	}
	for _, check := range cfChecks {
		if !strings.Contains(body, check) {
			t.Errorf("Cash Flow section missing: %s", check)
		}
	}

	// AC5: P&L Summary
	pnlChecks := []string{
		"قائمة الأرباح والخسائر",
		"الإيرادات",
		"تكلفة البضاعة المباعة",
		"إجمالي الربح",
		"هامش الربح",
		"pnlChart",
	}
	for _, check := range pnlChecks {
		if !strings.Contains(body, check) {
			t.Errorf("P&L section missing: %s", check)
		}
	}
}

// TestDashboardAnalyticsChartData verifies JS chart data variables are injected.
func TestDashboardAnalyticsChartData(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	jsDataChecks := []string{
		"cfInflowData",
		"cfOutflowData",
		"cfNetData",
		"pnlRevenueData",
		"pnlCOGSData",
		"pnlProfitData",
	}
	for _, check := range jsDataChecks {
		if !strings.Contains(body, check) {
			t.Errorf("Chart JS data variable missing: %s", check)
		}
	}
}

// TestDashboardAgingTableTotals verifies AR aging table shows formatted totals.
func TestDashboardAgingTableTotals(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	// Mock data has AR aging with specific totals
	arTotals := []string{
		"25000.00",
		"18000.00",
		"8000.00",
	}
	for _, total := range arTotals {
		if !strings.Contains(body, total) {
			t.Errorf("AR aging table missing total: %s", total)
		}
	}
}

// TestDashboardPnLValues verifies P&L section displays correct summary values.
func TestDashboardPnLValues(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	pnlValues := []string{
		"2350000.00",  // Revenue
		"1200000.00",  // COGS
		"1150000.00",  // Gross Profit
		"48.9%",       // Margin
	}
	for _, val := range pnlValues {
		if !strings.Contains(body, val) {
			t.Errorf("P&L summary missing value: %s", val)
		}
	}
}

// ── Sprint 7 P1 Tests ──────────────────────────────────────────────────

// TestDashboardP1KPIRow verifies P1 KPI cards (inventory turnover, fulfillment, return rate).
func TestDashboardP1KPIRow(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	checks := []string{
		"معدل دوران المخزون",
		"2.50",
		"معدل تنفيذ الطلبات",
		"78.5%",
		"معدل المرتجعات",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("P1 KPI row missing: %s", check)
		}
	}
}

// TestDashboardP1CLVTable verifies top CLV clients table renders.
func TestDashboardP1CLVTable(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	checks := []string{
		"أعلى العملاء قيمة",
		"شركة الفارس",
		"125000.00",
		"مؤسسة النجم",
		"98000.00",
		"إجمالي القيمة",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("CLV table missing: %s", check)
		}
	}
}

// TestDashboardP1Charts verifies P1 chart canvases and JS data variables.
func TestDashboardP1Charts(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	// Canvas elements
	canvasChecks := []string{
		"yoyChart",
		"movingAvgChart",
		"weekdayChart",
		"returnRateChart",
	}
	for _, check := range canvasChecks {
		if !strings.Contains(body, check) {
			t.Errorf("P1 chart canvas missing: %s", check)
		}
	}

	// JS data variables
	jsChecks := []string{
		"yoyRevenueData",
		"movingAvgData",
		"weekdayData",
		"returnMonthlyData",
	}
	for _, check := range jsChecks {
		if !strings.Contains(body, check) {
			t.Errorf("P1 chart JS variable missing: %s", check)
		}
	}
}

// TestDashboardP1MarginTiers verifies margin tier section renders with tier data.
func TestDashboardP1MarginTiers(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	checks := []string{
		"توزيع المنتجات حسب فئة السعر",
		"&lt; 50 ر.س",
		"50-200 ر.س",
		"200-500 ر.س",
		"500&#43; ر.س",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("Margin tier section missing: %s", check)
		}
	}
}

// TestDashboardP1SectionTitles verifies all P1 section headings are present.
func TestDashboardP1SectionTitles(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	titles := []string{
		"مقارنة الإيرادات (سنوي)",
		"المتوسط المتحرك للإيرادات",
		"متوسط الإيرادات حسب أيام الأسبوع",
		"معدل المرتجعات الشهري",
	}
	for _, title := range titles {
		if !strings.Contains(body, title) {
			t.Errorf("P1 section title missing: %s", title)
		}
	}
}

// TestDashboardP1WeekdayLabels verifies Arabic weekday labels in chart script.
func TestDashboardP1WeekdayLabels(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	days := []string{
		"السبت",
		"الأحد",
		"الاثنين",
		"الثلاثاء",
		"الأربعاء",
		"الخميس",
		"الجمعة",
	}
	for _, day := range days {
		if !strings.Contains(body, day) {
			t.Errorf("Weekday label missing: %s", day)
		}
	}
}

// ── Sprint 7 P2 Tests ──────────────────────────────────────────────────

// TestDashboardP2SupplierPerf verifies supplier performance table renders.
func TestDashboardP2SupplierPerf(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	checks := []string{
		"أداء الموردين",
		"شركة التوريدات",
		"مؤسسة المعدات",
		"عدد الفواتير",
		"إجمالي الإنفاق",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("Supplier performance missing: %s", check)
		}
	}
}

// TestDashboardP2ProcessingTime verifies avg processing time KPI.
func TestDashboardP2ProcessingTime(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	checks := []string{
		"متوسط وقت المعالجة",
		"3.2",
		"يوم",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("Processing time KPI missing: %s", check)
		}
	}
}

// TestDashboardP2Charts verifies P2 chart canvases and JS variables.
func TestDashboardP2Charts(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	canvasChecks := []string{
		"topProductsChart",
		"clientDistChart",
	}
	for _, check := range canvasChecks {
		if !strings.Contains(body, check) {
			t.Errorf("P2 chart canvas missing: %s", check)
		}
	}

	jsChecks := []string{
		"topProdLabels",
		"topProdValues",
		"clientDistLabels",
		"clientDistValues",
	}
	for _, check := range jsChecks {
		if !strings.Contains(body, check) {
			t.Errorf("P2 chart JS variable missing: %s", check)
		}
	}
}

// TestDashboardP2SectionTitles verifies all P2 section headings.
func TestDashboardP2SectionTitles(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	titles := []string{
		"أعلى المنتجات (حسب الكمية)",
		"توزيع العملاء",
		"مشاركة لوحة التحكم",
	}
	for _, title := range titles {
		if !strings.Contains(body, title) {
			t.Errorf("P2 section/element missing: %s", title)
		}
	}
}

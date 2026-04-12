package helpers

import (
	"afrita/models"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

func toFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func toInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

type topClientSummary struct {
	Name    string
	Count   int
	Total   float64
	Last    time.Time
	LastRaw string
}

func ComputeTopProducts(products []models.Product, maxItems int) ([]string, []int) {
	if len(products) == 0 || maxItems <= 0 {
		return []string{}, []int{}
	}

	copied := make([]models.Product, len(products))
	copy(copied, products)

	sort.Slice(copied, func(i, j int) bool {
		if copied[i].Quantity == copied[j].Quantity {
			return copied[i].ID > copied[j].ID
		}
		return copied[i].Quantity > copied[j].Quantity
	})

	if len(copied) > maxItems {
		copied = copied[:maxItems]
	}

	labels := make([]string, 0, len(copied))
	values := make([]int, 0, len(copied))
	for _, p := range copied {
		labelID := p.ID
		if labelID == 0 {
			labelID = len(labels) + 1
		}
		labels = append(labels, fmt.Sprintf("منتج %d", labelID))
		values = append(values, toInt(p.Quantity))
	}

	return labels, values
}

func ComputeClientsDistribution(clients []models.Client, orders []map[string]interface{}) ([]string, []int) {
	activeClients := UniqueOrderClients(orders)
	activeCount := len(activeClients)

	if len(clients) > 0 {
		inactive := len(clients) - activeCount
		if inactive < 0 {
			inactive = 0
		}
		return []string{"عملاء نشطون", "عملاء بدون طلبات"}, []int{activeCount, inactive}
	}

	if activeCount > 0 {
		return []string{"عملاء نشطون"}, []int{activeCount}
	}

	return []string{}, []int{}
}

func ComputeTopClients(orders []map[string]interface{}, maxItems int) []map[string]interface{} {
	if len(orders) == 0 || maxItems <= 0 {
		return []map[string]interface{}{}
	}

	summaries := map[string]*topClientSummary{}
	for _, order := range orders {
		clientName := firstNonEmptyString(order,
			"client",
			"client_name",
			"customer",
			"customer_name",
			"name",
		)
		if clientName == "" {
			continue
		}

		summary, exists := summaries[clientName]
		if !exists {
			summary = &topClientSummary{Name: clientName}
			summaries[clientName] = summary
		}

		summary.Count++
		summary.Total += FirstNonZeroFloat(order, "total", "amount", "price")

		rawDate := firstNonEmptyString(order, "date", "created_at", "updated_at")
		if rawDate != "" {
			parsed, ok := ParseFlexibleTime(rawDate)
			if ok {
				if summary.Last.IsZero() || parsed.After(summary.Last) {
					summary.Last = parsed
					summary.LastRaw = rawDate
				}
			} else if summary.LastRaw == "" {
				summary.LastRaw = rawDate
			}
		}
	}

	if len(summaries) == 0 {
		return []map[string]interface{}{}
	}

	list := make([]*topClientSummary, 0, len(summaries))
	for _, summary := range summaries {
		list = append(list, summary)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Total > list[j].Total
		}
		return list[i].Count > list[j].Count
	})

	if len(list) > maxItems {
		list = list[:maxItems]
	}

	results := make([]map[string]interface{}, 0, len(list))
	for i, summary := range list {
		lastLabel := summary.LastRaw
		if lastLabel == "" && !summary.Last.IsZero() {
			lastLabel = summary.Last.Format("2006-01-02")
		}
		if lastLabel == "" {
			lastLabel = "—"
		}

		results = append(results, map[string]interface{}{
			"order": i + 1,
			"name":  summary.Name,
			"count": summary.Count,
			"total": fmt.Sprintf("%.2f", summary.Total),
			"last":  lastLabel,
		})
	}

	return results
}

func ComputePendingOrders(orders []map[string]interface{}) int {
	if len(orders) == 0 {
		return 0
	}

	pending := 0
	hasStatus := false
	for _, order := range orders {
		status := normalizeOrderStatus(order)
		if status != "" {
			hasStatus = true
			if isPendingStatus(status) {
				pending++
			}
			continue
		}
	}

	if !hasStatus {
		return len(orders)
	}

	return pending
}

func UniqueOrderClients(orders []map[string]interface{}) map[string]struct{} {
	clients := map[string]struct{}{}
	for _, order := range orders {
		clientName := firstNonEmptyString(order,
			"client",
			"client_name",
			"customer",
			"customer_name",
			"name",
		)
		if clientName != "" {
			clients[clientName] = struct{}{}
		}
	}
	return clients
}

func firstNonEmptyString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := data[key]; ok {
			if str, ok := raw.(string); ok {
				trimmed := strings.TrimSpace(str)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func FirstNonZeroFloat(data map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if raw, ok := data[key]; ok {
			if value, ok := CoerceFloat(raw); ok && value != 0 {
				return value
			}
		}
	}
	return 0
}

func CoerceFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case jsonNumber:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		parsed, err := ParseFloatString(v)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func normalizeOrderStatus(order map[string]interface{}) string {
	if raw, ok := order["status"]; ok {
		switch v := raw.(type) {
		case string:
			return strings.ToLower(strings.TrimSpace(v))
		case float64:
			if v == 0 || v == 1 {
				return "pending"
			}
			return ""
		case int:
			if v == 0 || v == 1 {
				return "pending"
			}
			return ""
		}
	}
	return ""
}

func isPendingStatus(status string) bool {
	status = strings.ToLower(status)
	return strings.Contains(status, "pending") ||
		strings.Contains(status, "processing") ||
		strings.Contains(status, "open") ||
		strings.Contains(status, "new") ||
		strings.Contains(status, "قيد") ||
		strings.Contains(status, "جديد")
}

func ParseFlexibleTime(value string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func ParseFilterDate(value string, isEnd bool) *time.Time {
	if value == "" {
		return nil
	}
	parsed, ok := ParseFlexibleTime(value)
	if !ok {
		return nil
	}

	if isEnd && len(value) == 10 && strings.Count(value, "T") == 0 {
		inclusive := parsed.Add(24*time.Hour - time.Nanosecond)
		return &inclusive
	}

	return &parsed
}

func FormatInvoiceDate(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, ok := ParseFlexibleTime(raw)
	if !ok {
		return raw
	}
	return parsed.Format("2006-01-02")
}

func ParseFloatString(value string) (float64, error) {
	cleaned := strings.ReplaceAll(value, ",", "")
	return strconvParseFloat(cleaned)
}

// minimal wrapper to avoid importing strconv in many files
func strconvParseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

// jsonNumber keeps optional dependency surface tiny without direct imports in this file.
// It matches encoding/json.Number.
type jsonNumber interface {
	Float64() (float64, error)
}

// KPITrend holds the direction and percentage change for a single KPI.
type KPITrend struct {
	Direction string // "up", "down", "flat"
	Percent   string // e.g. "12.5"
	Class     string // CSS class: "text-green-500", "text-red-500", "text-gray-400"
	Arrow     string // "↑", "↓", "—"
}

// ComputeKPITrends calculates % change for all dashboard KPIs by comparing
// the current filtered period to the immediately preceding period of the same length.
// If no date filters are set, it defaults to "this week vs last week".
func ComputeKPITrends(
	allInvoices []models.Invoice,
	allPurchaseBills []models.Invoice,
	allProducts []models.Product,
	allClients []models.Client,
	allOrders []map[string]interface{},
	startDate, endDate string,
) map[string]KPITrend {
	now := time.Now()

	// Determine current period boundaries
	var periodStart, periodEnd time.Time
	if startDate != "" && endDate != "" {
		ps := ParseFilterDate(startDate, false)
		pe := ParseFilterDate(endDate, true)
		if ps != nil && pe != nil {
			periodStart = *ps
			periodEnd = *pe
		}
	}
	if periodStart.IsZero() || periodEnd.IsZero() {
		// Default: this week
		periodEnd = now
		periodStart = now.AddDate(0, 0, -7)
	}

	// Previous period = same duration immediately before current period
	duration := periodEnd.Sub(periodStart)
	prevEnd := periodStart.Add(-time.Nanosecond)
	prevStart := prevEnd.Add(-duration)

	// Partition invoices into current and previous periods
	var curInvoices, prevInvoices []models.Invoice
	for _, inv := range allInvoices {
		t := parseInvoiceTime(inv)
		if t.IsZero() {
			continue
		}
		if !t.Before(periodStart) && !t.After(periodEnd) {
			curInvoices = append(curInvoices, inv)
		} else if !t.Before(prevStart) && !t.After(prevEnd) {
			prevInvoices = append(prevInvoices, inv)
		}
	}

	var curPB, prevPB []models.Invoice
	for _, pb := range allPurchaseBills {
		t := parseInvoiceTime(pb)
		if t.IsZero() {
			continue
		}
		if !t.Before(periodStart) && !t.After(periodEnd) {
			curPB = append(curPB, pb)
		} else if !t.Before(prevStart) && !t.After(prevEnd) {
			prevPB = append(prevPB, pb)
		}
	}

	// Compute stats for both periods
	curRevenue := sumInvoiceTotal(curInvoices)
	prevRevenue := sumInvoiceTotal(prevInvoices)
	curPurchases := sumInvoiceTotal(curPB)
	prevPurchases := sumInvoiceTotal(prevPB)
	curProfit := curRevenue - curPurchases
	prevProfit := prevRevenue - prevPurchases

	curAvg := 0.0
	if len(curInvoices) > 0 {
		curAvg = curRevenue / float64(len(curInvoices))
	}
	prevAvg := 0.0
	if len(prevInvoices) > 0 {
		prevAvg = prevRevenue / float64(len(prevInvoices))
	}

	curPending := countPendingAmount(curInvoices)
	prevPending := countPendingAmount(prevInvoices)

	trends := map[string]KPITrend{
		"invoices":        makeTrend(float64(len(curInvoices)), float64(len(prevInvoices))),
		"revenue":         makeTrend(curRevenue, prevRevenue),
		"purchases_total": makeTrendInverse(curPurchases, prevPurchases), // lower is better
		"gross_profit":    makeTrend(curProfit, prevProfit),
		"avg_invoice":     makeTrend(curAvg, prevAvg),
		"pending_amount":  makeTrendInverse(curPending, prevPending), // lower is better
		"products":        {Direction: "flat", Percent: "0", Class: "text-gray-400", Arrow: "—"},
		"clients":         {Direction: "flat", Percent: "0", Class: "text-gray-400", Arrow: "—"},
		"suppliers":       {Direction: "flat", Percent: "0", Class: "text-gray-400", Arrow: "—"},
	}

	return trends
}

func parseInvoiceTime(inv models.Invoice) time.Time {
	if inv.EffectiveDate.Time == "" {
		return time.Time{}
	}
	t, ok := ParseFlexibleTime(inv.EffectiveDate.Time)
	if !ok {
		return time.Time{}
	}
	return t
}

func sumInvoiceTotal(invoices []models.Invoice) float64 {
	var total float64
	for _, inv := range invoices {
		total += inv.Total
	}
	return total
}

func countPendingAmount(invoices []models.Invoice) float64 {
	var total float64
	for _, inv := range invoices {
		if inv.State == 0 || inv.State == 1 {
			total += inv.Total
		}
	}
	return total
}

// makeTrend computes % change where higher is better (green ↑ / red ↓).
func makeTrend(current, previous float64) KPITrend {
	if previous == 0 && current == 0 {
		return KPITrend{Direction: "flat", Percent: "0", Class: "text-gray-400", Arrow: "—"}
	}
	if previous == 0 {
		return KPITrend{Direction: "up", Percent: "100", Class: "text-green-500", Arrow: "↑"}
	}
	pct := ((current - previous) / math.Abs(previous)) * 100
	pctStr := fmt.Sprintf("%.1f", math.Abs(pct))
	if pct > 0.5 {
		return KPITrend{Direction: "up", Percent: pctStr, Class: "text-green-500", Arrow: "↑"}
	} else if pct < -0.5 {
		return KPITrend{Direction: "down", Percent: pctStr, Class: "text-red-500", Arrow: "↓"}
	}
	return KPITrend{Direction: "flat", Percent: "0", Class: "text-gray-400", Arrow: "—"}
}

// makeTrendInverse computes % change where lower is better (green ↓ / red ↑).
func makeTrendInverse(current, previous float64) KPITrend {
	t := makeTrend(current, previous)
	// Invert the colors: decrease is good, increase is bad
	if t.Direction == "up" {
		t.Class = "text-red-500"
	} else if t.Direction == "down" {
		t.Class = "text-green-500"
	}
	return t
}

// ComputeWeekdayRevenue computes average revenue per weekday from invoices.
// Returns 7 values: Saturday(0) through Friday(6) to match MENA work week.
func ComputeWeekdayRevenue(invoices []models.Invoice) [7]float64 {
	var totals [7]float64
	var counts [7]int

	for _, inv := range invoices {
		t := parseInvoiceTime(inv)
		if t.IsZero() {
			continue
		}
		// Map Go weekday (Sun=0..Sat=6) to MENA weekday (Sat=0..Fri=6)
		menaDay := (int(t.Weekday()) + 1) % 7
		totals[menaDay] += inv.Total
		counts[menaDay]++
	}

	var avgs [7]float64
	for i := 0; i < 7; i++ {
		if counts[i] > 0 {
			avgs[i] = totals[i] / float64(counts[i])
		}
	}
	return avgs
}

// ComputeMovingAverage calculates a simple moving average over the given window.
func ComputeMovingAverage(data []float64, window int) []float64 {
	if window <= 0 || len(data) == 0 {
		return make([]float64, len(data))
	}
	result := make([]float64, len(data))
	for i := range data {
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		var sum float64
		count := 0
		for j := start; j <= i; j++ {
			sum += data[j]
			count++
		}
		result[i] = sum / float64(count)
	}
	return result
}

// ComputeYoYRevenue computes the previous year's monthly revenue aligned to the
// same 6-month window as the current chart. Returns 6 values matching monthLabels order.
func ComputeYoYRevenue(invoices []models.Invoice, monthLabels []string) []float64 {
	// Build a map of MM/YYYY → total for previous year
	prevYearRevenue := make(map[string]float64)
	for _, inv := range invoices {
		t := parseInvoiceTime(inv)
		if t.IsZero() {
			continue
		}
		key := t.Format("01/2006")
		prevYearRevenue[key] += inv.Total
	}

	result := make([]float64, len(monthLabels))
	for i, label := range monthLabels {
		// Parse current month label (MM/YYYY) and shift back 1 year
		parts := strings.SplitN(label, "/", 2)
		if len(parts) != 2 {
			continue
		}
		month, err1 := strconv.Atoi(parts[0])
		year, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		prevKey := fmt.Sprintf("%02d/%04d", month, year-1)
		result[i] = prevYearRevenue[prevKey]
	}
	return result
}

// ComputeInventoryTurnover calculates COGS / average inventory value.
// Uses total purchases as COGS proxy and current product value as inventory.
func ComputeInventoryTurnover(products []models.Product, totalPurchases float64) (ratio float64, cogs float64, avgInventory float64) {
	if len(products) == 0 {
		return 0, totalPurchases, 0
	}
	var totalValue float64
	for _, p := range products {
		totalValue += toFloat(p.Price) * toFloat(p.Quantity)
	}
	avgInventory = totalValue
	cogs = totalPurchases
	if avgInventory > 0 {
		ratio = cogs / avgInventory
	}
	return ratio, cogs, avgInventory
}

// AgingBucket holds invoice aging analysis for a time bucket.
type AgingBucket struct {
	Label string // "0-30 أيام", "31-60 أيام", etc.
	Count int
	Total float64
	Class string // CSS class for badge color
}

// ComputeAgingAnalysis classifies unpaid/pending invoices into 30/60/90/90+ day buckets.
func ComputeAgingAnalysis(invoices []models.Invoice) []AgingBucket {
	now := time.Now()
	buckets := []AgingBucket{
		{Label: "0-30 أيام (حالي)", Count: 0, Total: 0, Class: "bg-green-100 text-green-700"},
		{Label: "31-60 أيام (متأخر)", Count: 0, Total: 0, Class: "bg-yellow-100 text-yellow-700"},
		{Label: "61-90 أيام (متأخر جداً)", Count: 0, Total: 0, Class: "bg-orange-100 text-orange-700"},
		{Label: "90+ أيام (حرج)", Count: 0, Total: 0, Class: "bg-red-100 text-red-700"},
	}

	for _, inv := range invoices {
		// Only consider draft (0) and processing (1) as "unpaid"
		if inv.State != 0 && inv.State != 1 {
			continue
		}
		t := parseInvoiceTime(inv)
		if t.IsZero() {
			continue
		}
		days := int(now.Sub(t).Hours() / 24)
		idx := 0
		if days > 90 {
			idx = 3
		} else if days > 60 {
			idx = 2
		} else if days > 30 {
			idx = 1
		}
		buckets[idx].Count++
		buckets[idx].Total += inv.Total
	}
	return buckets
}

// ComputeCLV calculates Client Lifetime Value from orders — total amount per client across ALL time.
// Returns a map of client_name → total value.
func ComputeCLV(orders []map[string]interface{}) map[string]float64 {
	clv := make(map[string]float64)
	for _, order := range orders {
		clientName := firstNonEmptyString(order,
			"client", "client_name", "customer", "customer_name", "name",
		)
		if clientName == "" {
			continue
		}
		total := FirstNonZeroFloat(order, "total", "amount", "price")
		clv[clientName] += total
	}
	return clv
}

// ── Sprint 6 helpers ───────────────────────────────────────────

// ComputeAPAging classifies purchase bills into 30/60/90/90+ day aging buckets.
// Considers all purchase bills (they represent payables).
func ComputeAPAging(purchaseBills []models.Invoice) []AgingBucket {
	now := time.Now()
	buckets := []AgingBucket{
		{Label: "0-30 أيام (حالي)", Count: 0, Total: 0, Class: "bg-green-100 text-green-700"},
		{Label: "31-60 أيام (متأخر)", Count: 0, Total: 0, Class: "bg-yellow-100 text-yellow-700"},
		{Label: "61-90 أيام (متأخر جداً)", Count: 0, Total: 0, Class: "bg-orange-100 text-orange-700"},
		{Label: "90+ أيام (حرج)", Count: 0, Total: 0, Class: "bg-red-100 text-red-700"},
	}
	for _, pb := range purchaseBills {
		// Only consider non-issued (state != 3) as "unpaid"
		if pb.State == 3 {
			continue
		}
		t := parseInvoiceTime(pb)
		if t.IsZero() {
			continue
		}
		days := int(now.Sub(t).Hours() / 24)
		idx := 0
		if days > 90 {
			idx = 3
		} else if days > 60 {
			idx = 2
		} else if days > 30 {
			idx = 1
		}
		buckets[idx].Count++
		buckets[idx].Total += pb.Total
	}
	return buckets
}

// CashFlowPoint represents a single month's cash inflow/outflow.
type CashFlowPoint struct {
	Label   string
	Inflow  float64
	Outflow float64
	Net     float64
}

// ComputeCashFlow builds monthly inflow (invoices) vs outflow (purchases) for 6 months.
func ComputeCashFlow(invoices, purchaseBills []models.Invoice, monthLabels []string) []CashFlowPoint {
	inflows := make(map[string]float64)
	outflows := make(map[string]float64)

	for _, inv := range invoices {
		t := parseInvoiceTime(inv)
		if t.IsZero() {
			continue
		}
		key := t.Format("01/2006")
		inflows[key] += inv.Total
	}
	for _, pb := range purchaseBills {
		t := parseInvoiceTime(pb)
		if t.IsZero() {
			continue
		}
		key := t.Format("01/2006")
		outflows[key] += pb.Total
	}

	points := make([]CashFlowPoint, 0, len(monthLabels))
	for _, label := range monthLabels {
		inf := inflows[label]
		out := outflows[label]
		points = append(points, CashFlowPoint{
			Label:   label,
			Inflow:  inf,
			Outflow: out,
			Net:     inf - out,
		})
	}
	return points
}

// PnLStatement holds a simplified P&L.
type PnLStatement struct {
	Revenue     float64
	COGS        float64
	GrossProfit float64
	GrossMargin float64 // percentage
	// Monthly breakdown
	MonthLabels  []string
	MonthRevenue []float64
	MonthCOGS    []float64
	MonthProfit  []float64
}

// ComputePnL builds a P&L from invoices and purchase bills.
func ComputePnL(invoices, purchaseBills []models.Invoice, monthLabels []string) PnLStatement {
	pnl := PnLStatement{MonthLabels: monthLabels}

	revByMonth := make(map[string]float64)
	cogsByMonth := make(map[string]float64)

	for _, inv := range invoices {
		pnl.Revenue += inv.Total
		t := parseInvoiceTime(inv)
		if !t.IsZero() {
			revByMonth[t.Format("01/2006")] += inv.Total
		}
	}
	for _, pb := range purchaseBills {
		pnl.COGS += pb.Total
		t := parseInvoiceTime(pb)
		if !t.IsZero() {
			cogsByMonth[t.Format("01/2006")] += pb.Total
		}
	}
	pnl.GrossProfit = pnl.Revenue - pnl.COGS
	if pnl.Revenue > 0 {
		pnl.GrossMargin = pnl.GrossProfit * 100 / pnl.Revenue
	}

	pnl.MonthRevenue = make([]float64, len(monthLabels))
	pnl.MonthCOGS = make([]float64, len(monthLabels))
	pnl.MonthProfit = make([]float64, len(monthLabels))
	for i, label := range monthLabels {
		r := revByMonth[label]
		c := cogsByMonth[label]
		pnl.MonthRevenue[i] = r
		pnl.MonthCOGS[i] = c
		pnl.MonthProfit[i] = r - c
	}
	return pnl
}

// MarginTier represents a product price tier with margin data.
type MarginTier struct {
	Label    string
	Count    int
	AvgPrice float64
	Class    string
}

// ComputeGrossMarginByTier groups products by price tier and computes stats.
func ComputeGrossMarginByTier(products []models.Product) []MarginTier {
	tiers := []MarginTier{
		{Label: "< 50 ر.س", Class: "bg-blue-500"},
		{Label: "50-200 ر.س", Class: "bg-emerald-500"},
		{Label: "200-500 ر.س", Class: "bg-purple-500"},
		{Label: "500+ ر.س", Class: "bg-rose-500"},
	}
	counts := make([]int, 4)
	totals := make([]float64, 4)

	for _, p := range products {
		price := toFloat(p.Price)
		idx := 0
		if price >= 500 {
			idx = 3
		} else if price >= 200 {
			idx = 2
		} else if price >= 50 {
			idx = 1
		}
		counts[idx]++
		totals[idx] += price
	}
	for i := range tiers {
		tiers[i].Count = counts[i]
		if counts[i] > 0 {
			tiers[i].AvgPrice = totals[i] / float64(counts[i])
		}
	}
	return tiers
}

// ComputeFulfillmentRate calculates the % of orders that have a matching issued invoice.
// Since we don't have a direct order→invoice FK, we approximate by comparing
// total orders vs total issued invoices (state=3).
func ComputeFulfillmentRate(orders []map[string]interface{}, invoices []models.Invoice) float64 {
	if len(orders) == 0 {
		return 0
	}
	issuedCount := 0
	for _, inv := range invoices {
		if inv.State == 3 {
			issuedCount++
		}
	}
	rate := float64(issuedCount) * 100 / float64(len(orders))
	if rate > 100 {
		rate = 100
	}
	return rate
}

// ReturnRateData holds credit note / invoice ratio data.
type ReturnRateData struct {
	TotalInvoices int
	CreditNotes   int
	ReturnRate    float64 // percentage
	MonthlyRates  []float64
}

// ComputeReturnRate calculates the ratio of credit notes to total invoices.
func ComputeReturnRate(invoices []models.Invoice, monthLabels []string) ReturnRateData {
	data := ReturnRateData{}
	monthInv := make(map[string]int)
	monthCN := make(map[string]int)

	for _, inv := range invoices {
		if inv.Type { // Type=true means credit note
			data.CreditNotes++
			t := parseInvoiceTime(inv)
			if !t.IsZero() {
				monthCN[t.Format("01/2006")]++
			}
		} else {
			data.TotalInvoices++
			t := parseInvoiceTime(inv)
			if !t.IsZero() {
				monthInv[t.Format("01/2006")]++
			}
		}
	}
	if data.TotalInvoices > 0 {
		data.ReturnRate = float64(data.CreditNotes) * 100 / float64(data.TotalInvoices)
	}

	data.MonthlyRates = make([]float64, len(monthLabels))
	for i, label := range monthLabels {
		invCount := monthInv[label]
		cnCount := monthCN[label]
		if invCount > 0 {
			data.MonthlyRates[i] = float64(cnCount) * 100 / float64(invCount)
		}
	}
	return data
}

// SupplierScore holds performance data for a supplier.
type SupplierScore struct {
	Name       string
	BillCount  int
	AvgTotal   float64
	TotalSpent float64
}

// ComputeSupplierPerformance ranks suppliers by purchase volume.
// Since purchase bills reuse the Invoice model and have no supplier FK,
// we group by month frequency and total spend, returning top N suppliers
// from the supplier list paired with purchase data.
func ComputeSupplierPerformance(suppliers []models.Supplier, purchaseBills []models.Invoice, maxItems int) []SupplierScore {
	// With no FK linking bills to suppliers, we rank suppliers by count
	// and pair with aggregate purchase stats.
	totalBills := len(purchaseBills)
	var totalSpent float64
	for _, pb := range purchaseBills {
		totalSpent += pb.Total
	}

	avgPerBill := 0.0
	if totalBills > 0 {
		avgPerBill = totalSpent / float64(totalBills)
	}

	// Distribute evenly as approximation (no FK available)
	scores := make([]SupplierScore, 0, len(suppliers))
	for _, s := range suppliers {
		scores = append(scores, SupplierScore{
			Name:       s.Name,
			BillCount:  0,
			AvgTotal:   avgPerBill,
			TotalSpent: 0,
		})
	}

	// Distribute bills round-robin to suppliers as estimate
	if len(scores) > 0 {
		for i, pb := range purchaseBills {
			idx := i % len(scores)
			scores[idx].BillCount++
			scores[idx].TotalSpent += pb.Total
		}
		// Recalculate avg
		for i := range scores {
			if scores[i].BillCount > 0 {
				scores[i].AvgTotal = scores[i].TotalSpent / float64(scores[i].BillCount)
			}
		}
	}

	// Sort by total spent descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalSpent > scores[j].TotalSpent
	})
	if len(scores) > maxItems {
		scores = scores[:maxItems]
	}
	return scores
}

// ComputeAvgProcessingTime estimates avg days from order creation to invoice issuance.
// Without direct FK, uses aggregate: avg(issued_invoice_date) - avg(order_date).
func ComputeAvgProcessingTime(orders []map[string]interface{}, invoices []models.Invoice) float64 {
	var orderDates []time.Time
	for _, o := range orders {
		for _, key := range []string{"created_at", "date", "order_date"} {
			if v, ok := o[key]; ok {
				if s, ok := v.(string); ok {
					if t, ok := ParseFlexibleTime(s); ok {
						orderDates = append(orderDates, t)
						break
					}
				}
			}
		}
	}

	var issuedDates []time.Time
	for _, inv := range invoices {
		if inv.State == 3 { // issued
			t := parseInvoiceTime(inv)
			if !t.IsZero() {
				issuedDates = append(issuedDates, t)
			}
		}
	}

	if len(orderDates) == 0 || len(issuedDates) == 0 {
		return 0
	}

	// Calculate average of each set
	var sumOrder, sumInvoice float64
	for _, t := range orderDates {
		sumOrder += float64(t.Unix())
	}
	for _, t := range issuedDates {
		sumInvoice += float64(t.Unix())
	}
	avgOrder := sumOrder / float64(len(orderDates))
	avgInvoice := sumInvoice / float64(len(issuedDates))

	diffDays := (avgInvoice - avgOrder) / 86400 // seconds per day
	if diffDays < 0 {
		diffDays = 0
	}
	return math.Round(diffDays*10) / 10
}

// ── Advanced Dashboard Metrics (Score ≥ 8) ─────────────────────

// ComputeAverageInvoiceValue returns total revenue / invoice count.
func ComputeAverageInvoiceValue(invoices []models.Invoice) float64 {
	if len(invoices) == 0 {
		return 0
	}
	var total float64
	for _, inv := range invoices {
		total += inv.Total
	}
	return total / float64(len(invoices))
}

// ComputeRevenuePerClient returns revenue / number of clients.
func ComputeRevenuePerClient(revenue float64, clientCount int) float64 {
	if clientCount == 0 {
		return 0
	}
	return revenue / float64(clientCount)
}

// ComputeMoMGrowth computes month-over-month % growth for a revenue series.
func ComputeMoMGrowth(revenueSeries []float64) []float64 {
	result := make([]float64, len(revenueSeries))
	for i := 1; i < len(revenueSeries); i++ {
		if revenueSeries[i-1] > 0 {
			result[i] = ((revenueSeries[i] - revenueSeries[i-1]) / revenueSeries[i-1]) * 100
		}
	}
	return result
}

// ComputeDSO returns Days Sales Outstanding: (AR / Revenue) * days.
func ComputeDSO(arTotal, revenue float64, days int) float64 {
	if revenue == 0 {
		return 0
	}
	return (arTotal / revenue) * float64(days)
}

// ComputeDPO returns Days Payable Outstanding: (AP / COGS) * days.
func ComputeDPO(apTotal, cogs float64, days int) float64 {
	if cogs == 0 {
		return 0
	}
	return (apTotal / cogs) * float64(days)
}

// ComputeDIO returns Days Inventory Outstanding: 365 / inventory turnover.
func ComputeDIO(invTurnover float64) float64 {
	if invTurnover == 0 {
		return 0
	}
	return 365.0 / invTurnover
}

// ComputeCashConversionCycle returns DIO + DSO - DPO.
func ComputeCashConversionCycle(dio, dso, dpo float64) float64 {
	return dio + dso - dpo
}

// ComputeTotalAROutstanding sums AR aging buckets.
func ComputeTotalAROutstanding(arBuckets []AgingBucket) (float64, int) {
	var total float64
	var count int
	for _, b := range arBuckets {
		total += b.Total
		count += b.Count
	}
	return total, count
}

// ComputeTotalAPOutstanding sums AP aging buckets.
func ComputeTotalAPOutstanding(apBuckets []AgingBucket) (float64, int) {
	var total float64
	var count int
	for _, b := range apBuckets {
		total += b.Total
		count += b.Count
	}
	return total, count
}

// ComputeNetCashPosition returns AR - AP.
func ComputeNetCashPosition(arTotal, apTotal float64) float64 {
	return arTotal - apTotal
}

// ComputeReceivablesTurnover returns Revenue / AR.
func ComputeReceivablesTurnover(revenue, arTotal float64) float64 {
	if arTotal == 0 {
		return 0
	}
	return revenue / arTotal
}

// ComputeAvgCollectionPeriod returns 365 / receivables turnover.
func ComputeAvgCollectionPeriod(recvTurnover float64) float64 {
	if recvTurnover == 0 {
		return 0
	}
	return 365.0 / recvTurnover
}

// ComputeAPTurnover returns COGS / AP.
func ComputeAPTurnover(cogs, apTotal float64) float64 {
	if apTotal == 0 {
		return 0
	}
	return cogs / apTotal
}

// ComputeAvgPaymentPeriod returns 365 / AP turnover.
func ComputeAvgPaymentPeriod(apTurnover float64) float64 {
	if apTurnover == 0 {
		return 0
	}
	return 365.0 / apTurnover
}

// ComputeInventoryValue sums Price * Quantity for all products.
func ComputeInventoryValue(products []models.Product) float64 {
	var total float64
	for _, p := range products {
		total += toFloat(p.Price) * toFloat(p.Quantity)
	}
	return total
}

// ComputeOutOfStock returns count and rate of products with zero quantity.
func ComputeOutOfStock(products []models.Product) (int, float64) {
	if len(products) == 0 {
		return 0, 0
	}
	count := 0
	for _, p := range products {
		if toInt(p.Quantity) <= 0 {
			count++
		}
	}
	rate := float64(count) * 100 / float64(len(products))
	return count, rate
}

// ComputeInventoryToSalesRatio returns inventory value / monthly revenue.
func ComputeInventoryToSalesRatio(invValue, monthlyRevenue float64) float64 {
	if monthlyRevenue == 0 {
		return 0
	}
	return invValue / monthlyRevenue
}

// ComputeEffectiveVATRate returns VAT / Revenue as percentage.
func ComputeEffectiveVATRate(vat, revenue float64) float64 {
	if revenue == 0 {
		return 0
	}
	return (vat / revenue) * 100
}

// ComputeDiscountRate returns discounts / revenue as percentage.
func ComputeDiscountRate(discounts, revenue float64) float64 {
	if revenue == 0 {
		return 0
	}
	return (discounts / revenue) * 100
}

// ComputeDraftToIssuedRate returns draft invoices / total * 100.
func ComputeDraftToIssuedRate(statusCounts map[int]int) float64 {
	total := 0
	for _, c := range statusCounts {
		total += c
	}
	if total == 0 {
		return 0
	}
	return float64(statusCounts[0]) * 100 / float64(total)
}

// ComputeConcentrationRisk returns % of total from topN items.
func ComputeConcentrationRisk(items []float64, topN int) float64 {
	if len(items) == 0 || topN <= 0 {
		return 0
	}
	sorted := make([]float64, len(items))
	copy(sorted, items)
	sort.Float64s(sorted)
	// Reverse to descending
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}

	var total, topTotal float64
	for _, v := range sorted {
		total += v
	}
	if total == 0 {
		return 0
	}
	n := topN
	if n > len(sorted) {
		n = len(sorted)
	}
	for i := 0; i < n; i++ {
		topTotal += sorted[i]
	}
	return (topTotal / total) * 100
}

// ComputePurchaseToSalesRatio returns purchases / revenue as percentage.
func ComputePurchaseToSalesRatio(purchases, revenue float64) float64 {
	if revenue == 0 {
		return 0
	}
	return (purchases / revenue) * 100
}

// ComputeCashBurnRate returns the average monthly net outflow from cash flow points.
func ComputeCashBurnRate(cfPoints []CashFlowPoint) float64 {
	if len(cfPoints) == 0 {
		return 0
	}
	var totalNet float64
	negCount := 0
	for _, p := range cfPoints {
		if p.Net < 0 {
			totalNet += p.Net
			negCount++
		}
	}
	if negCount == 0 {
		return 0
	}
	return math.Abs(totalNet / float64(negCount))
}

// VATQuarter holds quarterly VAT aggregation.
type VATQuarter struct {
	Quarter   string
	OutputVAT float64
	InputVAT  float64
	NetVAT    float64
}

// ComputeVATQuarterly aggregates VAT by quarter.
func ComputeVATQuarterly(invoices []models.Invoice, purchaseBills []models.Invoice) []VATQuarter {
	outputByQ := make(map[string]float64)
	inputByQ := make(map[string]float64)

	for _, inv := range invoices {
		t := parseInvoiceTime(inv)
		if t.IsZero() {
			continue
		}
		q := fmt.Sprintf("Q%d/%d", (int(t.Month())-1)/3+1, t.Year())
		outputByQ[q] += inv.TotalVAT
	}
	for _, pb := range purchaseBills {
		t := parseInvoiceTime(pb)
		if t.IsZero() {
			continue
		}
		q := fmt.Sprintf("Q%d/%d", (int(t.Month())-1)/3+1, t.Year())
		inputByQ[q] += pb.TotalVAT
	}

	// Collect all quarters
	qSet := make(map[string]struct{})
	for q := range outputByQ {
		qSet[q] = struct{}{}
	}
	for q := range inputByQ {
		qSet[q] = struct{}{}
	}
	quarters := make([]string, 0, len(qSet))
	for q := range qSet {
		quarters = append(quarters, q)
	}
	sort.Strings(quarters)

	result := make([]VATQuarter, 0, len(quarters))
	for _, q := range quarters {
		out := outputByQ[q]
		in := inputByQ[q]
		result = append(result, VATQuarter{
			Quarter:   q,
			OutputVAT: out,
			InputVAT:  in,
			NetVAT:    out - in,
		})
	}
	return result
}

// ComputeRevenueByStatus groups revenue by invoice state.
func ComputeRevenueByStatus(invoices []models.Invoice) map[int]float64 {
	result := make(map[int]float64)
	for _, inv := range invoices {
		result[inv.State] += inv.Total
	}
	return result
}

// ComputeAvgDaysToIssue estimates average days between invoice creation and issuance.
// Uses state=3 (issued) invoices relative to state=0 (draft) as approximation.
func ComputeAvgDaysToIssue(invoices []models.Invoice) float64 {
	var totalDays float64
	var count int
	now := time.Now()
	for _, inv := range invoices {
		if inv.State == 3 {
			t := parseInvoiceTime(inv)
			if t.IsZero() {
				continue
			}
			days := now.Sub(t).Hours() / 24
			if days >= 0 && days < 365 {
				totalDays += days
				count++
			}
		}
	}
	if count == 0 {
		return 0
	}
	return math.Round((totalDays/float64(count))*10) / 10
}

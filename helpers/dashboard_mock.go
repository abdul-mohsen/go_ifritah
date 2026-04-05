package helpers

// ── Mock data for dashboard metrics that require backend data not yet available ──
// All mocks include IsMocked flag so the UI can show a "بيانات تجريبية" badge.

// MockBalanceSheet holds simulated balance sheet data.
type MockBalanceSheet struct {
	IsMocked           bool
	CurrentAssets      float64
	CurrentLiabilities float64
	TotalAssets        float64
	TotalDebt          float64
	Equity             float64
	Cash               float64
}

// GetMockBalanceSheet returns a simulated balance sheet scaled to revenue.
func GetMockBalanceSheet(revenue, totalPurchases float64) MockBalanceSheet {
	return MockBalanceSheet{
		IsMocked:           true,
		CurrentAssets:      revenue * 0.45,
		CurrentLiabilities: totalPurchases * 0.35,
		TotalAssets:        revenue * 0.80,
		TotalDebt:          totalPurchases * 0.25,
		Equity:             revenue*0.80 - totalPurchases*0.25,
		Cash:               revenue * 0.15,
	}
}

// MockOperatingExpenses holds simulated OpEx data.
type MockOperatingExpenses struct {
	IsMocked    bool
	Rent        float64
	Salaries    float64
	Utilities   float64
	Marketing   float64
	Other       float64
	TotalOpEx   float64
	NetIncome   float64
	OpExRatio   float64 // OpEx / Revenue %
}

// GetMockOperatingExpenses returns simulated OpEx scaled to revenue.
func GetMockOperatingExpenses(revenue, grossProfit float64) MockOperatingExpenses {
	rent := revenue * 0.05
	salaries := revenue * 0.15
	utilities := revenue * 0.02
	marketing := revenue * 0.03
	other := revenue * 0.05
	totalOpEx := rent + salaries + utilities + marketing + other
	netIncome := grossProfit - totalOpEx
	opExRatio := 0.0
	if revenue > 0 {
		opExRatio = (totalOpEx / revenue) * 100
	}
	return MockOperatingExpenses{
		IsMocked:  true,
		Rent:      rent,
		Salaries:  salaries,
		Utilities: utilities,
		Marketing: marketing,
		Other:     other,
		TotalOpEx: totalOpEx,
		NetIncome: netIncome,
		OpExRatio: opExRatio,
	}
}

// MockZATCACompliance holds simulated ZATCA e-invoicing compliance data.
type MockZATCACompliance struct {
	IsMocked          bool
	UUIDCount         int
	QRCodeCount       int
	SubmittedCount    int
	RejectedCount     int
	ComplianceRate    float64
	LastSubmissionDate string
}

// GetMockZATCACompliance returns simulated ZATCA compliance data.
func GetMockZATCACompliance(invoiceCount int) MockZATCACompliance {
	submitted := int(float64(invoiceCount) * 0.92)
	rejected := int(float64(invoiceCount) * 0.02)
	compRate := 0.0
	if invoiceCount > 0 {
		compRate = float64(submitted) * 100 / float64(invoiceCount)
	}
	return MockZATCACompliance{
		IsMocked:          true,
		UUIDCount:         invoiceCount,
		QRCodeCount:       invoiceCount,
		SubmittedCount:    submitted,
		RejectedCount:     rejected,
		ComplianceRate:    compRate,
		LastSubmissionDate: "2026-02-01",
	}
}

// MockPaymentTracking holds simulated payment collection data.
type MockPaymentTracking struct {
	IsMocked             bool
	CollectionEfficiency float64 // % of invoices paid on time
	OnTimeRate           float64
	OverdueCount         int
	OverdueAmount        float64
	AvgDaysToPay         float64
}

// GetMockPaymentTracking returns simulated payment tracking data.
func GetMockPaymentTracking(invoiceCount int, revenue float64) MockPaymentTracking {
	overdueCount := int(float64(invoiceCount) * 0.15)
	return MockPaymentTracking{
		IsMocked:             true,
		CollectionEfficiency: 78.5,
		OnTimeRate:           72.3,
		OverdueCount:         overdueCount,
		OverdueAmount:        revenue * 0.12,
		AvgDaysToPay:         23.5,
	}
}

// MockLiquidityRatios holds simulated liquidity metrics.
type MockLiquidityRatios struct {
	IsMocked     bool
	CurrentRatio float64 // CurrentAssets / CurrentLiabilities
	QuickRatio   float64 // (CurrentAssets - Inventory) / CurrentLiabilities
	DebtToEquity float64 // TotalDebt / Equity
}

// GetMockLiquidityRatios returns simulated liquidity ratios from balance sheet.
func GetMockLiquidityRatios(bs MockBalanceSheet, inventoryValue float64) MockLiquidityRatios {
	currentRatio := 0.0
	quickRatio := 0.0
	debtToEquity := 0.0
	if bs.CurrentLiabilities > 0 {
		currentRatio = bs.CurrentAssets / bs.CurrentLiabilities
		quickRatio = (bs.CurrentAssets - inventoryValue) / bs.CurrentLiabilities
	}
	if bs.Equity > 0 {
		debtToEquity = bs.TotalDebt / bs.Equity
	}
	return MockLiquidityRatios{
		IsMocked:     true,
		CurrentRatio: currentRatio,
		QuickRatio:   quickRatio,
		DebtToEquity: debtToEquity,
	}
}

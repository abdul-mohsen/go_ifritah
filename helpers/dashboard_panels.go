package helpers

// Dashboard panel structs populated from the backend
// /api/v2/dashboard/analytics endpoint and bound directly by templates.

type DashboardBalanceSheet struct {
	CurrentAssets      float64
	CurrentLiabilities float64
	TotalAssets        float64
	TotalDebt          float64
	Equity             float64
	Cash               float64
}

type DashboardOperatingExpenses struct {
	Rent      float64
	Salaries  float64
	Utilities float64
	Marketing float64
	Other     float64
	TotalOpEx float64
	NetIncome float64
	OpExRatio float64 // OpEx / Revenue %
}

type DashboardZATCACompliance struct {
	UUIDCount          int
	QRCodeCount        int
	SubmittedCount     int
	RejectedCount      int
	ComplianceRate     float64
	LastSubmissionDate string
}

type DashboardPaymentTrackingPanel struct {
	CollectionEfficiency float64 // % of invoices paid on time
	OnTimeRate           float64
	OverdueCount         int
	OverdueAmount        float64
	AvgDaysToPay         float64
}

type DashboardLiquidityRatios struct {
	CurrentRatio float64 // CurrentAssets / CurrentLiabilities
	QuickRatio   float64 // (CurrentAssets - Inventory) / CurrentLiabilities
	DebtToEquity float64 // TotalDebt / Equity
}

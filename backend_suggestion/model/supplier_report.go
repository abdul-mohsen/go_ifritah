// Package model — typed response structs for the supplier account report.
//
// File destination in backend repo: pkg/model/supplier_report.go
//
// The JSON shape matches what the afrita-go frontend already consumes in
// helpers.FetchSupplierReport. Keep these keys stable so the frontend can
// render the page and build Excel/PDF without extra backend requests.

package model

type SupplierReportResponse struct {
	Summary          SupplierReportSummary          `json:"summary"`
	Bills            []SupplierReportBill           `json:"bills"`
	Payments         []SupplierReportPayment        `json:"payments"`
	TopItems         []SupplierReportTopItem        `json:"top_items"`
	Aging            []SupplierReportAgingBucket    `json:"aging"`
	MonthlySpending  []SupplierReportMonthlySpend   `json:"monthly_spending"`
	PaymentBreakdown SupplierReportPaymentBreakdown `json:"payment_breakdown"`
}

type SupplierReportSummary struct {
	BillCount      int    `json:"bill_count"`
	TotalSpent     string `json:"total_spent"`
	TotalBeforeVAT string `json:"total_before_vat"`
	TotalVAT       string `json:"total_vat"`
	TotalDiscount  string `json:"total_discount"`
	TotalPayments  string `json:"total_payments"`
	PaymentCount   int    `json:"payment_count"`
	PaidTotal      string `json:"paid_total"`
	UnpaidTotal    string `json:"unpaid_total"`
	ClosingBalance string `json:"closing_balance"`
	AvgBill        string `json:"avg_bill"`
	ReceivedCount  int    `json:"received_count"`
}

type SupplierReportBill struct {
	ID                     int    `json:"id"`
	SequenceNumber         int    `json:"sequence_number"`
	SupplierSequenceNumber string `json:"supplier_sequence_number"`
	Total                  string `json:"total"`
	TotalBeforeVAT         string `json:"total_before_vat"`
	TotalVAT               string `json:"total_vat"`
	Discount               string `json:"discount"`
	State                  int    `json:"state"`
	EffectiveDate          string `json:"effective_date"`
	PaymentDueDate         string `json:"payment_due_date"`
	ReceivedAt             string `json:"received_at"`
	ReceivedBy             string `json:"received_by"`
	ItemCount              int    `json:"item_count"`
}

type SupplierReportPayment struct {
	ID            int    `json:"id"`
	VoucherNumber int    `json:"voucher_number"`
	VoucherType   string `json:"voucher_type"`
	EffectiveDate string `json:"effective_date"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Description   string `json:"description"`
}

type SupplierReportTopItem struct {
	ItemName   string `json:"item_name"`
	TotalQty   string `json:"total_qty"`
	TotalValue string `json:"total_value"`
	AvgPrice   string `json:"avg_price"`
	BillCount  int    `json:"bill_count"`
}

type SupplierReportAgingBucket struct {
	Bucket      string `json:"bucket"`
	BillCount   int    `json:"bill_count"`
	BucketTotal string `json:"bucket_total"`
}

type SupplierReportMonthlySpend struct {
	Month      string `json:"month"`
	TotalSpent string `json:"total_spent"`
}

type SupplierReportPaymentBreakdown struct {
	CashTotal         string `json:"cash_total"`
	BankTransferTotal string `json:"bank_transfer_total"`
}

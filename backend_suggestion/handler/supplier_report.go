// Package handlers — supplier account report aggregate endpoint.
//
// File destination in backend repo: pkg/handlers/supplier_report.go
//
// This replaces the frontend legacy report fallback that currently performs:
//   1. POST /api/v2/purchase_bill/all
//   2. GET /api/v2/purchase_bill/{id} for every purchase bill
//   3. POST /api/v2/cash_voucher/all
//
// Wire this route where supplier routes are registered:
//
//     apiV2.GET("/supplier/:id/report", h.GetSupplierReport)

package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
)

// GET /api/v2/supplier/:id/report?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *handler) GetSupplierReport(c *gin.Context) {
	supplierID, err := strconv.Atoi(c.Param("id"))
	if err != nil || supplierID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid supplier id"})
		return
	}

	fromDate, err := supplierReportDateParam(c.Query("from"), false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date; expected YYYY-MM-DD"})
		return
	}
	toDate, err := supplierReportDateParam(c.Query("to"), true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date; expected YYYY-MM-DD"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	params := db.GetSupplierReportSummaryParams{
		SupplierID: int32(supplierID),
		FromDate:   fromDate,
		ToDate:     toDate,
	}

	var (
		summary          db.GetSupplierReportSummaryRow
		bills            []db.ListSupplierReportBillsRow
		payments         []db.ListSupplierReportPaymentsRow
		topItems         []db.ListSupplierReportTopItemsRow
		aging            []db.ListSupplierReportAgingRow
		monthly          []db.ListSupplierReportMonthlySpendingRow
		paymentBreakdown db.GetSupplierReportPaymentBreakdownRow

		firstErr error
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			setErr(fn())
		}()
	}

	q := h.queries
	run(func() error { var err error; summary, err = q.GetSupplierReportSummary(ctx, params); return err })
	run(func() error {
		var err error
		bills, err = q.ListSupplierReportBills(ctx, db.ListSupplierReportBillsParams(params))
		return err
	})
	run(func() error {
		var err error
		payments, err = q.ListSupplierReportPayments(ctx, db.ListSupplierReportPaymentsParams(params))
		return err
	})
	run(func() error {
		var err error
		topItems, err = q.ListSupplierReportTopItems(ctx, db.ListSupplierReportTopItemsParams(params))
		return err
	})
	run(func() error {
		var err error
		aging, err = q.ListSupplierReportAging(ctx, db.ListSupplierReportAgingParams(params))
		return err
	})
	run(func() error {
		var err error
		monthly, err = q.ListSupplierReportMonthlySpending(ctx, db.ListSupplierReportMonthlySpendingParams(params))
		return err
	})
	run(func() error {
		var err error
		paymentBreakdown, err = q.GetSupplierReportPaymentBreakdown(ctx, db.GetSupplierReportPaymentBreakdownParams(params))
		return err
	})
	wg.Wait()

	if firstErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": firstErr.Error()})
		return
	}

	resp := model.SupplierReportResponse{
		Summary: model.SupplierReportSummary{
			BillCount:      supplierReportInt(summary.BillCount),
			TotalSpent:     supplierReportMoney(summary.TotalSpent),
			TotalBeforeVAT: supplierReportMoney(summary.TotalBeforeVat),
			TotalVAT:       supplierReportMoney(summary.TotalVat),
			TotalDiscount:  supplierReportMoney(summary.TotalDiscount),
			TotalPayments:  supplierReportMoney(summary.TotalPayments),
			PaymentCount:   supplierReportInt(summary.PaymentCount),
			PaidTotal:      supplierReportMoney(summary.PaidTotal),
			UnpaidTotal:    supplierReportMoney(summary.UnpaidTotal),
			ClosingBalance: supplierReportMoney(summary.ClosingBalance),
			AvgBill:        supplierReportMoney(summary.AvgBill),
			ReceivedCount:  supplierReportInt(summary.ReceivedCount),
		},
		Bills:            make([]model.SupplierReportBill, 0, len(bills)),
		Payments:         make([]model.SupplierReportPayment, 0, len(payments)),
		TopItems:         make([]model.SupplierReportTopItem, 0, len(topItems)),
		Aging:            normalizeSupplierReportAging(aging),
		MonthlySpending:  make([]model.SupplierReportMonthlySpend, 0, len(monthly)),
		PaymentBreakdown: model.SupplierReportPaymentBreakdown{CashTotal: supplierReportMoney(paymentBreakdown.CashTotal), BankTransferTotal: supplierReportMoney(paymentBreakdown.BankTransferTotal)},
	}

	for _, r := range bills {
		resp.Bills = append(resp.Bills, model.SupplierReportBill{
			ID:                     supplierReportInt(r.ID),
			SequenceNumber:         supplierReportInt(r.SequenceNumber),
			SupplierSequenceNumber: supplierReportString(r.SupplierSequenceNumber),
			Total:                  supplierReportMoney(r.Total),
			TotalBeforeVAT:         supplierReportMoney(r.TotalBeforeVat),
			TotalVAT:               supplierReportMoney(r.TotalVat),
			Discount:               supplierReportMoney(r.Discount),
			State:                  supplierReportInt(r.State),
			EffectiveDate:          supplierReportString(r.EffectiveDate),
			PaymentDueDate:         supplierReportString(r.PaymentDueDate),
			ReceivedAt:             supplierReportString(r.ReceivedAt),
			ReceivedBy:             supplierReportString(r.ReceivedBy),
			ItemCount:              supplierReportInt(r.ItemCount),
		})
	}

	for _, r := range payments {
		resp.Payments = append(resp.Payments, model.SupplierReportPayment{
			ID:            supplierReportInt(r.ID),
			VoucherNumber: supplierReportInt(r.VoucherNumber),
			VoucherType:   supplierReportString(r.VoucherType),
			EffectiveDate: supplierReportString(r.EffectiveDate),
			Amount:        supplierReportMoney(r.Amount),
			PaymentMethod: supplierReportString(r.PaymentMethod),
			Description:   supplierReportString(r.Description),
		})
	}

	for _, r := range topItems {
		resp.TopItems = append(resp.TopItems, model.SupplierReportTopItem{
			ItemName:   supplierReportString(r.ItemName),
			TotalQty:   supplierReportQuantity(r.TotalQty),
			TotalValue: supplierReportMoney(r.TotalValue),
			AvgPrice:   supplierReportMoney(r.AvgPrice),
			BillCount:  supplierReportInt(r.BillCount),
		})
	}

	for _, r := range monthly {
		resp.MonthlySpending = append(resp.MonthlySpending, model.SupplierReportMonthlySpend{
			Month:      supplierReportString(r.Month),
			TotalSpent: supplierReportMoney(r.TotalSpent),
		})
	}

	c.JSON(http.StatusOK, resp)
}

func supplierReportDateParam(value string, endOfDay bool) (sql.NullTime, error) {
	if value == "" {
		return sql.NullTime{}, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return sql.NullTime{}, err
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return sql.NullTime{Time: parsed, Valid: true}, nil
}

func normalizeSupplierReportAging(rows []db.ListSupplierReportAgingRow) []model.SupplierReportAgingBucket {
	order := []string{"current", "1-30", "31-60", "61-90", "90+"}
	byBucket := make(map[string]db.ListSupplierReportAgingRow, len(rows))
	for _, row := range rows {
		byBucket[supplierReportString(row.Bucket)] = row
	}

	out := make([]model.SupplierReportAgingBucket, 0, len(order))
	for _, bucket := range order {
		row := byBucket[bucket]
		out = append(out, model.SupplierReportAgingBucket{
			Bucket:      bucket,
			BillCount:   supplierReportInt(row.BillCount),
			BucketTotal: supplierReportMoney(row.BucketTotal),
		})
	}
	return out
}

func supplierReportMoney(value any) string {
	return fmt.Sprintf("%.2f", supplierReportFloat(value))
}

func supplierReportQuantity(value any) string {
	return fmt.Sprintf("%.3f", supplierReportFloat(value))
}

func supplierReportFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case []byte:
		f, _ := strconv.ParseFloat(string(v), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case sql.NullString:
		if !v.Valid {
			return 0
		}
		f, _ := strconv.ParseFloat(v.String, 64)
		return f
	case sql.NullFloat64:
		if !v.Valid {
			return 0
		}
		return v.Float64
	case sql.NullInt64:
		if !v.Valid {
			return 0
		}
		return float64(v.Int64)
	default:
		return 0
	}
}

func supplierReportInt(value any) int {
	return int(supplierReportFloat(value))
}

func supplierReportString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return ""
	case sql.NullTime:
		if v.Valid {
			return v.Time.Format(time.RFC3339)
		}
		return ""
	case time.Time:
		return v.Format(time.RFC3339)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

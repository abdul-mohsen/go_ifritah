package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"afrita/helpers"
	"afrita/models"
)

// supplierStatementEntry pairs one supplier with its fetched report, used
// to build a combined multi-supplier ledger statement.
type supplierStatementEntry struct {
	Supplier models.Supplier
	Report   helpers.SupplierReportResult
}

// supplierStatementTotals holds the totals aggregated across every
// supplier included in a multi-supplier ledger statement.
type supplierStatementTotals struct {
	SupplierCount  int
	TotalSpent     float64
	TotalPayments  float64
	ClosingBalance float64
}

// parseSupplierIDs parses a comma-separated list of supplier IDs (as
// produced by the checkbox picker on the suppliers list page), skipping
// invalid entries and de-duplicating.
func parseSupplierIDs(raw string) []int {
	var ids []int
	seen := make(map[int]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		ids = append(ids, n)
	}
	return ids
}

// loadSupplierStatementEntries fetches the supplier + report for each ID.
// Suppliers that can no longer be found (e.g. deleted after the checkbox
// was ticked) are skipped rather than failing the whole statement.
func loadSupplierStatementEntries(token string, ids []int, dateFrom, dateTo string) ([]supplierStatementEntry, error) {
	entries := make([]supplierStatementEntry, 0, len(ids))
	for _, id := range ids {
		supplier, found := findSupplierByID(token, strconv.Itoa(id))
		if !found {
			continue
		}
		report, err := helpers.FetchSupplierReport(token, id, dateFrom, dateTo)
		if err != nil {
			return nil, err
		}
		applySupplierCreditUtilization(&report, supplier)
		entries = append(entries, supplierStatementEntry{Supplier: supplier, Report: report})
	}
	return entries, nil
}

func aggregateSupplierStatementTotals(entries []supplierStatementEntry) supplierStatementTotals {
	totals := supplierStatementTotals{SupplierCount: len(entries)}
	for _, entry := range entries {
		totals.TotalSpent += entry.Report.Summary.TotalSpent
		totals.TotalPayments += entry.Report.Summary.TotalPayments
		totals.ClosingBalance += entry.Report.Summary.ClosingBalance
	}
	return totals
}

// loadSupplierStatementForDownload shares the request-parsing + fetch logic
// between the page handler and the three export handlers.
func loadSupplierStatementForDownload(w http.ResponseWriter, r *http.Request) ([]supplierStatementEntry, string, string, bool) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return nil, "", "", false
	}

	ids := parseSupplierIDs(r.URL.Query().Get("ids"))
	if len(ids) == 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "يرجى اختيار مورد واحد على الأقل")
		return nil, "", "", false
	}

	dateFrom, dateTo := supplierReportDateRange(r)
	entries, err := loadSupplierStatementEntries(token, ids, dateFrom, dateTo)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, msgSupplierReportFailed)
		return nil, "", "", false
	}
	if len(entries) == 0 {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, msgSupplierNotFound)
		return nil, "", "", false
	}

	return entries, dateFrom, dateTo, true
}

// HandleSupplierStatement renders the combined ledger statement for one or
// more suppliers selected via checkboxes on the suppliers list page.
// GET /dashboard/suppliers/statement?ids=1,2,3&from=&to=
func HandleSupplierStatement(w http.ResponseWriter, r *http.Request) {
	entries, dateFrom, dateTo, ok := loadSupplierStatementForDownload(w, r)
	if !ok {
		return
	}

	helpers.Render(w, r, "supplier-statement", map[string]interface{}{
		"title":     "كشف حساب موردين",
		"entries":   entries,
		"totals":    aggregateSupplierStatementTotals(entries),
		"date_from": dateFrom,
		"date_to":   dateTo,
		"ids_param": r.URL.Query().Get("ids"),
	})
}

// HandleExportSupplierStatementCSV exports the combined statement as CSV,
// with an extra "Supplier" column identifying which supplier each ledger
// row belongs to.
// GET /dashboard/suppliers/statement/export-csv?ids=1,2,3&from=&to=
func HandleExportSupplierStatementCSV(w http.ResponseWriter, r *http.Request) {
	entries, dateFrom, dateTo, ok := loadSupplierStatementForDownload(w, r)
	if !ok {
		return
	}

	filename := fmt.Sprintf("supplier_statement_%s_%s.csv", dateFrom, dateTo)
	w.Header().Set(headerContentType, "text/csv; charset=utf-8")
	w.Header().Set(headerContentDisp, "attachment; filename="+filename)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"المورد", "رقم الفاتورة", "التاريخ", "النوع", "المرجع", "الوصف", "مدين", "دائن", "الرصيد"})
	for _, entry := range entries {
		for _, ledgerEntry := range entry.Report.Ledger {
			_ = writer.Write([]string{
				entry.Supplier.Name,
				ledgerBillNo(ledgerEntry),
				ledgerEntry.Date,
				ledgerTypeName(ledgerEntry),
				ledgerEntry.Reference,
				ledgerEntry.Description,
				fmt.Sprintf("%.2f", ledgerEntry.Debit),
				fmt.Sprintf("%.2f", ledgerEntry.Credit),
				fmt.Sprintf("%.2f", ledgerEntry.Balance),
			})
		}
	}
}

// HandleExportSupplierStatementExcel exports the combined statement as an
// Excel-openable HTML table, one section per supplier - same technique as
// the existing single-supplier export.
// GET /dashboard/suppliers/statement/export-excel?ids=1,2,3&from=&to=
func HandleExportSupplierStatementExcel(w http.ResponseWriter, r *http.Request) {
	entries, dateFrom, dateTo, ok := loadSupplierStatementForDownload(w, r)
	if !ok {
		return
	}

	filename := fmt.Sprintf("supplier_statement_%s_%s.xls", dateFrom, dateTo)
	w.Header().Set(headerContentType, "application/vnd.ms-excel; charset=utf-8")
	w.Header().Set(headerContentDisp, "attachment; filename="+filename)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writeMultiSupplierReportDocument(w, entries, dateFrom, dateTo, true)
}

// HandleExportSupplierStatementPDF exports the combined statement as a
// print-ready HTML page (browser "Print / Save as PDF"), one section per
// supplier - same technique as the existing single-supplier export.
// GET /dashboard/suppliers/statement/export-pdf?ids=1,2,3&from=&to=
func HandleExportSupplierStatementPDF(w http.ResponseWriter, r *http.Request) {
	entries, dateFrom, dateTo, ok := loadSupplierStatementForDownload(w, r)
	if !ok {
		return
	}

	filename := fmt.Sprintf("supplier_statement_%s_%s.html", dateFrom, dateTo)
	w.Header().Set(headerContentType, "text/html; charset=utf-8")
	w.Header().Set(headerContentDisp, "inline; filename="+filename)
	writeMultiSupplierReportDocument(w, entries, dateFrom, dateTo, false)
}

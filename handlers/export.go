package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"afrita/helpers"
)

// HandleExportInvoicesCSV exports all invoices as a CSV file
func HandleExportInvoicesCSV(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	invoices, err := helpers.FetchAllInvoicesUnpaginated(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل الفواتير")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=invoices.csv")
	// UTF-8 BOM for Excel Arabic support
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	_ = writer.Write([]string{"رقم الفاتورة", "التاريخ", "الإجمالي", "ض.ق.م", "الخصم", "الحالة", "النوع"})

	for _, inv := range invoices {
		status, _ := helpers.InvoiceStatus(inv)
		status = helpers.TranslateInvoiceStatus(status)
		invType := helpers.InvoiceTypeLabel(inv)

		_ = writer.Write([]string{
			fmt.Sprintf("%d", inv.SequenceNumber),
			inv.EffectiveDate.Time,
			fmt.Sprintf("%.2f", inv.Total),
			fmt.Sprintf("%.2f", inv.TotalVAT),
			fmt.Sprintf("%.2f", inv.Discount),
			status,
			invType,
		})
	}
}

// HandleExportProductsCSV exports all products as a CSV file
func HandleExportProductsCSV(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	products, err := helpers.FetchProducts(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل المنتجات")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=products.csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"المعرف", "اسم القطعة", "السعر", "الكمية", "الحالة"})

	for _, p := range products {
		stockStatus := "متوفر"
		if helpers.ParseIntValue(p.Quantity) <= 0 {
			stockStatus = "منتهي"
		}
		_ = writer.Write([]string{
			fmt.Sprintf("%d", p.ID),
			p.PartName,
			p.Price,
			p.Quantity,
			stockStatus,
		})
	}
}

// HandleExportClientsCSV exports all clients as a CSV file
func HandleExportClientsCSV(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	clients, err := helpers.FetchClients(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل العملاء")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=clients.csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"المعرف", "الاسم", "البريد الإلكتروني", "الهاتف"})

	for _, c := range clients {
		_ = writer.Write([]string{c.ID, c.Name, c.Email, c.Phone})
	}
}

// HandleExportSuppliersCSV exports all suppliers as a CSV file
func HandleExportSuppliersCSV(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	suppliers, err := helpers.FetchSuppliers(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل الموردين")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=suppliers.csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"المعرف", "الاسم", "الهاتف", "العنوان", "الرقم الضريبي"})

	for _, s := range suppliers {
		_ = writer.Write([]string{
			fmt.Sprintf("%d", s.ID),
			s.Name,
			s.PhoneNumber,
			s.Address,
			s.VATNumber,
		})
	}
}

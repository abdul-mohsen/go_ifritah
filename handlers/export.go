package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"

	"afrita/helpers"
	"afrita/models"

	"github.com/xuri/excelize/v2"
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

type purchaseBillExportProduct struct {
	BillReference  string
	ProductName    string
	Quantity       int
	PurchasePrice  float64
	CostPrice      float64
	ShelfNumber    string
	Discount       float64
	TotalBeforeVAT float64
}

type purchaseBillExportBill struct {
	Reference             string
	StoreID               int
	SupplierID            int
	SupplierInvoiceNumber int
	Date                  string
	PaymentMethod         int
	Discount              float64
}

// HandleExportPurchaseBillsXLSX exports purchase bills and their line items
// into separate Excel worksheets.
func HandleExportPurchaseBillsXLSX(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	bills, err := helpers.FetchPurchaseBillsAll(token, 1, "", "")
	if err != nil {
		log.Printf("[purchase-bills export] backend list fetch failed: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل فواتير المشتريات")
		return
	}

	productRows := make([]purchaseBillExportProduct, 0)
	exportBills := make([]purchaseBillExportBill, 0, len(bills))
	for _, bill := range bills {
		detail, products, manualProducts, extra, err := helpers.FetchPurchaseBillDetail(token, fmt.Sprintf("%d", bill.ID))
		if err != nil {
			log.Printf("[purchase-bills export] detail fetch failed for bill %d: %v", bill.ID, err)
			helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل منتجات فواتير المشتريات")
			return
		}
		reference := fmt.Sprintf("PB-%d", bill.ID)
		exportBills = append(exportBills, purchaseBillExportBill{
			Reference: reference, StoreID: exportExtraInt(extra, "store_id"), SupplierID: exportExtraInt(extra, "supplier_id"),
			SupplierInvoiceNumber: exportExtraInt(extra, "supplier_sequence_number"), Date: detail.EffectiveDate.Time,
			PaymentMethod: exportExtraInt(extra, "payment_method"), Discount: detail.Discount,
		})
		productRows = appendPurchaseBillExportProducts(productRows, reference, products)
		productRows = appendPurchaseBillExportProducts(productRows, reference, manualProducts)
	}

	workbook, err := buildPurchaseBillExportWorkbook(exportBills, productRows, isArabic(token))
	if err != nil {
		log.Printf("[purchase-bills export] workbook build failed: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر إنشاء ملف Excel")
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=purchase-bills.xlsx")
	if err := workbook.Write(w); err != nil {
		log.Printf("[purchase-bills export] workbook write failed: %v", err)
	}
}

func appendPurchaseBillExportProducts(rows []purchaseBillExportProduct, reference string, items []models.BillItem) []purchaseBillExportProduct {
	for _, item := range items {
		rows = append(rows, purchaseBillExportProduct{
			BillReference:  reference,
			ProductName:    item.PartName,
			Quantity:       item.Quantity,
			PurchasePrice:  item.Price,
			CostPrice:      item.CostPrice,
			ShelfNumber:    item.ShelfNumber,
			Discount:       item.Discount,
			TotalBeforeVAT: purchaseBillLineTotal(item),
		})
	}
	return rows
}

func purchaseBillLineTotal(item models.BillItem) float64 {
	if item.TotalBeforeVAT != 0 {
		return item.TotalBeforeVAT
	}
	return item.Price * float64(item.Quantity)
}

func buildPurchaseBillExportWorkbook(bills []purchaseBillExportBill, productRows []purchaseBillExportProduct, arabic bool) (*excelize.File, error) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", "Bills"); err != nil {
		return nil, fmt.Errorf("rename bills worksheet: %w", err)
	}
	if _, err := workbook.NewSheet("Products"); err != nil {
		return nil, fmt.Errorf("create products worksheet: %w", err)
	}

	headerStyle, err := workbook.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("create header style: %w", err)
	}

	billHeaders, productHeaders := purchaseBillWorkbookHeaders(arabic)
	if err := writeExcelRow(workbook, "Bills", 1, billHeaders); err != nil {
		return nil, err
	}
	if err := workbook.SetRowStyle("Bills", 1, 1, headerStyle); err != nil {
		return nil, fmt.Errorf("style bills header: %w", err)
	}
	for index, bill := range bills {
		values := []interface{}{
			bill.Reference,
			bill.StoreID,
			bill.SupplierID,
			bill.SupplierInvoiceNumber,
			bill.Date,
			bill.PaymentMethod,
			bill.Discount,
		}
		if err := writeExcelRow(workbook, "Bills", index+2, values); err != nil {
			return nil, err
		}
	}

	if err := writeExcelRow(workbook, "Products", 1, productHeaders); err != nil {
		return nil, err
	}
	if err := workbook.SetRowStyle("Products", 1, 1, headerStyle); err != nil {
		return nil, fmt.Errorf("style products header: %w", err)
	}
	for index, product := range productRows {
		values := []interface{}{
			product.BillReference,
			product.ProductName,
			product.Quantity,
			product.PurchasePrice,
			product.CostPrice,
			product.ShelfNumber,
			product.Discount,
			product.TotalBeforeVAT,
		}
		if err := writeExcelRow(workbook, "Products", index+2, values); err != nil {
			return nil, err
		}
	}

	for _, sheet := range []string{"Bills", "Products"} {
		if err := workbook.SetColWidth(sheet, "A", "K", 18); err != nil {
			return nil, fmt.Errorf("set %s worksheet width: %w", sheet, err)
		}
	}
	return workbook, nil
}

func purchaseBillWorkbookHeaders(arabic bool) ([]interface{}, []interface{}) {
	billHeaders, productHeaders := billImportHeaders(billImportPurchase, arabic)
	bills := make([]interface{}, len(billHeaders))
	products := make([]interface{}, len(productHeaders))
	for index := range billHeaders {
		bills[index] = billHeaders[index]
	}
	for index := range productHeaders {
		products[index] = productHeaders[index]
	}
	return bills, products
}

func exportExtraInt(values map[string]interface{}, key string) int {
	value, ok := helpers.CoerceFloat(values[key])
	if !ok {
		return 0
	}
	return int(value)
}

// HandleExportSalesBillsXLSX exports sales bills using the same two-sheet
// contract accepted by the shared Excel importer.
func HandleExportSalesBillsXLSX(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	invoices, err := helpers.FetchAllInvoicesUnpaginated(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "Unable to load sales bills")
		return
	}
	workbook, err := buildSalesBillExportWorkbook(token, invoices, isArabic(token))
	if err != nil {
		log.Printf("[sales-bills export] workbook build failed: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "Unable to create Excel workbook")
		return
	}
	defer workbook.Close()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=sales-bills.xlsx")
	if err := workbook.Write(w); err != nil {
		log.Printf("[sales-bills export] workbook write failed: %v", err)
	}
}

func buildSalesBillExportWorkbook(token string, invoices []models.Invoice, arabic bool) (*excelize.File, error) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", "Bills"); err != nil {
		return nil, err
	}
	if _, err := workbook.NewSheet("Products"); err != nil {
		return nil, err
	}
	billHeaders, productHeaders := billImportHeaders(billImportSales, arabic)
	if err := writeStringRow(workbook, "Bills", 1, billHeaders); err != nil {
		return nil, err
	}
	if err := writeStringRow(workbook, "Products", 1, productHeaders); err != nil {
		return nil, err
	}
	headerStyle, err := workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := workbook.SetRowStyle("Bills", 1, 1, headerStyle); err != nil {
		return nil, err
	}
	if err := workbook.SetRowStyle("Products", 1, 1, headerStyle); err != nil {
		return nil, err
	}
	productRow := 2
	for index, invoice := range invoices {
		detail, products, manualProducts, extra, err := helpers.FetchBillDetail(token, fmt.Sprintf("%d", invoice.ID))
		if err != nil {
			return nil, fmt.Errorf("load sales bill %d: %w", invoice.ID, err)
		}
		reference := fmt.Sprintf("SB-%d", invoice.ID)
		customerID := exportExtraInt(extra, "client_id")
		customerName := exportExtraString(extra, "user_name")
		customerPhone := exportExtraString(extra, "user_phone_number")
		note := exportExtraString(extra, "note")
		vin := exportExtraString(extra, "vin")
		if err := writeExcelRow(workbook, "Bills", index+2, []interface{}{
			reference, exportExtraInt(extra, "store_id"), exportExtraInt(extra, "branch_id"), customerID, customerName, customerPhone,
			detail.EffectiveDate.Time, detail.Discount, note, vin,
		}); err != nil {
			return nil, err
		}
		for _, item := range append(products, manualProducts...) {
			if err := writeExcelRow(workbook, "Products", productRow, []interface{}{
				reference, exportedProductName(item), item.Quantity, item.Price, purchaseBillLineTotal(item),
			}); err != nil {
				return nil, err
			}
			productRow++
		}
	}
	for _, sheet := range []string{"Bills", "Products"} {
		if err := workbook.SetColWidth(sheet, "A", "I", 20); err != nil {
			return nil, err
		}
	}
	return workbook, nil
}

func exportExtraString(values map[string]interface{}, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func exportedProductName(item models.BillItem) string {
	if item.PartName != "" {
		return item.PartName
	}
	return item.PartNumber
}

func writeExcelRow(workbook *excelize.File, sheet string, row int, values []interface{}) error {
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return fmt.Errorf("build %s worksheet row %d: %w", sheet, row, err)
	}
	if err := workbook.SetSheetRow(sheet, cell, &values); err != nil {
		return fmt.Errorf("write %s worksheet row %d: %w", sheet, row, err)
	}
	return nil
}

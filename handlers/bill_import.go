package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/xuri/excelize/v2"
)

const (
	billImportSales    = "sales"
	billImportPurchase = "purchase"
)

type spreadsheetBill struct {
	reference string
	values    map[string]string
}

type spreadsheetProduct struct {
	reference string
	values    map[string]string
}

type bulkImportLineResult struct {
	Index int    `json:"index"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type bulkImportResponse struct {
	Error   string                 `json:"error,omitempty"`
	Total   int                    `json:"total"`
	Success int                    `json:"success"`
	Failed  int                    `json:"failed"`
	Results []bulkImportLineResult `json:"results"`
}

// HandleBillImportPage renders the shared sales and purchase bill import page.
func HandleBillImportPage(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	importType := billImportType(r.URL.Query().Get("type"))
	helpers.Render(w, r, "bill-import", map[string]interface{}{
		"title":       billImportTitle(importType, isArabic(token)),
		"import_type": importType,
		"arabic":      isArabic(token),
	})
}

func HandleDownloadBillImportTemplate(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	importType := billImportType(r.URL.Query().Get("type"))
	workbook, err := buildBillImportTemplate(importType, isArabic(token))
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "Unable to create Excel template")
		return
	}
	defer workbook.Close()

	w.Header().Set(headerContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-bills-import-template.xlsx", importType))
	if err := workbook.Write(w); err != nil {
		log.Printf("[bill import] template write failed: %v", err)
	}
}

// HandleImportBillsXLSX validates a two-sheet workbook and creates every
// referenced bill independently, returning per-bill results for partial failures.
func HandleImportBillsXLSX(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeBulkImportJSON(w, http.StatusBadRequest, bulkImportResponse{Error: "Unable to read upload"})
		return
	}
	importType := billImportType(r.URL.Query().Get("type"))
	file, _, err := r.FormFile("file")
	if err != nil {
		writeBulkImportJSON(w, http.StatusBadRequest, bulkImportResponse{Error: "Choose an .xlsx file"})
		return
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		writeBulkImportJSON(w, http.StatusBadRequest, bulkImportResponse{Error: "Unable to read upload"})
		return
	}

	bills, products, err := parseBillImportWorkbook(payload, importType)
	if err != nil {
		writeBulkImportJSON(w, http.StatusBadRequest, bulkImportResponse{Error: err.Error()})
		return
	}

	response := bulkImportResponse{Total: len(bills), Results: make([]bulkImportLineResult, 0, len(bills))}
	for index, bill := range bills {
		items := products[bill.reference]
		if len(items) == 0 {
			response.Failed++
			response.Results = append(response.Results, bulkImportLineResult{Index: index, Error: "Bill Reference has no products"})
			continue
		}
		var createErr error
		if importType == billImportPurchase {
			createErr = submitPurchaseImport(token, bill, items)
		} else {
			createErr = submitSalesImport(token, bill, items)
		}
		result := bulkImportLineResult{Index: index, OK: createErr == nil}
		if createErr != nil {
			result.Error = createErr.Error()
			response.Failed++
		} else {
			response.Success++
		}
		response.Results = append(response.Results, result)
	}
	helpers.APICache.Delete("invoices_all")
	helpers.APICache.Delete("purchase_bills")
	writeBulkImportJSON(w, http.StatusOK, response)
}

func billImportType(value string) string {
	if strings.EqualFold(value, billImportPurchase) {
		return billImportPurchase
	}
	return billImportSales
}

func isArabic(token string) bool {
	return getSettings(token)["language"] != "en"
}

func billImportTitle(importType string, arabic bool) string {
	if arabic {
		if importType == billImportPurchase {
			return "استيراد فواتير المشتريات"
		}
		return "استيراد فواتير المبيعات"
	}
	if importType == billImportPurchase {
		return "Import Purchase Bills"
	}
	return "Import Sales Bills"
}

// Column labels shared by more than one header row (the join column and a
// few fields that appear on both the Bills and Products sheets). Named as
// constants to avoid SonarCloud S1192 duplicate-literal warnings and to
// guarantee the Bills/Products join column text always matches exactly.
const (
	colBillReferenceAr = "مرجع الفاتورة"
	colBillReferenceEN = "Bill Reference"
	colDiscountAr      = "الخصم"
	colDiscountEN      = "Discount"
	colLineTotalAr     = "إجمالي السطر"
	colLineTotalEN     = "Line Total"
)

func billImportHeaders(importType string, arabic bool) ([]string, []string) {
	if arabic {
		if importType == billImportPurchase {
			return []string{colBillReferenceAr, "معرف المخزن", "معرف المورد", "رقم فاتورة المورد", "التاريخ", "طريقة الدفع", colDiscountAr},
				[]string{colBillReferenceAr, "اسم المنتج", "الكمية", "سعر الشراء", "سعر التكلفة", "رقم الرف", colDiscountAr, colLineTotalAr}
		}
		return []string{colBillReferenceAr, "معرف المخزن", "معرف الفرع", "معرف العميل", "اسم العميل", "هاتف العميل", "التاريخ", colDiscountAr, "ملاحظة", "رقم الهيكل"},
			[]string{colBillReferenceAr, "اسم المنتج", "الكمية", "سعر الوحدة", colLineTotalAr}
	}
	if importType == billImportPurchase {
		return []string{colBillReferenceEN, "Store ID", "Supplier ID", "Supplier Invoice Number", "Date", "Payment Method", colDiscountEN},
			[]string{colBillReferenceEN, "Product Name", "Quantity", "Purchase Price", "Cost Price", "Shelf Number", colDiscountEN, colLineTotalEN}
	}
	return []string{colBillReferenceEN, "Store ID", "Branch ID", "Customer ID", "Customer Name", "Customer Phone", "Date", colDiscountEN, "Note", "VIN"},
		[]string{colBillReferenceEN, "Product Name", "Quantity", "Unit Price", colLineTotalEN}
}

func buildBillImportTemplate(importType string, arabic bool) (*excelize.File, error) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", "Bills"); err != nil {
		return nil, err
	}
	if _, err := workbook.NewSheet("Products"); err != nil {
		return nil, err
	}
	billHeaders, productHeaders := billImportHeaders(importType, arabic)
	if err := writeStringRow(workbook, "Bills", 1, billHeaders); err != nil {
		return nil, err
	}
	if err := writeStringRow(workbook, "Products", 1, productHeaders); err != nil {
		return nil, err
	}
	if importType == billImportPurchase {
		const examplePB1, examplePB2 = "PB-EXAMPLE-001", "PB-EXAMPLE-002"
		_ = writeStringRow(workbook, "Bills", 2, []string{examplePB1, "1", "1", "10001", "2026-01-15", "10", "0"})
		_ = writeStringRow(workbook, "Bills", 3, []string{examplePB2, "1", "2", "10002", "2026-01-16", "10", "5"})
		_ = writeStringRow(workbook, "Products", 2, []string{examplePB1, "Oil Filter", "2", "25", "20", "A-01", "0", "50"})
		_ = writeStringRow(workbook, "Products", 3, []string{examplePB1, "Air Filter", "1", "30", "24", "A-02", "0", "30"})
		_ = writeStringRow(workbook, "Products", 4, []string{examplePB2, "Battery", "1", "200", "180", "B-01", "0", "200"})
	} else {
		const exampleSB1, exampleSB2 = "SB-EXAMPLE-001", "SB-EXAMPLE-002"
		_ = writeStringRow(workbook, "Bills", 2, []string{exampleSB1, "1", "1", "", "Walk-in Customer", "0500000001", "2026-01-15", "0", "First example bill", "VIN-EXAMPLE-001"})
		_ = writeStringRow(workbook, "Bills", 3, []string{exampleSB2, "1", "1", "", "Second Customer", "0500000002", "2026-01-16", "5", "Second example bill", "VIN-EXAMPLE-002"})
		_ = writeStringRow(workbook, "Products", 2, []string{exampleSB1, "Brake Pad", "2", "75", "150"})
		_ = writeStringRow(workbook, "Products", 3, []string{exampleSB1, "Engine Oil", "1", "40", "40"})
		_ = writeStringRow(workbook, "Products", 4, []string{exampleSB2, "Wiper Blade", "2", "20", "40"})
	}
	style, err := workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	for _, sheet := range []string{"Bills", "Products"} {
		if err := workbook.SetRowStyle(sheet, 1, 1, style); err != nil {
			return nil, err
		}
		if err := workbook.SetColWidth(sheet, "A", "I", 20); err != nil {
			return nil, err
		}
	}
	return workbook, nil
}

func writeStringRow(workbook *excelize.File, sheet string, row int, values []string) error {
	cells := make([]interface{}, len(values))
	for index := range values {
		cells[index] = values[index]
	}
	return writeExcelRow(workbook, sheet, row, cells)
}

func parseBillImportWorkbook(payload []byte, importType string) ([]spreadsheetBill, map[string][]spreadsheetProduct, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid .xlsx workbook")
	}
	defer workbook.Close()
	if !containsSheet(workbook.GetSheetList(), "Bills") || !containsSheet(workbook.GetSheetList(), "Products") {
		return nil, nil, fmt.Errorf("workbook must include Bills and Products sheets")
	}
	billHeaders, productHeaders := billImportHeaders(importType, false)
	billsRows, err := workbook.GetRows("Bills")
	if err != nil {
		return nil, nil, fmt.Errorf("read Bills sheet")
	}
	productRows, err := workbook.GetRows("Products")
	if err != nil {
		return nil, nil, fmt.Errorf("read Products sheet")
	}
	billMaps, err := rowsToMaps(billsRows, billHeaders)
	if err != nil {
		return nil, nil, fmt.Errorf("Bills: %w", err)
	}
	productMaps, err := rowsToMaps(productRows, productHeaders)
	if err != nil {
		return nil, nil, fmt.Errorf("Products: %w", err)
	}
	bills := make([]spreadsheetBill, 0, len(billMaps))
	references := make(map[string]bool)
	for row, values := range billMaps {
		ref := strings.TrimSpace(values["bill_reference"])
		if ref == "" || references[ref] {
			return nil, nil, fmt.Errorf("Bills row %d has an empty or duplicate Bill Reference", row+2)
		}
		if parsePositiveInt(values["store_id"]) == 0 {
			return nil, nil, fmt.Errorf("Bills row %d requires a numeric Store ID", row+2)
		}
		if strings.TrimSpace(values["date"]) == "" {
			return nil, nil, fmt.Errorf("Bills row %d requires Date", row+2)
		}
		if importType == billImportPurchase && (parsePositiveInt(values["supplier_id"]) == 0 || parsePositiveUint(values["supplier_invoice_number"]) == 0) {
			return nil, nil, fmt.Errorf("Bills row %d requires numeric Supplier ID and Supplier Invoice Number", row+2)
		}
		if importType == billImportSales && strings.TrimSpace(values["customer_id"]) != "" && parsePositiveInt(values["customer_id"]) == 0 {
			return nil, nil, fmt.Errorf("Bills row %d has an invalid Customer ID", row+2)
		}
		references[ref] = true
		bills = append(bills, spreadsheetBill{reference: ref, values: values})
	}
	if len(bills) == 0 {
		return nil, nil, fmt.Errorf("Bills sheet has no data rows")
	}
	products := make(map[string][]spreadsheetProduct)
	for row, values := range productMaps {
		ref := strings.TrimSpace(values["bill_reference"])
		if !references[ref] {
			return nil, nil, fmt.Errorf("Products row %d references an unknown Bill Reference", row+2)
		}
		if strings.TrimSpace(values["product_name"]) == "" || parsePositiveInt(values["quantity"]) == 0 {
			return nil, nil, fmt.Errorf("Products row %d requires Product Name and a positive Quantity", row+2)
		}
		priceKey := "unit_price"
		if importType == billImportPurchase {
			priceKey = "purchase_price"
		}
		price := parseSheetFloat(values[priceKey])
		lineTotal := parseSheetFloat(values["line_total"])
		if price == 0 && lineTotal > 0 {
			price = lineTotal / float64(parsePositiveInt(values["quantity"]))
			values[priceKey] = strconv.FormatFloat(price, 'f', -1, 64)
		}
		if price <= 0 {
			return nil, nil, fmt.Errorf("Products row %d requires %s or Line Total", row+2, strings.ReplaceAll(priceKey, "_", " "))
		}
		products[ref] = append(products[ref], spreadsheetProduct{reference: ref, values: values})
	}
	return bills, products, nil
}

func containsSheet(sheets []string, wanted string) bool {
	for _, sheet := range sheets {
		if sheet == wanted {
			return true
		}
	}
	return false
}

func rowsToMaps(rows [][]string, expected []string) ([]map[string]string, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("sheet needs a header and at least one row")
	}
	indices := make(map[string]int)
	for index, header := range rows[0] {
		indices[normalizeContractHeader(header)] = index
	}
	keys := make([]string, len(expected))
	for index, header := range expected {
		key := normalizeContractHeader(header)
		keys[index] = key
		if _, ok := indices[key]; !ok {
			return nil, fmt.Errorf("missing %q column", header)
		}
	}
	result := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		values := make(map[string]string, len(keys))
		empty := true
		for _, key := range keys {
			index := indices[key]
			if index < len(row) {
				values[key] = strings.TrimSpace(row[index])
				empty = empty && values[key] == ""
			}
		}
		if !empty {
			result = append(result, values)
		}
	}
	return result, nil
}

func normalizeContractHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "ـ", "", "(", "", ")", "")
	value = replacer.Replace(value)
	arabicHeaders := map[string]string{
		"مرجع_الفاتورة":     "bill_reference",
		"معرف_المخزن":       "store_id",
		"معرف_الفرع":        "branch_id",
		"معرف_المورد":       "supplier_id",
		"رقم_فاتورة_المورد": "supplier_invoice_number",
		"التاريخ":           "date",
		"طريقة_الدفع":       "payment_method",
		"الخصم":             "discount",
		"اسم_المنتج":        "product_name",
		"الكمية":            "quantity",
		"سعر_الشراء":        "purchase_price",
		"سعر_التكلفة":       "cost_price",
		"رقم_الرف":          "shelf_number",
		"إجمالي_السطر":      "line_total",
		"معرف_العميل":       "customer_id",
		"اسم_العميل":        "customer_name",
		"هاتف_العميل":       "customer_phone",
		"ملاحظة":            "note",
		"رقم_الهيكل":        "vin",
		"سعر_الوحدة":        "unit_price",
	}
	if canonical, ok := arabicHeaders[value]; ok {
		return canonical
	}
	return value
}

func parsePositiveInt(value string) int {
	value = strings.TrimSpace(value)
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func parsePositiveUint(value string) uint64 {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0
	}
	return parsed
}

func parseSheetFloat(value string) float64 {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return parsed
}

func submitSalesImport(token string, bill spreadsheetBill, products []spreadsheetProduct) error {
	items, total := importSalesItems(products)
	effective := helpers.ToBackendDate(bill.values["date"])
	if effective == "" {
		return fmt.Errorf("invalid Date")
	}
	var customerID *int
	if id := parsePositiveInt(bill.values["customer_id"]); id > 0 {
		customerID = &id
	}
	branchID := parsePositiveInt(bill.values["branch_id"])
	if branchID == 0 {
		branchID = 1
	}
	payload := models.BillPayload{
		StoreID:         parsePositiveInt(bill.values["store_id"]),
		Products:        []models.BillProductItem{},
		ManualProducts:  items,
		TotalAmount:     total,
		Discount:        importNumber(bill.values["discount"]),
		MaintenanceCost: "0",
		State:           3,
		VIN:             bill.values["vin"],
		UserName:        bill.values["customer_name"],
		UserPhoneNumber: bill.values["customer_phone"],
		Note:            bill.values["note"],
		EffectiveDate:   &effective,
		PaymentMethod:   10,
		ClientID:        customerID,
		BranchID:        branchID,
	}
	return postImportedBill(token, "/api/v2/bill", payload)
}

func submitPurchaseImport(token string, bill spreadsheetBill, products []spreadsheetProduct) error {
	items, total := importPurchaseItems(products)
	effective := helpers.ToBackendDate(bill.values["date"])
	if effective == "" {
		return fmt.Errorf("invalid Date")
	}
	supplierID := parsePositiveInt(bill.values["supplier_id"])
	payload := models.PurchaseBillPayload{
		StoreID:                parsePositiveInt(bill.values["store_id"]),
		MerchantID:             supplierID,
		SupplierID:             supplierID,
		SupplierSequenceNumber: parsePositiveUint(bill.values["supplier_invoice_number"]),
		State:                  1,
		EffectiveDate:          effective,
		Products:               []models.BillProductItem{},
		ManualProducts:         items,
		Discount:               importNumber(bill.values["discount"]),
		Subtotal:               total,
		PaymentMethod:          importPaymentMethod(bill.values["payment_method"]),
		PaidAmount:             "0",
		Attachments:            []string{},
	}
	return postImportedBill(token, "/api/v2/purchase_bill", payload)
}

func importSalesItems(products []spreadsheetProduct) ([]models.BillManualItem, float64) {
	items := make([]models.BillManualItem, 0, len(products))
	total := 0.0
	for _, product := range products {
		quantity := parsePositiveInt(product.values["quantity"])
		price := parseSheetFloat(product.values["unit_price"])
		lineTotal := parseSheetFloat(product.values["line_total"])
		if lineTotal == 0 {
			lineTotal = price * float64(quantity)
		}
		total += lineTotal
		items = append(items, models.BillManualItem{PartName: product.values["product_name"], Price: importNumber(product.values["unit_price"]), Quantity: strconv.Itoa(quantity)})
	}
	return items, total
}

func importPurchaseItems(products []spreadsheetProduct) ([]models.BillManualItem, float64) {
	items := make([]models.BillManualItem, 0, len(products))
	total := 0.0
	for _, product := range products {
		quantity := parsePositiveInt(product.values["quantity"])
		price := parseSheetFloat(product.values["purchase_price"])
		lineTotal := parseSheetFloat(product.values["line_total"])
		if lineTotal == 0 {
			lineTotal = price * float64(quantity)
		}
		total += lineTotal
		items = append(items, models.BillManualItem{
			PartName: product.values["product_name"], Price: importNumber(product.values["purchase_price"]),
			Quantity: strconv.Itoa(quantity), CostPrice: importNumber(product.values["cost_price"]),
			ShelfNumber: product.values["shelf_number"], Discount: importNumber(product.values["discount"]),
		})
	}
	return items, total
}

func importNumber(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0"
	}
	return strconv.FormatFloat(parseSheetFloat(value), 'f', -1, 64)
}

func importPaymentMethod(value string) int {
	if parsed := parsePositiveInt(value); parsed > 0 {
		return parsed
	}
	return 10
}

func postImportedBill(token, path string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("prepare bill")
	}
	log.Printf("[bill import] POST %s payload: %s", path, string(body))
	req, err := http.NewRequest(http.MethodPost, config.BackendDomain+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("prepare request")
	}
	req.Header.Set(headerContentType, mimeJSON)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		return fmt.Errorf("backend request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		response, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		log.Printf("[bill import] backend %s -> status=%d body=%s", path, resp.StatusCode, string(response))
		return fmt.Errorf("backend rejected bill (status %d): %s", resp.StatusCode, strings.TrimSpace(string(response)))
	}
	return nil
}

func writeBulkImportJSON(w http.ResponseWriter, status int, result bulkImportResponse) {
	w.Header().Set(headerContentType, "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}

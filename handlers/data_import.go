package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// ── Preview endpoint: parse CSV and return headers + sample rows ──

// importPreviewResponse is returned by the preview endpoint.
type importPreviewResponse struct {
	Headers []string   `json:"headers"`
	Samples [][]string `json:"samples"` // first 3 data rows
	Total   int        `json:"total"`   // total data rows
	Error   string     `json:"error,omitempty"`
}

// HandleDataImportPreview accepts a CSV upload, parses headers + sample rows,
// and returns them as JSON so the client can build a column-mapping UI.
func HandleDataImportPreview(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, importPreviewResponse{Error: "فشل في قراءة الملف: " + err.Error()})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, importPreviewResponse{Error: "يرجى اختيار ملف"})
		return
	}
	defer file.Close()

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, importPreviewResponse{Error: "فشل في قراءة الملف"})
		return
	}

	rawStr := string(rawBytes)
	rawStr = strings.TrimPrefix(rawStr, "\xef\xbb\xbf") // strip BOM

	separator := detectSeparator(rawStr)

	reader := csv.NewReader(strings.NewReader(rawStr))
	reader.Comma = separator
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, importPreviewResponse{Error: "فشل في تحليل الملف: " + err.Error()})
		return
	}

	if len(records) < 2 {
		writeJSON(w, http.StatusBadRequest, importPreviewResponse{Error: "الملف فارغ أو يحتوي على رأس فقط"})
		return
	}

	headers := records[0]
	dataRows := records[1:]
	samples := dataRows
	if len(samples) > 3 {
		samples = samples[:3]
	}

	writeJSON(w, http.StatusOK, importPreviewResponse{
		Headers: headers,
		Samples: samples,
		Total:   len(dataRows),
	})
}

// ── Import endpoint: process rows with user-defined column mapping ──

// importRequest is the JSON body sent by the client after column mapping.
type importRequest struct {
	Type         string         `json:"type"` // "bills", "purchase_bills", "products"
	StoreID      int            `json:"store_id"`
	Mapping      map[string]int `json:"mapping"`                 // field_name → column_index
	Rows         [][]string     `json:"rows"`                    // all data rows (excluding header)
	SelectedRows []int          `json:"selected_rows,omitempty"` // sales imports: the rows to post, from the preview
}

// HandleDataImportExecute processes the mapped rows and submits them to the backend.
func HandleDataImportExecute(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, importResult{Error: "بيانات غير صالحة: " + err.Error()})
		return
	}

	if req.StoreID == 0 {
		writeJSON(w, http.StatusBadRequest, importResult{Error: "يرجى اختيار المخزن"})
		return
	}

	if len(req.Rows) == 0 {
		writeJSON(w, http.StatusBadRequest, importResult{Error: "لا توجد بيانات للاستيراد"})
		return
	}

	switch req.Type {
	case "bills":
		importBills(w, token, req)
	case "purchase_bills":
		importPurchaseBills(w, token, req)
	case "products":
		importProducts(w, token, req)
	default:
		writeJSON(w, http.StatusBadRequest, importResult{Error: "نوع الاستيراد غير معروف: " + req.Type})
	}
}

// ── Bill import ─────────────────────────────────────────────────

// HandleDataImportCheck checks the rows of a sales data import before anything is posted. It answers each row with its status.
func HandleDataImportCheck(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, previewResponse{Error: "بيانات غير صالحة: " + err.Error()})
		return
	}
	if req.Type != "bills" {
		writeJSON(w, http.StatusBadRequest, previewResponse{Error: "only sales rows are checked"})
		return
	}
	if req.StoreID == 0 {
		writeJSON(w, http.StatusBadRequest, previewResponse{Error: "يرجى اختيار المخزن"})
		return
	}
	if len(req.Rows) == 0 {
		writeJSON(w, http.StatusBadRequest, previewResponse{Error: "لا توجد بيانات للاستيراد"})
		return
	}
	rows, err := checkSalesCandidates(token, billCandidates(req))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, previewResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, previewResponse{Total: len(rows), Rows: rows})
}

// importBills creates the bills of a sales data import. It posts only the rows that the user selected in the preview, and each
// of those rows is checked again before it is posted.
func importBills(w http.ResponseWriter, token string, req importRequest) {
	selected, err := selectedRowsOfInts(req.SelectedRows)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, importResult{Error: err.Error()})
		return
	}
	outcomes, err := commitSales(token, billCandidates(req), selected)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, importResult{Error: err.Error()})
		return
	}
	results, success, failed := billResultsOf(outcomes)
	helpers.APICache.Delete("invoices_all")
	writeJSON(w, http.StatusOK, importResult{Total: len(outcomes), Success: success, Failed: failed, Results: results})
}

// dataBillRow is one row of a data import, read through the mapping of its columns.
type dataBillRow struct {
	partName string
	price    float64
	quantity int
	discount string
	customer string
	phone    string
	vin      string
	note     string
	date     string
}

// dataBill is one bill of a data import: its row number, and the rows that make it.
type dataBill struct {
	row  int
	rows []dataBillRow
}

// dataCell returns the trimmed value of a mapped column. It is "" when the column is not mapped or the row is short.
func dataCell(mapping map[string]int, row []string, field string) string {
	idx, ok := mapping[field]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// parseDataRow reads one row of a data import through the mapping.
func parseDataRow(mapping map[string]int, row []string) dataBillRow {
	br := dataBillRow{quantity: 1}
	br.partName = dataCell(mapping, row, "part_name")
	br.price = parsePrice(dataCell(mapping, row, "price"))
	if q := helpers.ParseIntValue(dataCell(mapping, row, "quantity")); q > 0 {
		br.quantity = q
	}
	br.discount = cleanNumeric(dataCell(mapping, row, "discount"))
	br.customer = dataCell(mapping, row, "customer_name")
	br.phone = dataCell(mapping, row, "customer_phone")
	br.vin = dataCell(mapping, row, "vin")
	br.note = dataCell(mapping, row, "note")
	br.date = dataCell(mapping, row, "date")
	return br
}

// dataBillOf builds the sale request of the rows of one bill of a data import.
func dataBillOf(req importRequest, rows []dataBillRow) models.BillPayload {
	items := make([]models.BillManualItem, 0, len(rows))
	var total float64
	for _, r := range rows {
		name := r.partName
		if name == "" {
			name = "بند"
		}
		items = append(items, models.BillManualItem{
			PartName: name,
			Price:    formatPrice(r.price),
			Quantity: strconv.Itoa(r.quantity),
		})
		total += r.price * float64(r.quantity)
	}
	first := rows[0]
	discount := first.discount
	if discount == "" {
		discount = "0"
	}
	bp := models.BillPayload{
		StoreID:         req.StoreID,
		Products:        []models.BillProductItem{},
		ManualProducts:  items,
		TotalAmount:     total,
		Discount:        discount,
		MaintenanceCost: "0",
		State:           3, // issued
		UserName:        first.customer,
		UserPhoneNumber: first.phone,
		Note:            first.note,
		VIN:             first.vin,
		PaymentMethod:   10, // cash
	}
	if first.date != "" {
		if d := parseDateToRFC3339(first.date); d != "" {
			bp.EffectiveDate = &d
		}
	}
	return bp
}

// dataBillsOf groups the rows of a data import into bills. A group of rows is one bill, and it takes the number of its group. A row
// on its own is a bill, and it takes the number of the rows before it.
func dataBillsOf(req importRequest) []dataBill {
	if billGroupIdx, hasBillGroup := req.Mapping["bill_group"]; hasBillGroup {
		return groupedDataBills(req, billGroupIdx)
	}
	var bills []dataBill
	for _, row := range req.Rows {
		br := parseDataRow(req.Mapping, row)
		if br.partName == "" && br.price == 0 {
			continue
		}
		bills = append(bills, dataBill{row: len(bills) + 1, rows: []dataBillRow{br}})
	}
	return bills
}

// groupedDataBills groups the rows of a data import by the value of their bill_group column.
func groupedDataBills(req importRequest, billGroupIdx int) []dataBill {
	groups := make(map[string][]dataBillRow)
	var order []string
	for _, row := range req.Rows {
		br := parseDataRow(req.Mapping, row)
		if br.partName == "" && br.price == 0 {
			continue
		}
		key := ""
		if billGroupIdx < len(row) {
			key = strings.TrimSpace(row[billGroupIdx])
		}
		if key == "" {
			key = fmt.Sprintf("__single_%d", len(order))
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], br)
	}
	bills := make([]dataBill, 0, len(order))
	for i, key := range order {
		bills = append(bills, dataBill{row: i + 1, rows: groups[key]})
	}
	return bills
}

// dataBatchKey names the batch of a data import: the SHA-256 of the request as it encodes without its selection, so a check and a
// commit of the same rows share it.
func dataBatchKey(req importRequest) string {
	req.SelectedRows = nil
	content, _ := json.Marshal(req)
	return importBatchKey(content)
}

// billCandidates builds the sales of a data import, one for each of its bills, with the identity of each bill's row.
func billCandidates(req importRequest) []salesCandidate {
	var candidates []salesCandidate
	batchKey := dataBatchKey(req)
	for _, bill := range dataBillsOf(req) {
		bp := dataBillOf(req, bill.rows)
		bp.PosID, bp.BillRequestID = importSaleIdentity(batchKey, bill.row)
		customer, date, total := salesPreviewFields(&bp)
		candidates = append(candidates, salesCandidate{row: bill.row, payload: &bp, customer: customer, date: date, total: total})
	}
	return candidates
}

// ── Purchase bill import ────────────────────────────────────────

func importPurchaseBills(w http.ResponseWriter, token string, req importRequest) {
	var results []billResult
	successCount := 0
	failCount := 0

	for i, row := range req.Rows {
		partName := getField(row, req.Mapping, "part_name")
		priceStr := getField(row, req.Mapping, "price")
		qtyStr := getField(row, req.Mapping, "quantity")
		discountStr := getField(row, req.Mapping, "discount")
		dateStr := getField(row, req.Mapping, "date")
		supplierIDStr := getField(row, req.Mapping, "supplier_id")

		if partName == "" && priceStr == "" {
			continue
		}

		price := parsePrice(priceStr)
		qty := helpers.ParseIntValue(qtyStr)
		if qty <= 0 {
			qty = 1
		}
		discount := cleanNumeric(discountStr)
		if discount == "" {
			discount = "0"
		}
		supplierID, _ := strconv.Atoi(supplierIDStr)

		if partName == "" {
			partName = "بند"
		}

		manualItem := models.BillManualItem{
			PartName: partName,
			Price:    formatPrice(price),
			Quantity: strconv.Itoa(qty),
		}

		// Generate a random product for the products array (backend requires it)
		bigN, _ := rand.Int(rand.Reader, big.NewInt(900000))
		randID := int(bigN.Int64()) + 100000
		productItem := models.BillProductItem{
			ID:       randID,
			PartName: partName,
			Price:    formatPrice(price),
			Quantity: strconv.Itoa(qty),
		}

		effectiveDate := ""
		if dateStr != "" {
			effectiveDate = parseDateBare(dateStr)
		}
		if effectiveDate == "" {
			effectiveDate = time.Now().Format("2006-01-02")
		}

		emptyPDF := ""
		payload := models.PurchaseBillPayload{
			StoreID:        req.StoreID,
			SupplierID:     supplierID,
			EffectiveDate:  effectiveDate,
			Products:       []models.BillProductItem{productItem},
			ManualProducts: []models.BillManualItem{manualItem},
			Discount:       discount,
			Subtotal:       price * float64(qty),
			PaymentMethod:  10,
			State:          1,
			PDFLink:        &emptyPDF,
			Attachments:    []string{},
		}

		jsonPayload, _ := json.Marshal(payload)
		log.Printf("[DATA IMPORT] PurchaseBill %d payload: %s", i+1, string(jsonPayload))

		apiReq, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/purchase_bill", bytes.NewBuffer(jsonPayload))
		apiReq.Header.Set("Content-Type", "application/json")

		resp, err := helpers.DoAuthedRequest(apiReq, token)
		if err != nil {
			results = append(results, billResult{Row: i + 1, OK: false, Error: "خطأ في الاتصال: " + err.Error()})
			failCount++
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			log.Printf("[DATA IMPORT] PurchaseBill %d failed: %d body=[%s]", i+1, resp.StatusCode, string(respBody))
			results = append(results, billResult{Row: i + 1, OK: false, Error: fmt.Sprintf("خطأ %d: %s", resp.StatusCode, truncate(string(respBody), 120))})
			failCount++
		} else {
			results = append(results, billResult{Row: i + 1, OK: true})
			successCount++
		}
	}

	helpers.APICache.Delete("purchase_bills")
	writeJSON(w, http.StatusOK, importResult{
		Total:   successCount + failCount,
		Success: successCount,
		Failed:  failCount,
		Results: results,
	})
}

// ── Product import ──────────────────────────────────────────────

func importProducts(w http.ResponseWriter, token string, req importRequest) {
	// Collect all products then submit as one batch
	type prodRow struct {
		price       int
		costPrice   int
		quantity    int
		shelfNumber string
	}

	var products []map[string]interface{}
	for _, row := range req.Rows {
		qtyStr := getField(row, req.Mapping, "quantity")
		priceStr := getField(row, req.Mapping, "price")
		costStr := getField(row, req.Mapping, "cost_price")
		shelfStr := getField(row, req.Mapping, "shelf_number")

		if priceStr == "" && qtyStr == "" {
			continue
		}

		price := int(parsePrice(priceStr))
		costPrice := int(parsePrice(costStr))
		qty := helpers.ParseIntValue(qtyStr)
		if qty <= 0 {
			qty = 1
		}

		bigN, _ := rand.Int(rand.Reader, big.NewInt(900000))
		id := int(bigN.Int64()) + 100000

		products = append(products, map[string]interface{}{
			"product_id":   id,
			"quantity":     qty,
			"price":        price,
			"cost_price":   costPrice,
			"shelf_number": shelfStr,
		})
	}

	if len(products) == 0 {
		writeJSON(w, http.StatusBadRequest, importResult{Error: "لا توجد منتجات صالحة للاستيراد"})
		return
	}

	payload := map[string]interface{}{
		"store_id": req.StoreID,
		"products": products,
	}

	jsonPayload, _ := json.Marshal(payload)
	log.Printf("[DATA IMPORT] Products payload (%d items): %s", len(products), truncate(string(jsonPayload), 500))

	apiReq, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/product", bytes.NewBuffer(jsonPayload))
	apiReq.Header.Set("Content-Type", "application/json")

	resp, err := helpers.DoAuthedRequest(apiReq, token)
	if err != nil {
		writeJSON(w, http.StatusOK, importResult{
			Total: len(products), Failed: len(products),
			Results: []billResult{{Row: 1, OK: false, Error: "خطأ في الاتصال: " + err.Error()}},
		})
		return
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[DATA IMPORT] Products batch failed: %d body=[%s]", resp.StatusCode, string(respBody))
		writeJSON(w, http.StatusOK, importResult{
			Total: len(products), Failed: len(products),
			Results: []billResult{{Row: 1, OK: false, Error: fmt.Sprintf("خطأ %d: %s", resp.StatusCode, truncate(string(respBody), 120))}},
		})
		return
	}

	helpers.APICache.Delete("products")
	writeJSON(w, http.StatusOK, importResult{
		Total:   len(products),
		Success: len(products),
		Results: []billResult{{Row: 1, OK: true}},
	})
}

// ── Helpers ─────────────────────────────────────────────────────

func getField(row []string, mapping map[string]int, field string) string {
	if idx, ok := mapping[field]; ok && idx >= 0 && idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

// parseDateToRFC3339 tries common date formats and returns RFC3339.
func parseDateToRFC3339(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return ""
}

// parseDateBare returns YYYY-MM-DD from various date formats.
func parseDateBare(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

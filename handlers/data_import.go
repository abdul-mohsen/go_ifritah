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
	Type    string            `json:"type"`    // "bills", "purchase_bills", "products"
	StoreID int               `json:"store_id"`
	Mapping map[string]int    `json:"mapping"` // field_name → column_index
	Rows    [][]string        `json:"rows"`    // all data rows (excluding header)
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

func importBills(w http.ResponseWriter, token string, req importRequest) {
	var results []billResult
	successCount := 0
	failCount := 0

	// Group rows by bill_group if mapped
	billGroupIdx, hasBillGroup := req.Mapping["bill_group"]

	type billRow struct {
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

	parseRow := func(row []string) billRow {
		br := billRow{quantity: 1}
		if idx, ok := req.Mapping["part_name"]; ok && idx < len(row) {
			br.partName = strings.TrimSpace(row[idx])
		}
		if idx, ok := req.Mapping["price"]; ok && idx < len(row) {
			br.price = parsePrice(strings.TrimSpace(row[idx]))
		}
		if idx, ok := req.Mapping["quantity"]; ok && idx < len(row) {
			q := helpers.ParseIntValue(strings.TrimSpace(row[idx]))
			if q > 0 {
				br.quantity = q
			}
		}
		if idx, ok := req.Mapping["discount"]; ok && idx < len(row) {
			br.discount = cleanNumeric(row[idx])
		}
		if idx, ok := req.Mapping["customer_name"]; ok && idx < len(row) {
			br.customer = strings.TrimSpace(row[idx])
		}
		if idx, ok := req.Mapping["customer_phone"]; ok && idx < len(row) {
			br.phone = strings.TrimSpace(row[idx])
		}
		if idx, ok := req.Mapping["vin"]; ok && idx < len(row) {
			br.vin = strings.TrimSpace(row[idx])
		}
		if idx, ok := req.Mapping["note"]; ok && idx < len(row) {
			br.note = strings.TrimSpace(row[idx])
		}
		if idx, ok := req.Mapping["date"]; ok && idx < len(row) {
			br.date = strings.TrimSpace(row[idx])
		}
		return br
	}

	buildBillPayload := func(rows []billRow) models.BillPayload {
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
			d := parseDateToRFC3339(first.date)
			if d != "" {
				bp.EffectiveDate = &d
			}
		}
		return bp
	}

	if hasBillGroup {
		// Group rows by bill_group column value
		groups := make(map[string][]billRow)
		var order []string
		for _, row := range req.Rows {
			br := parseRow(row)
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

		for i, key := range order {
			grp := groups[key]
			if len(grp) == 0 {
				continue
			}
			bp := buildBillPayload(grp)
			ok, errMsg := submitBill(token, bp, i+1)
			results = append(results, billResult{Row: i + 1, OK: ok, Error: errMsg})
			if ok {
				successCount++
			} else {
				failCount++
			}
		}
	} else {
		// Each row is a separate bill
		rowNum := 0
		for _, row := range req.Rows {
			br := parseRow(row)
			if br.partName == "" && br.price == 0 {
				continue
			}
			rowNum++
			bp := buildBillPayload([]billRow{br})
			ok, errMsg := submitBill(token, bp, rowNum)
			results = append(results, billResult{Row: rowNum, OK: ok, Error: errMsg})
			if ok {
				successCount++
			} else {
				failCount++
			}
		}
	}

	helpers.APICache.Delete("invoices_all")
	writeJSON(w, http.StatusOK, importResult{
		Total:   successCount + failCount,
		Success: successCount,
		Failed:  failCount,
		Results: results,
	})
}

func submitBill(token string, bp models.BillPayload, num int) (bool, string) {
	jsonPayload, _ := json.Marshal(bp)
	log.Printf("[DATA IMPORT] Bill %d payload: %s", num, string(jsonPayload))

	apiReq, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill", bytes.NewBuffer(jsonPayload))
	apiReq.Header.Set("Content-Type", "application/json")

	resp, err := helpers.DoAuthedRequest(apiReq, token)
	if err != nil {
		return false, "خطأ في الاتصال: " + err.Error()
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[DATA IMPORT] Bill %d failed: %d body=[%s]", num, resp.StatusCode, string(respBody))
		return false, fmt.Sprintf("خطأ %d: %s", resp.StatusCode, truncate(string(respBody), 120))
	}
	return true, ""
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

		manualItem := models.PurchaseBillProduct{
			Name:     partName,
			Price:    formatPrice(price),
			Quantity: strconv.Itoa(qty),
		}

		payload := models.PurchaseBillPayload{
			StoreID:        int32(req.StoreID),
			SupplierID:     int32(supplierID),
			Products:       []models.PurchaseBillProduct{},
			ManualProducts: []models.PurchaseBillProduct{manualItem},
			Discount:       discount,
			PaymentMethod:  10,
			State:          1,
			PaidAmount:     "0",
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

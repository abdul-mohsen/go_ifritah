package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// HandleImportBillsPage displays the CSV import form.
func HandleImportBillsPage(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stores, _ := helpers.FetchStores(token)
	if stores == nil {
		stores = []models.Store{}
	}

	helpers.Render(w, r, "import-bills", map[string]interface{}{
		"title":  "استيراد فواتير من CSV",
		"stores": stores,
	})
}

// HandleImportBillsUpload processes the uploaded CSV and submits bills one by one.
func HandleImportBillsUpload(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Parse multipart form (max 10 MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "فشل في قراءة الملف: " + err.Error()})
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "يرجى اختيار ملف CSV"})
		return
	}
	defer file.Close()

	defaultStoreID := helpers.ParseIntValue(r.FormValue("store_id"))
	if defaultStoreID == 0 {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "يرجى اختيار المخزن"})
		return
	}

	// Read CSV with flexible detection
	rawBytes, err := io.ReadAll(file)
	if err != nil {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "فشل في قراءة الملف"})
		return
	}

	// Strip BOM if present
	rawStr := string(rawBytes)
	rawStr = strings.TrimPrefix(rawStr, "\xef\xbb\xbf")

	// Detect separator: tab, semicolon, or comma
	separator := detectSeparator(rawStr)

	reader := csv.NewReader(strings.NewReader(rawStr))
	reader.Comma = separator
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // variable number of fields

	records, err := reader.ReadAll()
	if err != nil {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "فشل في تحليل CSV: " + err.Error()})
		return
	}

	if len(records) < 2 {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "الملف فارغ أو يحتوي على رأس فقط"})
		return
	}

	// Map columns from header row
	header := records[0]
	colMap := mapColumns(header)

	if colMap.partName < 0 && colMap.price < 0 {
		writeImportJSON(w, http.StatusBadRequest, importResult{
			Error: "لم يتم العثور على أعمدة مطلوبة. يجب أن يحتوي الملف على عمود اسم القطعة أو السعر على الأقل. الأعمدة المكتشفة: " + strings.Join(header, ", "),
		})
		return
	}

	// Parse rows into bill groups
	bills := parseBillRows(records[1:], colMap, defaultStoreID)

	if len(bills) == 0 {
		writeImportJSON(w, http.StatusBadRequest, importResult{Error: "لم يتم العثور على بيانات صالحة في الملف"})
		return
	}

	// Submit each bill one by one with state=3 (completed/issued)
	var results []billResult
	successCount := 0
	failCount := 0

	for i, bill := range bills {
		bill.State = 3 // completed/issued

		jsonPayload, _ := json.Marshal(bill)
		log.Printf("[CSV IMPORT] Bill %d/%d payload: %s", i+1, len(bills), string(jsonPayload))

		req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")

		resp, err := helpers.DoAuthedRequest(req, token)
		if err != nil {
			log.Printf("[CSV IMPORT] Bill %d request error: %v", i+1, err)
			results = append(results, billResult{
				Row:   i + 1,
				OK:    false,
				Error: "خطأ في الاتصال: " + err.Error(),
			})
			failCount++
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			log.Printf("[CSV IMPORT] Bill %d failed: %d body=[%s]", i+1, resp.StatusCode, string(respBody))
			results = append(results, billResult{
				Row:   i + 1,
				OK:    false,
				Error: fmt.Sprintf("خطأ %d: %s", resp.StatusCode, truncate(string(respBody), 120)),
			})
			failCount++
		} else {
			log.Printf("[CSV IMPORT] Bill %d succeeded", i+1)
			results = append(results, billResult{
				Row: i + 1,
				OK:  true,
			})
			successCount++
		}
	}

	helpers.APICache.Delete("invoices_all")

	writeImportJSON(w, http.StatusOK, importResult{
		Total:   len(bills),
		Success: successCount,
		Failed:  failCount,
		Results: results,
	})
}

// ── Column detection ────────────────────────────────────────────

// columnMap holds the detected column indices (or -1 if not found).
type columnMap struct {
	partName      int
	price         int
	quantity      int
	discount      int
	maintenance   int
	storeID       int
	customerName  int
	customerPhone int
	note          int
	vin           int
	billGroup     int // optional: group rows into separate bills
}

// mapColumns normalizes header names and maps them to known fields.
func mapColumns(header []string) columnMap {
	cm := columnMap{
		partName: -1, price: -1, quantity: -1, discount: -1,
		maintenance: -1, storeID: -1, customerName: -1,
		customerPhone: -1, note: -1, vin: -1, billGroup: -1,
	}

	for i, raw := range header {
		col := normalizeColumnName(raw)
		switch {
		// Part / product name
		case matchesAny(col, "part_name", "partname", "part", "product", "productname",
			"product_name", "item", "itemname", "item_name", "name", "description", "desc",
			"اسم", "القطعة", "اسم_القطعة", "المنتج", "الصنف", "البند", "وصف"):
			cm.partName = i
		// Price
		case matchesAny(col, "price", "unit_price", "unitprice", "amount", "cost",
			"السعر", "سعر", "المبلغ", "سعر_الوحدة"):
			cm.price = i
		// Quantity
		case matchesAny(col, "quantity", "qty", "count", "units", "الكمية", "كمية", "عدد"):
			cm.quantity = i
		// Discount
		case matchesAny(col, "discount", "disc", "خصم", "الخصم"):
			cm.discount = i
		// Maintenance cost
		case matchesAny(col, "maintenance", "maintenance_cost", "maintenancecost",
			"صيانة", "تكلفة_الصيانة"):
			cm.maintenance = i
		// Store ID
		case matchesAny(col, "store_id", "storeid", "store", "المخزن", "رقم_المخزن"):
			cm.storeID = i
		// Customer name
		case matchesAny(col, "customer", "customername", "customer_name", "client",
			"clientname", "client_name", "user_name", "username",
			"الزبون", "العميل", "اسم_الزبون", "اسم_العميل"):
			cm.customerName = i
		// Customer phone
		case matchesAny(col, "phone", "customer_phone", "customerphone", "mobile",
			"user_phone_number", "الهاتف", "الجوال", "رقم_الجوال"):
			cm.customerPhone = i
		// Note
		case matchesAny(col, "note", "notes", "comment", "remarks", "ملاحظة", "ملاحظات"):
			cm.note = i
		// VIN
		case matchesAny(col, "vin", "chassis", "شاصي", "رقم_الشاصي"):
			cm.vin = i
		// Bill group (to separate multiple items into different bills)
		case matchesAny(col, "bill", "billnumber", "bill_number", "invoice",
			"invoicenumber", "invoice_number", "group", "bill_id", "billid",
			"رقم_الفاتورة", "الفاتورة"):
			cm.billGroup = i
		}
	}

	return cm
}

// normalizeColumnName strips whitespace, punctuation, converts to lowercase ASCII.
func normalizeColumnName(s string) string {
	s = strings.TrimSpace(s)
	// Remove BOM and special chars
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return unicode.ToLower(r)
		}
		return '_'
	}, s)
	// Collapse multiple underscores
	re := regexp.MustCompile(`_+`)
	s = re.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

func matchesAny(col string, options ...string) bool {
	for _, opt := range options {
		if col == opt {
			return true
		}
	}
	return false
}

// detectSeparator sniffs the first few lines to determine the delimiter.
func detectSeparator(content string) rune {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx > 0 {
		firstLine = content[:idx]
	}

	tabs := strings.Count(firstLine, "\t")
	semis := strings.Count(firstLine, ";")
	commas := strings.Count(firstLine, ",")

	if tabs > commas && tabs > semis {
		return '\t'
	}
	if semis > commas {
		return ';'
	}
	return ','
}

// ── Row parsing ─────────────────────────────────────────────────

// parsedRow holds one parsed CSV line.
type parsedRow struct {
	partName      string
	price         float64
	quantity      int
	discount      string
	maintenance   string
	storeID       int
	customerName  string
	customerPhone string
	note          string
	vin           string
	billGroup     string
}

// parseBillRows converts CSV records into BillPayload objects.
// If a billGroup column exists, rows sharing the same group become one bill.
// Otherwise each row is its own bill.
func parseBillRows(records [][]string, cm columnMap, defaultStoreID int) []models.BillPayload {
	rows := make([]parsedRow, 0, len(records))

	for _, rec := range records {
		row := parsedRow{
			quantity: 1,
			storeID:  defaultStoreID,
		}

		if cm.partName >= 0 && cm.partName < len(rec) {
			row.partName = strings.TrimSpace(rec[cm.partName])
		}
		if cm.price >= 0 && cm.price < len(rec) {
			row.price = parsePrice(rec[cm.price])
		}
		if cm.quantity >= 0 && cm.quantity < len(rec) {
			q := helpers.ParseIntValue(strings.TrimSpace(rec[cm.quantity]))
			if q > 0 {
				row.quantity = q
			}
		}
		if cm.discount >= 0 && cm.discount < len(rec) {
			row.discount = cleanNumeric(rec[cm.discount])
		}
		if cm.maintenance >= 0 && cm.maintenance < len(rec) {
			row.maintenance = cleanNumeric(rec[cm.maintenance])
		}
		if cm.storeID >= 0 && cm.storeID < len(rec) {
			sid := helpers.ParseIntValue(strings.TrimSpace(rec[cm.storeID]))
			if sid > 0 {
				row.storeID = sid
			}
		}
		if cm.customerName >= 0 && cm.customerName < len(rec) {
			row.customerName = strings.TrimSpace(rec[cm.customerName])
		}
		if cm.customerPhone >= 0 && cm.customerPhone < len(rec) {
			row.customerPhone = strings.TrimSpace(rec[cm.customerPhone])
		}
		if cm.note >= 0 && cm.note < len(rec) {
			row.note = strings.TrimSpace(rec[cm.note])
		}
		if cm.vin >= 0 && cm.vin < len(rec) {
			row.vin = strings.TrimSpace(rec[cm.vin])
		}
		if cm.billGroup >= 0 && cm.billGroup < len(rec) {
			row.billGroup = strings.TrimSpace(rec[cm.billGroup])
		}

		// Skip empty rows
		if row.partName == "" && row.price == 0 {
			continue
		}

		rows = append(rows, row)
	}

	// Group rows into bills
	if cm.billGroup >= 0 {
		return groupRowsIntoBills(rows)
	}

	// No grouping column — each row is a separate bill
	bills := make([]models.BillPayload, 0, len(rows))
	for _, row := range rows {
		bills = append(bills, rowToBill(row))
	}
	return bills
}

// groupRowsIntoBills groups rows by billGroup value.
func groupRowsIntoBills(rows []parsedRow) []models.BillPayload {
	groups := make(map[string][]parsedRow)
	var order []string

	for _, row := range rows {
		key := row.billGroup
		if key == "" {
			key = "__single_" + fmt.Sprintf("%d", len(order))
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], row)
	}

	bills := make([]models.BillPayload, 0, len(groups))
	for _, key := range order {
		grp := groups[key]
		if len(grp) == 0 {
			continue
		}

		// Use first row's metadata for the bill
		first := grp[0]
		items := make([]models.BillManualItem, 0, len(grp))
		var total float64
		for _, row := range grp {
			name := row.partName
			if name == "" {
				name = "بند"
			}
			items = append(items, models.BillManualItem{
				PartName: name,
				Price:    formatPrice(row.price),
				Quantity: strconv.Itoa(row.quantity),
			})
			total += row.price * float64(row.quantity)
		}

		discount := first.discount
		if discount == "" {
			discount = "0"
		}
		maintenance := first.maintenance
		if maintenance == "" {
			maintenance = "0"
		}

		bills = append(bills, models.BillPayload{
			StoreID:         first.storeID,
			Products:        []models.BillProductItem{},
			ManualProducts:  items,
			TotalAmount:     total,
			Discount:        discount,
			MaintenanceCost: maintenance,
			State:           3,
			UserName:        first.customerName,
			UserPhoneNumber: first.customerPhone,
			Note:            first.note,
			VIN:             first.vin,
		})
	}

	return bills
}

// rowToBill converts a single parsed row into a BillPayload.
func rowToBill(row parsedRow) models.BillPayload {
	name := row.partName
	if name == "" {
		name = "بند"
	}

	discount := row.discount
	if discount == "" {
		discount = "0"
	}
	maintenance := row.maintenance
	if maintenance == "" {
		maintenance = "0"
	}

	return models.BillPayload{
		StoreID:  row.storeID,
		Products: []models.BillProductItem{},
		ManualProducts: []models.BillManualItem{
			{
				PartName: name,
				Price:    formatPrice(row.price),
				Quantity: strconv.Itoa(row.quantity),
			},
		},
		TotalAmount:     row.price * float64(row.quantity),
		Discount:        discount,
		MaintenanceCost: maintenance,
		State:           3,
		UserName:        row.customerName,
		UserPhoneNumber: row.customerPhone,
		Note:            row.note,
		VIN:             row.vin,
	}
}

// ── Helpers ─────────────────────────────────────────────────────

// parsePrice handles various price formats: "1,200.50", "1.200,50", "SAR 500", etc.
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Remove currency symbols and text
	currencyRe := regexp.MustCompile(`(?i)(sar|ر\.?س\.?|ريال|usd|\$|€|£)`)
	s = currencyRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)

	// Determine decimal separator
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")

	if lastComma > lastDot {
		// European format: 1.200,50 → remove dots, replace comma with dot
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	} else {
		// Standard format: 1,200.50 → remove commas
		s = strings.ReplaceAll(s, ",", "")
	}

	// Remove any remaining non-numeric chars except dot and minus
	cleanRe := regexp.MustCompile(`[^\d.\-]`)
	s = cleanRe.ReplaceAllString(s, "")

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// cleanNumeric removes non-numeric chars from a value.
func cleanNumeric(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "0"
	}
	val := parsePrice(s)
	return formatPrice(val)
}

// formatPrice formats a float as a string for the backend.
func formatPrice(f float64) string {
	if f == float64(int(f)) {
		return strconv.Itoa(int(f))
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ── Response types ──────────────────────────────────────────────

type importResult struct {
	Error   string       `json:"error,omitempty"`
	Total   int          `json:"total"`
	Success int          `json:"success"`
	Failed  int          `json:"failed"`
	Results []billResult `json:"results,omitempty"`
}

type billResult struct {
	Row   int    `json:"row"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func writeImportJSON(w http.ResponseWriter, status int, result importResult) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}

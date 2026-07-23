package helpers

import (
	"afrita/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// DerefString safely dereferences a *string, returning "" if nil.
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ParseUint64Value(value string) uint64 {
	if value == "" {
		return 0
	}
	v, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func ParseIntValue(value string) int {
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return i
}

func ParseFloatValue(value string) float64 {
	if value == "" {
		return 0
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return f
}

// FormatStringPrice formats a price value as string for the backend.
// Backend requires price, discount, maintenance_cost as strings.
func FormatStringPrice(value string) string {
	if value == "" {
		return "0"
	}
	// Validate it's a valid number, return as-is (string)
	_, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return "0"
	}
	return value
}

func BuildBillPayload(r *http.Request) models.BillPayload {
	_ = r.ParseForm()

	// Products section uses product IDs (linked products from catalog)
	// Include products_name so the backend receives the part name
	products := BuildBillProductItemsWithNames(r.Form["products_product_id"], r.Form["products_price"], r.Form["products_quantity"], r.Form["products_name"])
	// Manual section uses part names (free-text manual items)
	manualProducts := BuildBillManualItems(r.Form["manual_part_name"], r.Form["manual_price"], r.Form["manual_quantity"], r.Form["manual_part_number"])

	// Backend requires discount and maintenance_cost as STRINGS
	discount := FormatStringPrice(r.FormValue("discount"))
	maintenanceCost := FormatStringPrice(r.FormValue("maintenance_cost"))

	// All outgoing date fields go through helpers.ToBackendDate* so they carry
	// the Riyadh offset (+03:00).
	deliverDate := ToBackendDatePtr(r.FormValue("deliver_date"))
	effectiveDate := ToBackendDatePtr(r.FormValue("effective_date"))
	paymentDueDate := ToBackendDatePtr(r.FormValue("payment_due_date"))

	// client_id — optional
	var clientID *int
	if cid := ParseIntValue(r.FormValue("client_id")); cid > 0 {
		clientID = &cid
	}

	return models.BillPayload{
		StoreID:         ParseIntValue(r.FormValue("store_id")),
		Products:        products,
		ManualProducts:  manualProducts,
		TotalAmount:     ParseFloatValue(r.FormValue("total_amount")),
		Discount:        discount,
		MaintenanceCost: maintenanceCost,
		State:           ParseIntValue(r.FormValue("state")),
		VIN:             r.FormValue("vin"),
		UserName:        r.FormValue("user_name"),
		UserPhoneNumber: r.FormValue("user_phone_number"),
		Note:            r.FormValue("note"),
		EffectiveDate:   effectiveDate,
		PaymentDueDate:  paymentDueDate,
		DeliverDate:     deliverDate,
		PaymentMethod:   ParseIntValue(r.FormValue("payment_method")),
		ClientID:        clientID,
		BranchID:        ParseIntValue(r.FormValue("branch_id")),
	}
}

// BuildBillProductItems builds linked product items (with product ID).
// Backend requires price as STRING and product reference as "id".
func BuildBillProductItems(ids []string, prices []string, quantities []string) []models.BillProductItem {
	return BuildBillProductItemsWithNames(ids, prices, quantities, nil)
}

// productRow holds the parsed values for one row of parallel product form
// arrays (id/price/quantity, plus optional name and cost_price). It is
// shared by BuildBillProductItemsWithNames and BuildPurchaseBillPayload to
// avoid duplicating the index-bounds/empty-row logic.
type productRow struct {
	id        int
	name      string
	price     string
	quantity  string
	costPrice string
}

// readProductRow extracts row i from a set of parallel slices, applying
// formatting and the "0" defaults that the backend expects. Any of the
// secondary slices (names, costPrices) may be nil.
func readProductRow(i int, ids, prices, quantities, names, costPrices []string) productRow {
	row := productRow{price: "0", quantity: "0"}
	if i < len(ids) {
		row.id = ParseIntValue(ids[i])
	}
	if i < len(prices) {
		row.price = FormatStringPrice(prices[i])
	}
	if i < len(quantities) {
		if q := quantities[i]; q != "" {
			row.quantity = q
		}
	}
	if i < len(names) {
		row.name = names[i]
	}
	if i < len(costPrices) {
		row.costPrice = FormatStringPrice(costPrices[i])
	}
	return row
}

// productRowMaxLen returns the longest length across the supplied slices,
// which is the number of iterations needed to read every row.
func productRowMaxLen(slices ...[]string) int {
	m := 0
	for _, s := range slices {
		if len(s) > m {
			m = len(s)
		}
	}
	return m
}

// optionalSellingPrice returns row i of sellingPrices as a formatted price
// string pointer, or nil if the row is out of range or blank — a blank
// value means "no override," not "set the price to 0."
func optionalSellingPrice(sellingPrices []string, i int) *string {
	if i >= len(sellingPrices) {
		return nil
	}
	raw := strings.TrimSpace(sellingPrices[i])
	if raw == "" {
		return nil
	}
	formatted := FormatStringPrice(raw)
	return &formatted
}

// BuildBillProductItemsWithNames builds product items including part_name (for purchase bills).
func BuildBillProductItemsWithNames(ids []string, prices []string, quantities []string, names []string) []models.BillProductItem {
	items := make([]models.BillProductItem, 0)
	max := productRowMaxLen(ids, prices, quantities)
	for i := 0; i < max; i++ {
		row := readProductRow(i, ids, prices, quantities, names, nil)
		if row.id == 0 && row.price == "0" && row.quantity == "0" {
			continue
		}
		items = append(items, models.BillProductItem{
			ID:       row.id,
			PartName: row.name,
			Price:    row.price,
			Quantity: row.quantity,
		})
	}
	return items
}

// BuildBillManualItems builds manual (free-text) product items.
// Backend requires price as STRING and product name as "name", part number as "part_name".
func BuildBillManualItems(names []string, prices []string, quantities []string, partNumbers []string) []models.BillManualItem {
	items := make([]models.BillManualItem, 0)
	max := len(names)
	if len(prices) > max {
		max = len(prices)
	}
	if len(quantities) > max {
		max = len(quantities)
	}

	for i := 0; i < max; i++ {
		name := ""
		if i < len(names) {
			name = names[i]
		}
		price := "0"
		if i < len(prices) {
			price = FormatStringPrice(prices[i])
		}
		qtyStr := "0"
		if i < len(quantities) {
			qtyStr = quantities[i]
			if qtyStr == "" {
				qtyStr = "0"
			}
		}
		if name == "" && price == "0" && qtyStr == "0" {
			continue
		}
		partNum := ""
		if i < len(partNumbers) {
			partNum = partNumbers[i]
		}
		items = append(items, models.BillManualItem{
			PartName:   name,
			PartNumber: partNum,
			Price:      price,
			Quantity:   qtyStr,
		})
	}
	return items
}

// BuildBillItems builds legacy BillItem list (used for display/reading).
func BuildBillItems(names []string, prices []string, quantities []string) []models.BillItem {
	items := make([]models.BillItem, 0)
	max := len(names)
	if len(prices) > max {
		max = len(prices)
	}
	if len(quantities) > max {
		max = len(quantities)
	}

	for i := 0; i < max; i++ {
		name := ""
		if i < len(names) {
			name = names[i]
		}
		price := 0.0
		if i < len(prices) {
			price = ParseFloatValue(prices[i])
		}
		qty := 0
		if i < len(quantities) {
			qty = ParseIntValue(quantities[i])
		}
		if name == "" && price == 0 && qty == 0 {
			continue
		}
		items = append(items, models.BillItem{
			PartName: name,
			Price:    price,
			Quantity: qty,
		})
	}
	return items
}

// FormatFloat formats a float as string, suitable for backend API.
func FormatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

func BuildPurchaseBillPayload(r *http.Request) models.PurchaseBillPayload {
	// Use ParseMultipartForm (not ParseForm) because the purchase bill form
	// uses hx-encoding="multipart/form-data" for file uploads.
	// ParseForm does NOT parse multipart bodies, leaving r.Form empty.
	_ = r.ParseMultipartForm(32 << 20)

	// Every UI item uses products_* fields. Only a dropdown-selected item is
	// stock-tracked; typed and CSV-imported items remain manual purchase lines.
	// products_price was removed from the UI — the single cost price the user
	// enters is products_cost_price, which maps to both Price (backend subtotal
	// field) and CostPrice (inventory cost basis).
	ids := r.Form["products_product_id"]
	// products_price kept for backward compatibility with any in-flight forms
	// that still carry the old field; if absent, products_cost_price is used.
	prices := r.Form["products_price"]
	if len(prices) == 0 {
		prices = r.Form["products_cost_price"]
	}
	quantities := r.Form["products_quantity"]
	names := r.Form["products_part_name"]
	costPrices := r.Form["products_cost_price"]
	shelfNumbers := r.Form["products_shelf_number"]
	trackStocks := r.Form["products_track_stock"]
	sellingPrices := r.Form["products_selling_price"]

	var products []models.BillProductItem
	var manualProducts []models.BillManualItem

	max := productRowMaxLen(ids, prices, quantities)
	for i := 0; i < max; i++ {
		row := readProductRow(i, ids, prices, quantities, names, costPrices)
		if row.name == "" && row.price == "0" {
			continue
		}
		trackStock := i < len(trackStocks) && trackStocks[i] == "true"
		shelfNumber := ""
		if i < len(shelfNumbers) {
			shelfNumber = shelfNumbers[i]
		}
		if trackStock && row.id > 0 {
			products = append(products, models.BillProductItem{
				ID:           row.id,
				PartName:     row.name,
				Price:        row.price,
				Quantity:     row.quantity,
				CostPrice:    row.costPrice,
				ShelfNumber:  shelfNumber,
				TrackStock:   true,
				SellingPrice: optionalSellingPrice(sellingPrices, i),
			})
			continue
		}
		manualProducts = append(manualProducts, models.BillManualItem{
			PartName:    row.name,
			Price:       row.price,
			Quantity:    row.quantity,
			CostPrice:   row.costPrice,
			ShelfNumber: shelfNumber,
		})
	}

	// Manual products from separate manual_* form fields
	manualNames := r.Form["manual_part_name"]
	manualPrices := r.Form["manual_price"]
	manualQtys := r.Form["manual_quantity"]
	manualPartNums := r.Form["manual_part_number"]
	manualMax := len(manualNames)
	if len(manualPrices) > manualMax {
		manualMax = len(manualPrices)
	}
	if len(manualQtys) > manualMax {
		manualMax = len(manualQtys)
	}
	for i := 0; i < manualMax; i++ {
		name := ""
		if i < len(manualNames) {
			name = manualNames[i]
		}
		price := "0"
		if i < len(manualPrices) {
			price = FormatStringPrice(manualPrices[i])
		}
		qtyStr := "0"
		if i < len(manualQtys) {
			qtyStr = manualQtys[i]
			if qtyStr == "" {
				qtyStr = "0"
			}
		}
		if name == "" && price == "0" && qtyStr == "0" {
			continue
		}
		partNum := ""
		if i < len(manualPartNums) {
			partNum = manualPartNums[i]
		}
		manualProducts = append(manualProducts, models.BillManualItem{
			PartName:   name,
			PartNumber: partNum,
			Price:      price,
			Quantity:   qtyStr,
		})
	}

	// Ensure non-nil slices so JSON encodes as [] instead of null
	if products == nil {
		products = []models.BillProductItem{}
	}
	if manualProducts == nil {
		manualProducts = []models.BillManualItem{}
	}

	supplierID := ParseIntValue(r.FormValue("supplier_id"))

	// All outgoing date fields use ToBackendDate* (RFC3339 + Riyadh offset).
	effectiveDate := ToBackendDate(r.FormValue("payment_date"))
	paymentDueDate := ToBackendDatePtr(r.FormValue("payment_due_date"))
	deliverDate := ToBackendDatePtr(r.FormValue("deliver_date"))

	// pdf_link — send null if no file uploaded
	var pdfLink *string
	if pl := r.FormValue("pdf_link"); pl != "" {
		pdfLink = &pl
	}

	// attachments — collect from form
	attachments := r.Form["attachments"]
	if attachments == nil {
		attachments = []string{}
	}

	return models.PurchaseBillPayload{
		StoreID:                ParseIntValue(r.FormValue("store_id")),
		MerchantID:             supplierID,
		SupplierID:             supplierID,
		SupplierSequenceNumber: ParseUint64Value(r.FormValue("supplier_sequance_number")),
		State:                  1,
		EffectiveDate:          effectiveDate,
		Products:               products,
		ManualProducts:         manualProducts,
		Discount:               FormatStringPrice(r.FormValue("discount")),
		Subtotal:               ParseFloatValue(r.FormValue("total_amount")),
		PaymentDueDate:         paymentDueDate,
		DeliverDate:            deliverDate,
		PaymentMethod:          ParseIntValue(r.FormValue("payment_method")),
		PaidAmount:             FormatStringPrice(r.FormValue("paid_amount")),
		PDFLink:                pdfLink,
		Attachments:            attachments,
	}
}

//nolint:unused // TODO: Wire to client CRUD operations
func buildClientPayload(form map[string][]string) map[string]interface{} {
	getValue := func(key string) string {
		if vals, ok := form[key]; ok && len(vals) > 0 {
			return vals[0]
		}
		return ""
	}

	return map[string]interface{}{
		"name":         getValue("name"),
		"phone_number": getValue("phone_number"),
		"address":      getValue("address"),
		"email":        getValue("email"),
		"tax_number":   getValue("tax_number"),
	}
}

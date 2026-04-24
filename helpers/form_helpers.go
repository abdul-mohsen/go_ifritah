package helpers

import (
	"afrita/models"
	"fmt"
	"net/http"
	"strconv"
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

// DateToRFC3339 converts a bare YYYY-MM-DD date to RFC3339 format.
// Returns nil if the input is empty.
func DateToRFC3339(date string) *string {
	if date == "" {
		return nil
	}
	if len(date) == 10 {
		rfc := date + "T00:00:00Z"
		return &rfc
	}
	return &date
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

	// Bill backend parses effective_date, payment_due_date with time.RFC3339
	// deliver_date is *time.Time (JSON binding expects RFC3339)
	deliverDate := DateToRFC3339(r.FormValue("deliver_date"))
	effectiveDate := DateToRFC3339(r.FormValue("effective_date"))
	paymentDueDate := DateToRFC3339(r.FormValue("payment_due_date"))

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

// BuildBillProductItemsWithNames builds product items including part_name (for purchase bills).
func BuildBillProductItemsWithNames(ids []string, prices []string, quantities []string, names []string) []models.BillProductItem {
	items := make([]models.BillProductItem, 0)
	max := len(ids)
	if len(prices) > max {
		max = len(prices)
	}
	if len(quantities) > max {
		max = len(quantities)
	}

	for i := 0; i < max; i++ {
		id := 0
		if i < len(ids) {
			id = ParseIntValue(ids[i])
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
		if id == 0 && price == "0" && qtyStr == "0" {
			continue
		}
		name := ""
		if i < len(names) {
			name = names[i]
		}
		items = append(items, models.BillProductItem{
			ID:       id,
			PartName: name,
			Price:    price,
			Quantity: qtyStr,
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

	// Catalog products from products_* form fields
	ids := r.Form["products_product_id"]
	prices := r.Form["products_price"]
	quantities := r.Form["products_quantity"]
	names := r.Form["products_part_name"]
	costPrices := r.Form["products_cost_price"]

	var products []models.BillProductItem
	var manualProducts []models.BillManualItem

	max := len(ids)
	if len(prices) > max {
		max = len(prices)
	}
	if len(quantities) > max {
		max = len(quantities)
	}

	for i := 0; i < max; i++ {
		id := 0
		if i < len(ids) {
			id = ParseIntValue(ids[i])
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
		name := ""
		if i < len(names) {
			name = names[i]
		}
		costPrice := ""
		if i < len(costPrices) {
			costPrice = FormatStringPrice(costPrices[i])
		}

		if id == 0 && price == "0" && qtyStr == "0" {
			continue
		}

		if id != 0 {
			// Linked product — goes to "products"
			products = append(products, models.BillProductItem{
				ID:        id,
				PartName:  name,
				Price:     price,
				Quantity:  qtyStr,
				CostPrice: costPrice,
			})
		} else {
			// Manual product from products_* with id=0 — goes to "manual_products"
			manualProducts = append(manualProducts, models.BillManualItem{
				PartName:  name,
				Price:     price,
				Quantity:  qtyStr,
				CostPrice: costPrice,
			})
		}
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

	// All date fields: send as RFC3339 (ISO 8601) — "2024-01-15T00:00:00Z"
	effectiveDate := DateToRFC3339(r.FormValue("payment_date"))

	// payment_due_date and deliver_date use RFC3339
	paymentDueDate := DateToRFC3339(r.FormValue("payment_due_date"))
	deliverDate := DateToRFC3339(r.FormValue("deliver_date"))

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
		EffectiveDate:          DerefString(effectiveDate),
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

package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

// coerceString extracts a string from an interface{} value
func coerceString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int(val)) {
			return strconv.Itoa(int(val))
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return ""
	}
}

// coerceFloat extracts a float64 from an interface{} value
func coerceFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// coerceInt extracts an int from an interface{} value
func coerceInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

// HandleOrders displays the orders list page
func HandleOrders(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	orders, err := helpers.FetchOrders(token)
	if err != nil {
		orders = []map[string]interface{}{}
	}

	// Translate order statuses to Arabic
	for i := range orders {
		if status, ok := orders[i]["status"].(string); ok {
			switch status {
			case "pending":
				orders[i]["status"] = "قيد الانتظار"
			case "completed":
				orders[i]["status"] = "مكتمل"
			case "canceled":
				orders[i]["status"] = "ملغي"
			case "processing":
				orders[i]["status"] = "قيد المعالجة"
			}
		}
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		filtered := make([]map[string]interface{}, 0)
		for _, o := range orders {
			num, _ := o["number"].(string)
			client, _ := o["client"].(string)
			if helpers.ContainsInsensitive(num, query) || helpers.ContainsInsensitive(client, query) {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedOrders, pagination := helpers.PaginateSlice(orders, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "orders", map[string]interface{}{
		"title":      "الطلبات",
		"orders":     pagedOrders,
		"query":      query,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
	})
}

// HandleAddOrder displays the add order form
func HandleAddOrder(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "add-order", map[string]interface{}{
		"title": "إضافة طلب",
	})
}

// HandleCreateOrder creates a new order
func HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	storeID := helpers.ParseIntValue(r.FormValue("store_id"))
	if storeID == 0 {
		storeID = 1
	}

	// Build products array from form
	partNames := r.Form["part_name[]"]
	quantities := r.Form["quantity[]"]
	prices := r.Form["price[]"]
	productIDs := r.Form["product_id[]"]

	products := make([]map[string]interface{}, 0)
	for i := range partNames {
		if partNames[i] == "" {
			continue
		}
		p := map[string]interface{}{
			"name":     partNames[i],
			"quantity": "1",
			"price":    "0",
		}
		if i < len(quantities) && quantities[i] != "" {
			p["quantity"] = quantities[i]
		}
		if i < len(prices) && prices[i] != "" {
			p["price"] = prices[i]
		}
		if i < len(productIDs) && productIDs[i] != "" {
			p["product_id"] = helpers.ParseIntValue(productIDs[i])
		}
		products = append(products, p)
	}

	payload := map[string]interface{}{
		"store_id":        storeID,
		"sequence_number": r.FormValue("sequence_number"),
		"customer_name":   r.FormValue("customer_name"),
		"note":            r.FormValue("note"),
		"products":        products,
	}
	jsonPayload, _ := json.Marshal(payload)
	log.Printf("[CREATE ORDER] Payload: %s", string(jsonPayload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/order", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[CREATE ORDER] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في إنشاء الطلب")
		return
	}

	helpers.APICache.Delete("orders")
	helpers.WriteSuccessRedirect(w, "/dashboard/orders", "تم إنشاء الطلب بنجاح")
}

// HandleDeleteOrder deletes an order
func HandleDeleteOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/order/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("orders")
	helpers.WriteSuccessRedirect(w, "/dashboard/orders", "تم حذف الطلب بنجاح")
}

// HandleOrderDetail displays order details
func HandleOrderDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	raw, err := helpers.FetchOrderDetail(token, id)
	if err != nil {
		log.Printf("[ORDER DETAIL] fetch error: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "فشل في تحميل بيانات الطلب")
		return
	}

	// Build order map matching template field names
	order := map[string]interface{}{
		"ID":             coerceInt(raw["id"]),
		"SequenceNumber": coerceString(raw["sequence_number"]),
		"CustomerName":   coerceString(raw["customer_name"]),
		"ClientName":     coerceString(raw["client_name"]),
		"StoreName":      coerceString(raw["store_name"]),
		"Total":          coerceFloat(raw["total"]),
		"Note":           coerceString(raw["note"]),
		"Status":         coerceString(raw["status"]),
	}

	// Build items array if present
	if items, ok := raw["products"].([]interface{}); ok {
		orderItems := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				qty := coerceFloat(m["quantity"])
				price := coerceFloat(m["price"])
				orderItems = append(orderItems, map[string]interface{}{
					"PartName":  coerceString(m["name"]),
					"Quantity":  qty,
					"UnitPrice": price,
					"LineTotal": qty * price,
				})
			}
		}
		order["Items"] = orderItems
	}

	// Translate status to Arabic
	statusAr := ""
	switch order["Status"] {
	case "pending":
		statusAr = "قيد الانتظار"
	case "completed":
		statusAr = "مكتمل"
	case "canceled":
		statusAr = "ملغي"
	case "processing":
		statusAr = "قيد المعالجة"
	default:
		statusAr = coerceString(order["Status"])
	}

	helpers.Render(w, r, "order-detail", map[string]interface{}{
		"title":     "تفاصيل الطلب",
		"order":     order,
		"status_ar": statusAr,
	})
}

// HandleEditOrder displays the edit order form
func HandleEditOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	raw, err := helpers.FetchOrderDetail(token, id)
	if err != nil {
		log.Printf("[EDIT ORDER] fetch error: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "فشل في تحميل بيانات الطلب")
		return
	}

	order := map[string]interface{}{
		"ID":             coerceInt(raw["id"]),
		"SequenceNumber": coerceString(raw["sequence_number"]),
		"CustomerName":   coerceString(raw["customer_name"]),
		"ClientName":     coerceString(raw["client_name"]),
		"Total":          coerceFloat(raw["total"]),
		"Status":         coerceString(raw["status"]),
		"Note":           coerceString(raw["note"]),
	}

	helpers.Render(w, r, "edit-order", map[string]interface{}{
		"title": "تعديل الطلب",
		"order": order,
	})
}

// HandleUpdateOrder updates an existing order
func HandleUpdateOrder(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	payload := map[string]interface{}{
		"sequence_number": r.FormValue("number"),
		"customer_name":   r.FormValue("client"),
		"status":          r.FormValue("status"),
		"note":            r.FormValue("notes"),
	}
	if t := r.FormValue("total"); t != "" {
		payload["total"] = t
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/order/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[UPDATE ORDER] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في تحديث الطلب")
		return
	}

	helpers.APICache.Delete("orders")
	helpers.WriteSuccessRedirect(w, "/dashboard/orders", "تم تحديث الطلب بنجاح")
}

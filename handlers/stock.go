package handlers

import (
	"afrita/helpers"
	"afrita/models"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// HandleStockAdjustments renders the stock adjustment page with product list.
func HandleStockAdjustments(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	products, _ := helpers.FetchProducts(token)
	if products == nil {
		products = []models.Product{}
	}

	stores, _ := helpers.FetchStores(token)
	if stores == nil {
		stores = []models.Store{}
	}

	helpers.Render(w, r, "stock-adjustments", map[string]interface{}{
		"title":    "تسوية المخزون",
		"products": products,
		"stores":   stores,
	})
}

// HandleCreateStockAdjustment processes a manual stock adjustment.
func HandleCreateStockAdjustment(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	_ = r.ParseForm()

	productIDStr := r.FormValue("product_id")
	storeIDStr := r.FormValue("store_id")
	quantityStr := r.FormValue("quantity_change")
	reason := r.FormValue("reason")
	note := r.FormValue("note")

	productID, err := strconv.Atoi(productIDStr)
	if err != nil || productID <= 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "يرجى اختيار المنتج")
		return
	}

	quantity, err := strconv.Atoi(quantityStr)
	if err != nil || quantity == 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "الكمية غير صالحة")
		return
	}

	storeID, err := strconv.Atoi(storeIDStr)
	if err != nil || storeID <= 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "يرجى اختيار المخزن")
		return
	}

	if reason == "" {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "يرجى تحديد سبب التسوية")
		return
	}

	adj := models.StockAdjustRequest{
		ProductID:      productID,
		StoreID:        storeID,
		QuantityChange: quantity,
		Reason:         reason,
		Note:           note,
	}

	if err := helpers.SubmitStockAdjustment(token, adj); err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "فشل في تسوية المخزون: "+err.Error())
		return
	}

	// Invalidate product cache since quantities changed
	helpers.APICache.Delete("products")
	helpers.WriteSuccessRedirect(w, "/dashboard/stock/adjustments", "تم تسوية المخزون بنجاح")
}

// HandleProductStockMovements returns stock movements for a product (HTMX partial).
func HandleProductStockMovements(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	movements, err := helpers.FetchStockMovements(token, id)
	if err != nil {
		// Return empty table on error (backend may not have stock system yet)
		movements = []models.StockMovement{}
	}

	helpers.RenderPartial(w, "stock-movements", map[string]interface{}{
		"movements": movements,
	})
}

// HandleStockCheck checks stock availability for invoice items (HTMX/JSON endpoint).
func HandleStockCheck(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	var items []models.StockCheckItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "بيانات غير صالحة"})
		return
	}

	result, err := helpers.CheckStockAvailability(token, items)
	if err != nil {
		// If stock check fails (backend doesn't support it yet), allow the sale
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.StockCheckResponse{
			Enforcement:   models.StockEnforcementDisable,
			AllSufficient: true,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleStockEnforcement returns the current enforcement mode (JSON).
func HandleStockEnforcement(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	mode, err := helpers.FetchStockEnforcement(token)
	if err != nil {
		mode = models.StockEnforcementDisable
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.StockEnforcementResponse{Mode: mode})
}

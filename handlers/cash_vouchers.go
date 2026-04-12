package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"afrita/config"
	"afrita/helpers"
	"afrita/resources"

	"github.com/gorilla/mux"
)

// HandleCashVouchers renders the cash vouchers list page.
func HandleCashVouchers(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")
	voucherType := r.URL.Query().Get("type")
	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	if page < 0 {
		page = 0
	}
	if perPage < 1 {
		perPage = 10
	}

	vouchers, err := helpers.FetchCashVouchers(token, 1, 10000, query, voucherType)
	if err != nil {
		log.Printf("[CASH VOUCHERS] Fetch error: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	displayVouchers := make([]map[string]interface{}, 0)
	for _, cv := range vouchers {
		statusKey, statusClass := helpers.CashVoucherStatusByState(cv.State)
		statusLabel := resources.L(statusKey)

		typeLabel := resources.L("cash_voucher_type." + cv.VoucherType)

		dateStr := cv.EffectiveDate
		if len(dateStr) >= 10 {
			dateStr = dateStr[:10]
		}

		displayVouchers = append(displayVouchers, map[string]interface{}{
			"id":             cv.ID,
			"voucher_number": cv.VoucherNumber,
			"voucher_type":   cv.VoucherType,
			"type_label":     typeLabel,
			"amount":         fmt.Sprintf("%.2f", cv.Amount),
			"date":           dateStr,
			"payment_method": cv.PaymentMethod,
			"recipient_name": cv.RecipientName,
			"state":          cv.State,
			"status":         statusLabel,
			"status_class":   statusClass,
		})
	}

	pagedVouchers, pagination := helpers.PaginateSlice(displayVouchers, page, perPage)

	// Add order numbers after pagination
	offset := page * perPage
	for i := range pagedVouchers {
		pagedVouchers[i]["order"] = offset + i + 1
	}

	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "cash-vouchers", map[string]interface{}{
		"title":        resources.L("cash_voucher.list_title"),
		"vouchers":     pagedVouchers,
		"pagination":   pagination,
		"prev_page":    prevPage,
		"next_page":    nextPage,
		"query":        query,
		"voucher_type": voucherType,
	})
}

// HandleAddCashVoucher renders the create cash voucher form.
func HandleAddCashVoucher(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stores, _ := helpers.FetchStores(token)
	suppliers, _ := helpers.FetchSuppliers(token)

	defaultStoreID := 0
	if len(stores) > 0 {
		defaultStoreID = stores[0].ID
	}

	helpers.Render(w, r, "add-cash-voucher", map[string]interface{}{
		"title":            resources.L("cash_voucher.add_title"),
		"stores":           stores,
		"suppliers":        suppliers,
		"default_store_id": defaultStoreID,
	})
}

// HandleCreateCashVoucher creates a new cash voucher.
func HandleCreateCashVoucher(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	payload := helpers.BuildCashVoucherPayload(r)

	amountVal, _ := strconv.ParseFloat(payload.Amount, 64)
	if amountVal <= 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.L("cash_voucher.amount_required"))
		return
	}

	jsonPayload, _ := json.Marshal(payload)
	log.Printf("[CREATE CASH VOUCHER] Payload: %s", string(jsonPayload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/cash_voucher", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[CREATE CASH VOUCHER] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, resources.L("cash_voucher.create_error"))
		return
	}

	helpers.APICache.DeletePrefix("cash_vouchers")
	helpers.WriteSuccessRedirect(w, "/dashboard/cash-vouchers", resources.L("cash_voucher.create_success"))
}

// HandleGetCashVoucher shows details for a cash voucher.
func HandleGetCashVoucher(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	raw, err := helpers.FetchCashVoucherDetail(token, id)
	if err != nil {
		log.Printf("[CASH VOUCHER DETAIL] Fetch error for ID %s: %v", id, err)
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, resources.L("cash_voucher.not_found"))
		return
	}

	// Resolve store name
	stores, _ := helpers.FetchStores(token)
	storeName := ""
	storeID := 0
	if v, ok := raw["store_id"]; ok {
		storeID = int(helpers.SafeFloat(v))
	}
	for _, s := range stores {
		if s.ID == storeID {
			storeName = s.Name
			break
		}
	}

	// State info
	state := int(helpers.SafeFloat(raw["state"]))
	statusKey, statusClass := helpers.CashVoucherStatusByState(state)
	statusLabel := resources.L(statusKey)

	// Voucher type label
	voucherType := ""
	if v, ok := raw["voucher_type"].(string); ok {
		voucherType = v
	}
	typeLabel := resources.L("cash_voucher_type." + voucherType)

	// Payment method label
	paymentMethod := ""
	if v, ok := raw["payment_method"].(string); ok {
		paymentMethod = v
	}
	methodLabel := resources.L("cash_voucher_method." + paymentMethod)

	// Recipient type label
	recipientType := ""
	if v, ok := raw["recipient_type"].(string); ok {
		recipientType = v
	}
	recipientTypeLabel := resources.L("cash_voucher_recipient." + recipientType)

	// Format date
	dateStr := ""
	if v, ok := raw["effective_date"].(string); ok && len(v) >= 10 {
		dateStr = v[:10]
	}

	// Amount
	amount := helpers.SafeFloat(raw["amount"])

	helpers.Render(w, r, "cash-voucher-detail", map[string]interface{}{
		"title":                resources.L("cash_voucher.detail_title"),
		"voucher":              raw,
		"id":                   id,
		"voucher_number":       int(helpers.SafeFloat(raw["voucher_number"])),
		"voucher_type":         voucherType,
		"type_label":           typeLabel,
		"date":                 dateStr,
		"amount":               fmt.Sprintf("%.2f", amount),
		"payment_method":       paymentMethod,
		"method_label":         methodLabel,
		"state":                state,
		"status":               statusLabel,
		"status_class":         statusClass,
		"recipient_type":       recipientType,
		"recipient_type_label": recipientTypeLabel,
		"recipient_name":       raw["recipient_name"],
		"reference_type":       raw["reference_type"],
		"reference_id":         raw["reference_id"],
		"description":          raw["description"],
		"note":                 raw["note"],
		"bank_name":            raw["bank_name"],
		"bank_account":         raw["bank_account"],
		"transaction_reference": raw["transaction_reference"],
		"store_name":           storeName,
		"store_id":             storeID,
	})
}

// HandleEditCashVoucher renders the edit form for a cash voucher (draft only).
func HandleEditCashVoucher(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	raw, err := helpers.FetchCashVoucherDetail(token, id)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, resources.L("cash_voucher.not_found"))
		return
	}

	// Only draft vouchers can be edited
	state := int(helpers.SafeFloat(raw["state"]))
	if state != 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.L("cash_voucher.update_error"))
		return
	}

	stores, _ := helpers.FetchStores(token)
	suppliers, _ := helpers.FetchSuppliers(token)

	// Format date for input field
	dateStr := ""
	if v, ok := raw["effective_date"].(string); ok && len(v) >= 10 {
		dateStr = v[:10]
	}

	helpers.Render(w, r, "edit-cash-voucher", map[string]interface{}{
		"title":     resources.L("cash_voucher.edit_title"),
		"voucher":   raw,
		"id":        id,
		"date":      dateStr,
		"amount":    fmt.Sprintf("%.2f", helpers.SafeFloat(raw["amount"])),
		"stores":    stores,
		"suppliers": suppliers,
	})
}

// HandleUpdateCashVoucher updates an existing cash voucher (draft only).
func HandleUpdateCashVoucher(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	payload := helpers.BuildCashVoucherPayload(r)

	amountVal, _ := strconv.ParseFloat(payload.Amount, 64)
	if amountVal <= 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.L("cash_voucher.amount_required"))
		return
	}

	jsonPayload, _ := json.Marshal(payload)
	log.Printf("[UPDATE CASH VOUCHER] ID=%s Payload: %s", id, string(jsonPayload))

	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/cash_voucher/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[UPDATE CASH VOUCHER] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, resources.L("cash_voucher.update_error"))
		return
	}

	helpers.APICache.DeletePrefix("cash_vouchers")
	helpers.WriteSuccessRedirect(w, "/dashboard/cash-vouchers/"+id, resources.L("cash_voucher.update_success"))
}

// HandleDeleteCashVoucher deletes a draft cash voucher.
func HandleDeleteCashVoucher(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/cash_voucher/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[DELETE CASH VOUCHER] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, resources.L("cash_voucher.delete_error"))
		return
	}

	helpers.APICache.DeletePrefix("cash_vouchers")
	helpers.WriteSuccessRedirect(w, "/dashboard/cash-vouchers", resources.L("cash_voucher.delete_success"))
}

// HandleApproveCashVoucher approves a draft cash voucher.
func HandleApproveCashVoucher(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	if err := helpers.ApproveCashVoucher(token, id); err != nil {
		log.Printf("[APPROVE CASH VOUCHER] Error: %v", err)
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.L("cash_voucher.approve_error"))
		return
	}

	helpers.APICache.DeletePrefix("cash_vouchers")
	helpers.WriteSuccessRedirect(w, "/dashboard/cash-vouchers/"+id, resources.L("cash_voucher.approve_success"))
}

// HandlePostCashVoucher posts an approved cash voucher (irreversible).
func HandlePostCashVoucher(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	if err := helpers.PostCashVoucher(token, id); err != nil {
		log.Printf("[POST CASH VOUCHER] Error: %v", err)
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.L("cash_voucher.post_error"))
		return
	}

	helpers.APICache.DeletePrefix("cash_vouchers")
	helpers.WriteSuccessRedirect(w, "/dashboard/cash-vouchers/"+id, resources.L("cash_voucher.post_success"))
}

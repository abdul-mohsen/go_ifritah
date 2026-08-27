package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

// HandleStores displays the stores list page
func HandleStores(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")

	stores, err := helpers.FetchStoresList(token, helpers.ListOpts{
		Page:    0,
		PerPage: 10000,
		Query:   query,
	})
	if err != nil {
		stores = []models.Store{}
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedStores, pagination := helpers.PaginateSlice(stores, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "stores", map[string]interface{}{
		"title":      "المخازن",
		"stores":     pagedStores,
		"query":      query,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
	})
}

// HandleAddStore displays the add store form
func HandleAddStore(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	branches, _ := helpers.FetchBranches(token)
	// Optional ?branch_id=N preselect — used by the ZATCA "add store" deep-link.
	selectedBranch := 0
	if b := r.URL.Query().Get("branch_id"); b != "" {
		selectedBranch, _ = strconv.Atoi(b)
	}
	helpers.Render(w, r, "add-store", map[string]interface{}{
		"title":           "إضافة مخزن",
		"branches":        branches,
		"selected_branch": selectedBranch,
	})
}

// findStoreByID fetches all stores (cached) and returns the one matching the given ID.
func findStoreByID(token string, id string) (models.Store, bool) {
	stores, err := helpers.FetchStores(token)
	if err != nil {
		return models.Store{}, false
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return models.Store{}, false
	}
	for _, s := range stores {
		if s.ID == idInt {
			return s, true
		}
	}
	return models.Store{}, false
}

// HandleStoreDetail displays store details
func HandleStoreDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	store, found := findStoreByID(token, id)
	if !found {
		store = models.Store{ID: helpers.ParseIntValue(id), Name: "مخزن #" + id}
	}

	helpers.Render(w, r, "store-detail", map[string]interface{}{
		"title": "تفاصيل المخزن",
		"store": store,
	})
}

// HandleEditStore displays the edit store form
func HandleEditStore(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	idInt, _ := strconv.Atoi(id)
	store, err := helpers.FetchStoreByID(token, idInt)
	if err != nil || store.ID == 0 {
		// Fallback to list lookup so the page still renders
		if s, found := findStoreByID(token, id); found {
			store = s
		} else {
			store = models.Store{ID: idInt}
		}
	}
	branches, _ := helpers.FetchBranches(token)
	selectedBranch := 0
	if store.BranchID != nil {
		selectedBranch = *store.BranchID
	}

	helpers.Render(w, r, "edit-store", map[string]interface{}{
		"title":           "تعديل المخزن",
		"store":           store,
		"branches":        branches,
		"selected_branch": selectedBranch,
	})
}

// HandleCreateStore creates a new store
func HandleCreateStore(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseMultipartForm(32 << 20)
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم المخزن"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": "إضافة مخزن",
		}, errs, oldValues)
		helpers.Render(w, r, "add-store", data)
		return
	}

	payload := buildStorePayload(r)
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/store", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "فشل في إنشاء المتجر")
		return
	}

	helpers.APICache.Delete("stores")
	helpers.WriteSuccessRedirect(w, "/dashboard/stores", "تم إنشاء المتجر بنجاح")
}

// HandleUpdateStore updates an existing store
func HandleUpdateStore(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseMultipartForm(32 << 20)
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم المخزن"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": "تعديل المخزن",
			"store": models.Store{ID: helpers.ParseIntValue(id)},
		}, errs, oldValues)
		helpers.Render(w, r, "edit-store", data)
		return
	}

	payload := buildStorePayload(r)
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/store/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في تحديث المتجر")
		return
	}

	helpers.APICache.Delete("stores")
	helpers.WriteSuccessRedirect(w, "/dashboard/stores", "تم تحديث المتجر بنجاح")
}

// HandleDeleteStore deletes a store
func HandleDeleteStore(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/store/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("stores")
	helpers.WriteSuccessRedirect(w, "/dashboard/stores", "تم حذف المتجر بنجاح")
}

// buildStorePayload extracts store fields from the form, including the full
// national-address breakdown so that ZATCA + branch UIs can read a single
// canonical address per store.
func buildStorePayload(r *http.Request) map[string]interface{} {
	payload := map[string]interface{}{
		"name":              r.FormValue("name"),
		"building_number":   r.FormValue("building_number"),
		"street_name":       r.FormValue("street_name"),
		"district":          r.FormValue("district"),
		"city":              r.FormValue("city"),
		"region":            r.FormValue("region"),
		"postal_code":       r.FormValue("postal_code"),
		"additional_number": r.FormValue("additional_number"),
		"unit_number":       r.FormValue("unit_number"),
		"country":           r.FormValue("country"),
		"address_name":      r.FormValue("address_name"),
	}
	if branchStr := r.FormValue("branch_id"); branchStr != "" {
		if v, err := strconv.Atoi(branchStr); err == nil {
			payload["branch_id"] = v
		}
	}
	return payload
}

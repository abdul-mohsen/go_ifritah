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

	stores, err := helpers.FetchStores(token)
	if err != nil {
		stores = []models.Store{}
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		filtered := make([]models.Store, 0)
		for _, s := range stores {
			if helpers.ContainsInsensitive(s.Name, query) {
				filtered = append(filtered, s)
			}
		}
		stores = filtered
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedStores, pagination := helpers.PaginateSlice(stores, page, perPage)
	prevPage := 0
	nextPage := 0
	if pagination.Page > 1 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages {
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
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "add-store", map[string]interface{}{
		"title": "إضافة مخزن",
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

	store, found := findStoreByID(token, id)
	if !found {
		store = models.Store{ID: helpers.ParseIntValue(id), Name: ""}
	}

	helpers.Render(w, r, "edit-store", map[string]interface{}{
		"title": "تعديل المخزن",
		"store": store,
	})
}

// HandleCreateStore creates a new store
func HandleCreateStore(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
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

	payload := map[string]string{"name": r.FormValue("name")}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/store", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("stores")
	helpers.WriteSuccessRedirect(w, "/dashboard/stores", "تم إنشاء المتجر بنجاح")
}

// HandleUpdateStore updates an existing store
func HandleUpdateStore(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
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

	payload := map[string]string{"name": r.FormValue("name")}
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

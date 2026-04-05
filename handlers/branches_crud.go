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
	"afrita/models"

	"github.com/gorilla/mux"
)

// HandleBranches displays the branches list page
func HandleBranches(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	branches, err := helpers.FetchBranches(token)
	if err != nil {
		branches = []models.Branch{}
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		filtered := make([]models.Branch, 0)
		for _, b := range branches {
			if helpers.ContainsInsensitive(b.Name, query) || helpers.ContainsInsensitive(b.Address, query) || helpers.ContainsInsensitive(b.Phone, query) {
				filtered = append(filtered, b)
			}
		}
		branches = filtered
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedBranches, pagination := helpers.PaginateSlice(branches, page, perPage)
	prevPage := 0
	nextPage := 0
	if pagination.Page > 1 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "branches", map[string]interface{}{
		"title":      "الفروع",
		"branches":   pagedBranches,
		"query":      query,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
	})
}

// HandleAddBranch displays the add branch form
func HandleAddBranch(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "add-branch", map[string]interface{}{
		"title": "إضافة فرع",
	})
}

// findBranchByID fetches all branches (cached) and returns the one matching the given ID.
func findBranchByID(token string, id string) (models.Branch, bool) {
	branches, err := helpers.FetchBranches(token)
	if err != nil {
		return models.Branch{}, false
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return models.Branch{}, false
	}
	for _, b := range branches {
		if b.ID == idInt {
			return b, true
		}
	}
	return models.Branch{}, false
}

// HandleBranchDetail displays branch details
func HandleBranchDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	branch, found := findBranchByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, "الفرع غير موجود")
		return
	}

	helpers.Render(w, r, "branch-detail", map[string]interface{}{
		"title":  "تفاصيل الفرع",
		"branch": branch,
	})
}

// HandleEditBranch displays the edit branch form
func HandleEditBranch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	branch, found := findBranchByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, "الفرع غير موجود")
		return
	}

	helpers.Render(w, r, "edit-branch", map[string]interface{}{
		"title":  "تعديل الفرع",
		"branch": branch,
	})
}

// HandleCreateBranch creates a new branch
func HandleCreateBranch(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم الفرع"},
		{Field: "location", Value: r.FormValue("location"), Required: true, MinLen: 2, MaxLen: 200, Label: "الموقع"},
		{Field: "phone", Value: r.FormValue("phone"), Pattern: helpers.PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم جوال سعودي يبدأ بـ 05 ويتكون من 10 أرقام"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "location", "phone"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": "إضافة فرع",
		}, errs, oldValues)
		helpers.Render(w, r, "add-branch", data)
		return
	}

	payload := map[string]interface{}{
		"name":    r.FormValue("name"),
		"address": r.FormValue("location"),
		"phone":   r.FormValue("phone"),
	}
	if mgr := r.FormValue("manager_id"); mgr != "" {
		payload["manager_id"] = helpers.ParseIntValue(mgr)
	}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/branch", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[CREATE BRANCH] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في إنشاء الفرع")
		return
	}

	helpers.APICache.Delete("branches")
	helpers.WriteSuccessRedirect(w, "/dashboard/branches", "تم إنشاء الفرع بنجاح")
}

// HandleUpdateBranch updates an existing branch
func HandleUpdateBranch(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم الفرع"},
		{Field: "location", Value: r.FormValue("location"), Required: true, MinLen: 2, MaxLen: 200, Label: "الموقع"},
		{Field: "phone", Value: r.FormValue("phone"), Pattern: helpers.PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم جوال سعودي يبدأ بـ 05 ويتكون من 10 أرقام"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "location", "phone"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": "تعديل الفرع",
			"branch": models.Branch{
				ID:      helpers.ParseIntValue(id),
				Name:    r.FormValue("name"),
				Address: r.FormValue("location"),
				Phone:   r.FormValue("phone"),
			},
		}, errs, oldValues)
		helpers.Render(w, r, "edit-branch", data)
		return
	}

	payload := map[string]interface{}{
		"name":    r.FormValue("name"),
		"address": r.FormValue("location"),
		"phone":   r.FormValue("phone"),
	}
	if mgr := r.FormValue("manager_id"); mgr != "" {
		payload["manager_id"] = helpers.ParseIntValue(mgr)
	}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/branch/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[UPDATE BRANCH] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في تحديث الفرع")
		return
	}

	helpers.APICache.Delete("branches")
	helpers.WriteSuccessRedirect(w, "/dashboard/branches", "تم تحديث الفرع بنجاح")
}

// HandleDeleteBranch deletes a branch
func HandleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/branch/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[DELETE BRANCH] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في حذف الفرع")
		return
	}

	helpers.APICache.Delete("branches")
	helpers.WriteSuccessRedirect(w, "/dashboard/branches", "تم حذف الفرع بنجاح")
}

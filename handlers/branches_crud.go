package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"afrita/resources"

	"github.com/gorilla/mux"
)

// HandleBranches displays the branches list page
func HandleBranches(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	query := r.URL.Query().Get("q")
	typed := helpers.TypedListFilters("branches", r.URL.Query())

	// Search/sort are 100% backend-driven on this branch — no FE post-filter.
	branches, err := helpers.FetchBranchesList(token, helpers.ListOpts{
		Page:    0,
		PerPage: 10000,
		Query:   query,
		Typed:   typed,
	})
	if err != nil {
		branches = []models.Branch{}
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedBranches, pagination := helpers.PaginateSlice(branches, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "branches", map[string]interface{}{
		"title":      resources.T(lang, "branch.list_title"),
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
	lang := helpers.GetLang(r)
	helpers.Render(w, r, "add-branch", map[string]interface{}{
		"title": resources.T(lang, "branch.add_title"),
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
	lang := helpers.GetLang(r)

	branch, found := findBranchByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, resources.T(lang, "branch.not_found"))
		return
	}

	linkedStore, hasStore := helpers.FetchBranchLinkedStore(token, branch.ID)

	helpers.Render(w, r, "branch-detail", map[string]interface{}{
		"title":        resources.T(lang, "branch.detail_title"),
		"branch":       branch,
		"linked_store": linkedStore,
		"has_store":    hasStore,
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
	lang := helpers.GetLang(r)

	branch, found := findBranchByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, resources.T(lang, "branch.not_found"))
		return
	}

	linkedStore, hasStore := helpers.FetchBranchLinkedStore(token, branch.ID)

	helpers.Render(w, r, "edit-branch", map[string]interface{}{
		"title":        resources.T(lang, "branch.edit_title"),
		"branch":       branch,
		"linked_store": linkedStore,
		"has_store":    hasStore,
	})
}

// HandleCreateBranch creates a new branch
func HandleCreateBranch(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: resources.T(lang, "branch.label.name")},
		{Field: "location", Value: r.FormValue("location"), Required: true, MinLen: 2, MaxLen: 200, Label: resources.T(lang, "branch.label.location")},
		{Field: "phone", Value: r.FormValue("phone"), Pattern: helpers.PatternSaudiPhone, Label: resources.T(lang, "client.label.phone"), PatternMsg: resources.T(lang, "validation.saudi_phone_detailed")},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "location", "phone"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": resources.T(lang, "branch.add_title"),
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
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, resources.T(lang, "branch.create_error"))
		return
	}

	helpers.APICache.Delete("branches")
	helpers.WriteSuccessRedirect(w, "/dashboard/branches", resources.T(lang, "branch.create_success"))
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
	lang := helpers.GetLang(r)

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: resources.T(lang, "branch.label.name")},
		{Field: "phone", Value: r.FormValue("phone"), Pattern: helpers.PatternSaudiPhone, Label: resources.T(lang, "client.label.phone"), PatternMsg: resources.T(lang, "validation.saudi_phone_detailed")},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "location", "phone"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": resources.T(lang, "branch.edit_title"),
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
		"name":  r.FormValue("name"),
		"phone": r.FormValue("phone"),
	}
	if loc := strings.TrimSpace(r.FormValue("location")); loc != "" {
		payload["address"] = loc
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
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, resources.T(lang, "branch.update_error"))
		return
	}

	helpers.APICache.Delete("branches")
	helpers.WriteSuccessRedirect(w, "/dashboard/branches", resources.T(lang, "branch.update_success"))
}

// HandleDeleteBranch deletes a branch
func HandleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

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
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, resources.T(lang, "branch.delete_error"))
		return
	}

	helpers.APICache.Delete("branches")
	helpers.WriteSuccessRedirect(w, "/dashboard/branches", resources.T(lang, "branch.delete_success"))
}

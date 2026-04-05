package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"afrita/config"
	"afrita/helpers"
)

// HandleVerifyVIN performs VIN lookup and renders result partial.
func HandleVerifyVIN(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	vin := r.URL.Query().Get("vin")
	if vin == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "VIN is required")
		return
	}

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/vin/car/csv/"+url.PathEscape(vin), nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		fmt.Fprintf(w, "VIN lookup failed: %d", resp.StatusCode)
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	helpers.RenderPartial(w, "vin-result", data)
}

// HandlePartsSearch renders the parts search page.
func HandlePartsSearch(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "parts-search", map[string]interface{}{
		"title": "بحث القطع",
	})
}

// HandlePartsSearchResults renders parts search results partial.
func HandlePartsSearchResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := r.FormValue("query")
	vin := r.FormValue("vin")
	if query == "" {
		fmt.Fprint(w, "")
		return
	}

	var req *http.Request
	payload, _ := json.Marshal(map[string]string{"query": query})
	if vin != "" {
		req, _ = http.NewRequest("POST", config.BackendDomain+"/api/v2/vin/part/"+url.PathEscape(vin), bytes.NewBuffer(payload))
	} else {
		req, _ = http.NewRequest("POST", config.BackendDomain+"/api/v2/part/", bytes.NewBuffer(payload))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	data := map[string]interface{}{
		"results": results,
	}
	helpers.RenderPartial(w, "parts-results", data)
}

// HandlePartsSearchJSON returns parts search results as JSON (for AJAX dropdowns).
// POST body: query=<search term>  — returns [{oem_number, id, type, url, link}, ...]
func HandlePartsSearchJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := r.FormValue("query")
	payload, _ := json.Marshal(map[string]string{"query": query})
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/part/", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer resp.Body.Close()

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil || results == nil {
		_ = json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	_ = json.NewEncoder(w).Encode(results)
}

// HandleProductsSearchJSON returns product catalog results as JSON for invoice dropdowns.
func HandleProductsSearchJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := strings.ToLower(strings.TrimSpace(r.FormValue("query")))
	products, err := helpers.FetchProducts(token)
	if err != nil || products == nil {
		_ = json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	type productResult struct {
		ID       int     `json:"id"`
		Name     string  `json:"name"`
		Price    float64 `json:"price"`
		Quantity int     `json:"quantity"`
	}

	var results []productResult
	for _, p := range products {
		if query != "" {
			idStr := fmt.Sprintf("%d", p.ID)
			if !strings.Contains(strings.ToLower(p.PartName), query) && !strings.Contains(idStr, query) {
				continue
			}
		}
		results = append(results, productResult{
			ID:       p.ID,
			Name:     p.PartName,
			Price:    p.Price,
			Quantity: p.Quantity,
		})
		if len(results) >= 50 {
			break
		}
	}

	if results == nil {
		results = []productResult{}
	}
	_ = json.NewEncoder(w).Encode(results)
}

// HandleCarsSearch renders the cars search page.
func HandleCarsSearch(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "cars-search", map[string]interface{}{
		"title": "بحث السيارات",
	})
}

// HandleCarsSearchResults renders cars search results partial.
func HandleCarsSearchResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		fmt.Fprint(w, "")
		return
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/cars/search?query="+url.QueryEscape(query), nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "")
		return
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	data := map[string]interface{}{
		"results": results,
	}
	helpers.RenderPartial(w, "cars-results", data)
}

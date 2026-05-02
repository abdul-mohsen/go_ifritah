package handlers

// Branch store address handlers — bridge between the per-branch ZATCA UI
// and the `store` table.
//
// Why this exists:
//   The zatca_integration daemon's onboarding flow reads the taxpayer's
//   physical-address fields with the following precedence:
//
//      Street/Building/District/Postal/Country  →  branch_zatca_config (preferred)
//                                                  store          (fallback)
//      City                                     →  store ONLY  (no fallback)
//
//   `branch_zatca_config` does not have a `city` column, so the UI must
//   write City directly to the linked store. We co-locate the rest of the
//   address fields here too so the store row stays consistent and the
//   fallback always works even if branch_zatca_config rows are wiped.
//
// API surface (frontend → backend proxy):
//   GET  /api/branch/{id}/store-address
//   PUT  /api/branch/{id}/store-address
//
// Body (PUT):
//   {
//     "street_name":      "string",
//     "building_number":  "string",
//     "district":         "string",
//     "city":             "string",   // required for ZATCA onboarding
//     "postal_code":      "string",
//     "country":          "string"    // optional, defaults to "SA"
//   }
//
// Behavior on PUT:
//   1. GET /api/v2/branch/{id} → take stores[0].id (the "first" linked store).
//   2. If found  → PUT /api/v2/store/{storeId} preserving its `name`.
//   3. If absent → POST /api/v2/store with name="<branch.name> - Main",
//      branch_id=<id>, and the address fields.
//
//   Subsequent edits will always re-find the same first store.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

type branchStoreAddressBody struct {
	StreetName     string `json:"street_name"`
	BuildingNumber string `json:"building_number"`
	District       string `json:"district"`
	City           string `json:"city"`
	PostalCode     string `json:"postal_code"`
	Country        string `json:"country"`
}

// fetchBranchDetail calls backend GET /api/v2/branch/{id} and returns the
// raw `detail` map. The response shape is:
//
//	{ "detail": { "id":..., "name":..., "stores":[{id,name},...], ... } }
func fetchBranchDetail(sessionID, branchID string) (map[string]interface{}, int, error) {
	return fetchDetailJSON(sessionID,
		config.BackendDomain+"/api/v2/branch/"+branchID,
		"branch fetch")
}

func firstStoreID(detail map[string]interface{}) (int, string, bool) {
	stores, ok := detail["stores"].([]interface{})
	if !ok || len(stores) == 0 {
		return 0, "", false
	}
	first, ok := stores[0].(map[string]interface{})
	if !ok {
		return 0, "", false
	}
	idF, _ := first["id"].(float64)
	name, _ := first["name"].(string)
	return int(idF), name, idF > 0
}

func fetchStoreDetail(sessionID string, storeID int) (map[string]interface{}, int, error) {
	return fetchDetailJSON(sessionID,
		fmt.Sprintf("%s/api/v2/store/%d", config.BackendDomain, storeID),
		"store fetch")
}

// fetchDetailJSON does an authed GET, parses the JSON body, and returns the
// nested `detail` object if the BE wraps the row in one. Used by the branch
// and store fetchers, which share an identical envelope shape.
func fetchDetailJSON(sessionID, url, opLabel string) (map[string]interface{}, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("%s %d: %s", opLabel, resp.StatusCode, string(body))
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, resp.StatusCode, err
	}
	if d, ok := raw["detail"].(map[string]interface{}); ok {
		return d, resp.StatusCode, nil
	}
	return raw, resp.StatusCode, nil
}

// HandleGetBranchStoreAddress returns the address fields of the branch's
// first linked store. If the branch has no store yet, returns an empty
// payload (HTTP 200) so the UI can show blank inputs.
func HandleGetBranchStoreAddress(w http.ResponseWriter, r *http.Request) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		writeBranchStoreErr(w, http.StatusUnauthorized, "غير مصرح - يرجى تسجيل الدخول")
		return
	}
	branchID := mux.Vars(r)["id"]
	if branchID == "" {
		writeBranchStoreErr(w, http.StatusBadRequest, "branch id required")
		return
	}

	detail, sc, err := fetchBranchDetail(sessionID, branchID)
	if err != nil {
		log.Printf("[BRANCH-STORE-ADDR] GET branch error: %v", err)
		if sc == 0 {
			sc = http.StatusBadGateway
		}
		writeBranchStoreErr(w, sc, "فشل جلب بيانات الفرع")
		return
	}

	storeID, _, found := firstStoreID(detail)
	if !found {
		writeBranchStoreJSON(w, http.StatusOK, map[string]interface{}{
			"detail": branchStoreAddressBody{Country: "SA"},
			"linked": false,
		})
		return
	}

	store, sc2, err := fetchStoreDetail(sessionID, storeID)
	if err != nil {
		log.Printf("[BRANCH-STORE-ADDR] GET store error: %v", err)
		if sc2 == 0 {
			sc2 = http.StatusBadGateway
		}
		writeBranchStoreErr(w, sc2, "فشل جلب بيانات المخزن")
		return
	}

	body := branchStoreAddressBody{
		StreetName:     str(store["street_name"]),
		BuildingNumber: str(store["building_number"]),
		District:       str(store["district"]),
		City:           str(store["city"]),
		PostalCode:     str(store["postal_code"]),
		Country:        defaultStr(str(store["country"]), "SA"),
	}
	writeBranchStoreJSON(w, http.StatusOK, map[string]interface{}{
		"detail":   body,
		"linked":   true,
		"store_id": storeID,
	})
}

// HandleUpdateBranchStoreAddress writes the address to the first linked
// store, creating one if necessary. City is required.
func HandleUpdateBranchStoreAddress(w http.ResponseWriter, r *http.Request) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		writeBranchStoreErr(w, http.StatusUnauthorized, "غير مصرح - يرجى تسجيل الدخول")
		return
	}
	branchID := mux.Vars(r)["id"]
	if branchID == "" {
		writeBranchStoreErr(w, http.StatusBadRequest, "branch id required")
		return
	}

	var body branchStoreAddressBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBranchStoreErr(w, http.StatusBadRequest, "صيغة الطلب غير صالحة")
		return
	}
	body.StreetName = strings.TrimSpace(body.StreetName)
	body.BuildingNumber = strings.TrimSpace(body.BuildingNumber)
	body.District = strings.TrimSpace(body.District)
	body.City = strings.TrimSpace(body.City)
	body.PostalCode = strings.TrimSpace(body.PostalCode)
	body.Country = strings.TrimSpace(body.Country)
	if body.Country == "" {
		body.Country = "SA"
	}
	if body.City == "" {
		writeBranchStoreErr(w, http.StatusBadRequest, "المدينة مطلوبة")
		return
	}

	detail, sc, err := fetchBranchDetail(sessionID, branchID)
	if err != nil {
		log.Printf("[BRANCH-STORE-ADDR] PUT fetch branch: %v", err)
		if sc == 0 {
			sc = http.StatusBadGateway
		}
		writeBranchStoreErr(w, sc, "فشل جلب بيانات الفرع")
		return
	}

	storeID, _, hasStore := firstStoreID(detail)

	if hasStore {
		// Preserve store name; backend's PUT requires it.
		store, _, _ := fetchStoreDetail(sessionID, storeID)
		name := str(store["name"])
		if name == "" {
			name = strings.TrimSpace(str(detail["name"])) + " - Main"
		}
		payload := map[string]interface{}{
			"name":            name,
			"branch_id":       intFromAny(detail["id"]),
			"street_name":     body.StreetName,
			"building_number": body.BuildingNumber,
			"district":        body.District,
			"city":            body.City,
			"postal_code":     body.PostalCode,
			"country":         body.Country,
		}
		if err := proxyJSON(sessionID, "PUT",
			fmt.Sprintf("%s/api/v2/store/%d", config.BackendDomain, storeID),
			payload, w); err != nil {
			log.Printf("[BRANCH-STORE-ADDR] PUT store: %v", err)
		}
		return
	}

	// No store yet — create one tied to this branch.
	branchName := strings.TrimSpace(str(detail["name"]))
	if branchName == "" {
		branchName = "Branch " + branchID
	}
	payload := map[string]interface{}{
		"name":            branchName + " - Main",
		"branch_id":       intFromAny(detail["id"]),
		"street_name":     body.StreetName,
		"building_number": body.BuildingNumber,
		"district":        body.District,
		"city":            body.City,
		"postal_code":     body.PostalCode,
		"country":         body.Country,
	}
	if err := proxyJSON(sessionID, "POST",
		config.BackendDomain+"/api/v2/store", payload, w); err != nil {
		log.Printf("[BRANCH-STORE-ADDR] POST store: %v", err)
	}
}

// ── small helpers ────────────────────────────────────────────────────

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func intFromAny(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	}
	return 0
}

func writeBranchStoreJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeBranchStoreErr(w http.ResponseWriter, status int, msg string) {
	writeBranchStoreJSON(w, status, map[string]string{"detail": msg})
}

func proxyJSON(sessionID, method, url string, payload map[string]interface{}, w http.ResponseWriter) error {
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(buf))
	if err != nil {
		writeBranchStoreErr(w, http.StatusInternalServerError, "internal error")
		return err
	}
	req.Header.Set(headerContentType, mimeJSON)
	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			writeBranchStoreErr(w, http.StatusUnauthorized, "انتهت الجلسة")
			return err
		}
		writeBranchStoreErr(w, http.StatusBadGateway, "فشل الاتصال بالخادم")
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	return nil
}

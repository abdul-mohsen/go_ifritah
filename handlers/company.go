package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"afrita/config"
	"afrita/helpers"
	"afrita/resources"
)

// Company info handlers — proxy to backend `/api/v2/company`.
//
// Maps to the `company` MySQL table:
//   id, state, name, vat_number, vat_registration_number,
//   commercial_registration_number, name_ar, business_category
//
// `company.name_ar` (Arabic legal name) is REQUIRED on every ZATCA e-invoice
// per the integration daemon at github.com/abdul-mohsen/zatca_integration —
// see Taxpayer.CompanyNameAR and SellerInfo.NameAR.
//
// ────────────────────────────────────────────────────────────────────────────
// BACKEND CHANGES NEEDED on github.com/abdul-mohsen/ifritah-go (dev branch):
//
//   1. Add to pkg/db/queries/company.sql:
//        -- name: GetCompany :one
//        SELECT id, name, COALESCE(name_ar,'') AS name_ar,
//               COALESCE(vat_registration_number,'') AS vat_registration_number,
//               COALESCE(commercial_registration_number,'') AS commercial_registration_number,
//               COALESCE(business_category,'Supply activities') AS business_category
//        FROM company WHERE id = ? LIMIT 1;
//
//        -- name: UpdateCompany :exec
//        UPDATE company
//           SET name = ?, name_ar = ?,
//               vat_registration_number = ?, commercial_registration_number = ?,
//               business_category = ?
//         WHERE id = ?;
//
//   2. Register handlers in main.go authorized group:
//        authorized.GET("company",  h.GetCompany)
//        authorized.PUT("company",  h.UpdateCompany)
//
//   3. Resolve company_id from JWT claims (same as h.getUserCompany used by
//      the branch handlers) so a tenant can only read/update its own row.
//
//   4. Validation in UpdateCompany:
//        - name_ar: required, non-empty trimmed (CHECK in DB allows NULL but
//          ZATCA invoices reject empty CompanyNameAR).
//        - vat_registration_number: 15 digits, regex `^3\d{13}3$` (already
//          enforced by chk_vat_registration_number CHECK in dev_schema).
//
// Until the backend exposes these endpoints, this proxy returns whatever the
// backend returns (likely 404). The frontend handles that gracefully and
// keeps the form usable so QA can validate the UI flow.
// ────────────────────────────────────────────────────────────────────────────

// writeCompanyErr writes an error envelope as JSON. Centralizing this
// keeps the proxy handlers small and avoids duplicating the
// header/encode/return triplet at every error branch.
func writeCompanyErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": msg})
}

// dispatchCompany performs the authed backend request and copies its
// response back to the caller. It maps transport errors to the same
// 401/502 envelopes used by every branch of the proxy handlers.
func dispatchCompany(w http.ResponseWriter, sessionID, lang string, backendReq *http.Request) (*http.Response, bool) {
	backendReq.Header.Set(headerContentType, mimeJSON)
	resp, err := helpers.DoAuthedRequestWithRetry(backendReq, sessionID)
	if err != nil {
		log.Printf("[COMPANY] %s error: %v", backendReq.Method, err)
		if helpers.IsUnauthorizedError(err) {
			writeCompanyErr(w, http.StatusUnauthorized, resources.T(lang, "general.session_expired"))
		} else {
			writeCompanyErr(w, http.StatusBadGateway, resources.T(lang, "general.server_connection_error"))
		}
		return nil, false
	}
	return resp, true
}

// HandleGetCompany proxies GET /api/v2/company to the backend.
func HandleGetCompany(w http.ResponseWriter, r *http.Request) {
	lang := helpers.GetLang(r)
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		writeCompanyErr(w, http.StatusUnauthorized, resources.T(lang, "auth.unauthorized_login"))
		return
	}

	backendReq, err := http.NewRequest("GET", config.BackendDomain+"/api/v2/company", nil)
	if err != nil {
		writeCompanyErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp, ok := dispatchCompany(w, sessionID, lang, backendReq)
	if !ok {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(resp.StatusCode)

	// Backend returns either the bare company row or {"detail": {...}}.
	// Normalize so the frontend can always read `payload.detail`.
	if resp.StatusCode == http.StatusOK && len(body) > 0 {
		var raw map[string]interface{}
		if json.Unmarshal(body, &raw) == nil {
			if _, hasDetail := raw["detail"]; hasDetail {
				w.Write(body)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"detail": raw})
			return
		}
	}
	w.Write(body)
}

// HandleUpdateCompany proxies PUT /api/v2/company to the backend.
func HandleUpdateCompany(w http.ResponseWriter, r *http.Request) {
	lang := helpers.GetLang(r)
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		writeCompanyErr(w, http.StatusUnauthorized, resources.T(lang, "auth.unauthorized_login"))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCompanyErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	backendReq, err := http.NewRequest("PUT", config.BackendDomain+"/api/v2/company", bytes.NewBuffer(body))
	if err != nil {
		writeCompanyErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp, ok := dispatchCompany(w, sessionID, lang, backendReq)
	if !ok {
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

// writePDFErrorJSON writes a JSON error response for PDF failures.
// The frontend JS fetches the PDF via fetch() and checks for this JSON on error.
func writePDFErrorJSON(w http.ResponseWriter, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "pdf_unavailable",
		"message": "تعذر تحميل ملف PDF، يرجى المحاولة لاحقاً",
	})
}

// HandleBillPDF streams the invoice PDF from the backend.
// If the backend returns an error, a JSON error is returned so the frontend JS can show a toast.
func HandleBillPDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	sessionID, err := r.Cookie("session_id")
	if err != nil {
		helpers.HandleUnauthorized(w, r)
		return
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/bill/pdf/"+id, nil)
	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID.Value)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		writePDFErrorJSON(w, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		helpers.HandleUnauthorized(w, r)
		return
	}

	if resp.StatusCode != http.StatusOK {
		writePDFErrorJSON(w, resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=invoice-"+id+".pdf")
	reader := http.MaxBytesReader(w, resp.Body, 50<<20)
	defer reader.Close()
	_, _ = io.Copy(w, reader)
}

// HandleCreditBill displays credit bill details.
func HandleCreditBill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Credit bills use the same backend format, so use FetchBillDetail with the credit endpoint
	// For now, fetch raw and parse like the regular bill
	invoice, _, _, _, err := helpers.FetchCreditBillDetail(token, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	status, statusClass := helpers.InvoiceStatus(invoice)
	invoiceType := helpers.InvoiceTypeLabel(invoice)

	helpers.Render(w, r, "credit-invoice-detail", map[string]interface{}{
		"title":         "إشعار دائن",
		"invoice":       invoice,
		"total_display": fmt.Sprintf("%.2f", invoice.Total),
		"status":        status,
		"status_class":  statusClass,
		"type":          invoiceType,
	})
}

// HandleCreditBillPDF streams the credit bill PDF from the backend.
func HandleCreditBillPDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	sessionID, err := r.Cookie("session_id")
	if err != nil {
		helpers.HandleUnauthorized(w, r)
		return
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/bill/credit/pdf/"+id, nil)
	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID.Value)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		writePDFErrorJSON(w, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		helpers.HandleUnauthorized(w, r)
		return
	}

	if resp.StatusCode != http.StatusOK {
		writePDFErrorJSON(w, resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=credit-bill-"+id+".pdf")
	reader := http.MaxBytesReader(w, resp.Body, 50<<20)
	defer reader.Close()
	_, _ = io.Copy(w, reader)
}

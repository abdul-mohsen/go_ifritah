package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"afrita/config"
	"afrita/models"
)

// FetchCashVouchers fetches cash vouchers from backend with pagination, search, and type filter.
func FetchCashVouchers(token string, page, perPage int, query, voucherType string) ([]models.CashVoucher, error) {
	cacheKey := fmt.Sprintf("cash_vouchers_%d_%d_%s_%s", page, perPage, query, voucherType)
	if cached, ok := APICache.Get(cacheKey); ok {
		if v, ok := cached.([]models.CashVoucher); ok {
			return v, nil
		}
	}

	payload := map[string]interface{}{
		"page_number": page - 1,
		"page_size":   perPage,
	}
	if query != "" {
		payload["query"] = query
	}
	if voucherType != "" {
		payload["voucher_type"] = voucherType
	}

	body, _ := json.Marshal(payload)
	log.Printf("🔵 [API REQUEST] POST %s/api/v2/cash_voucher/all", config.BackendDomain)

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/cash_voucher/all", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("🔴 [CASH VOUCHER] API error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("🔴 [CASH VOUCHER] Backend status: %d", resp.StatusCode)
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	vouchers, err := decodeCashVoucherList(bodyBytes)
	if err != nil {
		return nil, err
	}

	APICache.Set(cacheKey, vouchers, 1*time.Minute)
	return vouchers, nil
}

// FetchCashVoucherDetail fetches a single cash voucher as raw map.
func FetchCashVoucherDetail(token string, id string) (map[string]interface{}, error) {
	log.Printf("🔵 [API REQUEST] GET %s/api/v2/cash_voucher/%s", config.BackendDomain, id)

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/cash_voucher/"+id, nil)
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BuildCashVoucherPayload builds a CashVoucherPayload from form data.
func BuildCashVoucherPayload(r *http.Request) models.CashVoucherPayload {
	if err := r.ParseForm(); err != nil {
		log.Printf("[CASH VOUCHER] Form parse error: %v", err)
	}

	amount := strings.TrimSpace(r.FormValue("amount"))
	recipientID := ParseIntValue(r.FormValue("recipient_id"))
	referenceID := ParseIntValue(r.FormValue("reference_id"))
	storeID := ParseIntValue(r.FormValue("store_id"))
	branchID := ParseIntValue(r.FormValue("branch_id"))
	if branchID == 0 {
		branchID = 1
	}

	effectiveDate := r.FormValue("effective_date")
	if effectiveDate != "" && len(effectiveDate) == 10 {
		effectiveDate = effectiveDate + "T00:00:00Z"
	}

	return models.CashVoucherPayload{
		VoucherType:          r.FormValue("voucher_type"),
		EffectiveDate:        effectiveDate,
		Amount:               amount,
		PaymentMethod:        r.FormValue("payment_method"),
		RecipientType:        r.FormValue("recipient_type"),
		RecipientID:          recipientID,
		RecipientName:        r.FormValue("recipient_name"),
		ReferenceType:        r.FormValue("reference_type"),
		ReferenceID:          referenceID,
		Description:          r.FormValue("description"),
		Note:                 r.FormValue("note"),
		BankName:             r.FormValue("bank_name"),
		BankAccount:          r.FormValue("bank_account"),
		TransactionReference: r.FormValue("transaction_reference"),
		StoreID:              storeID,
		BranchID:             branchID,
	}
}

// CashVoucherStatusByState returns the label key and CSS class for a cash voucher state.
func CashVoucherStatusByState(state int) (string, string) {
	switch state {
	case 0:
		return "cash_voucher_status.draft", "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
	case 1:
		return "cash_voucher_status.approved", "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300"
	case 2:
		return "cash_voucher_status.posted", "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
	default:
		return "cash_voucher_status.draft", "bg-gray-100 text-gray-800"
	}
}

// ApproveCashVoucher sends an approve request for a cash voucher.
func ApproveCashVoucher(token string, id string) error {
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/cash_voucher/"+id+"/approve", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// PostCashVoucher sends a post request for a cash voucher (irreversible).
func PostCashVoucher(token string, id string) error {
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/cash_voucher/"+id+"/post", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// decodeCashVoucherList parses cash voucher list response with string-number coercion.
func decodeCashVoucherList(body []byte) ([]models.CashVoucher, error) {
	var rawList []map[string]interface{}
	if err := json.Unmarshal(body, &rawList); err != nil {
		var wrapper map[string]json.RawMessage
		if wErr := json.Unmarshal(body, &wrapper); wErr != nil {
			return nil, err
		}
		for _, key := range []string{"data", "items", "results"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &rawList) == nil {
					break
				}
			}
		}
		if rawList == nil {
			return nil, fmt.Errorf("unsupported response shape for cash vouchers")
		}
	}

	vouchers := make([]models.CashVoucher, 0, len(rawList))
	for _, m := range rawList {
		cv := models.CashVoucher{}
		if v, ok := CoerceFloat(m["id"]); ok {
			cv.ID = int(v)
		}
		if v, ok := CoerceFloat(m["voucher_number"]); ok {
			cv.VoucherNumber = int(v)
		}
		if v, ok := m["voucher_type"].(string); ok {
			cv.VoucherType = v
		}
		if v, ok := CoerceFloat(m["amount"]); ok {
			cv.Amount = v
		}
		if v, ok := m["payment_method"].(string); ok {
			cv.PaymentMethod = v
		}
		if v, ok := CoerceFloat(m["state"]); ok {
			cv.State = int(v)
		}
		if v, ok := m["recipient_type"].(string); ok {
			cv.RecipientType = v
		}
		if v, ok := CoerceFloat(m["recipient_id"]); ok {
			cv.RecipientID = int(v)
		}
		if v, ok := m["recipient_name"].(string); ok {
			cv.RecipientName = v
		}
		if v, ok := m["reference_type"].(string); ok {
			cv.ReferenceType = v
		}
		if v, ok := CoerceFloat(m["reference_id"]); ok {
			cv.ReferenceID = int(v)
		}
		if v, ok := m["description"].(string); ok {
			cv.Description = v
		}
		if v, ok := m["note"].(string); ok {
			cv.Note = v
		}
		if v, ok := m["bank_name"].(string); ok {
			cv.BankName = v
		}
		if v, ok := m["bank_account"].(string); ok {
			cv.BankAccount = v
		}
		if v, ok := m["transaction_reference"].(string); ok {
			cv.TransactionReference = v
		}
		if v, ok := CoerceFloat(m["store_id"]); ok {
			cv.StoreID = int(v)
		}

		// Parse effective_date
		if ed, ok := m["effective_date"].(string); ok {
			cv.EffectiveDate = ed
		}

		vouchers = append(vouchers, cv)
	}
	return vouchers, nil
}

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"afrita/helpers"
	"afrita/models"
)

const maxPurchaseBillBulkImportBody = 10 << 20

// HandleBulkPurchaseBillImport validates and forwards each imported purchase
// bill independently so one rejected record does not hide other outcomes.
func HandleBulkPurchaseBillImport(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPurchaseBillBulkImportBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var payloads []models.PurchaseBillPayload
	if err := decoder.Decode(&payloads); err != nil {
		writePurchaseBillBulkImportError(w, http.StatusBadRequest, "invalid purchase bill import payload")
		return
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		writePurchaseBillBulkImportError(w, http.StatusBadRequest, "invalid purchase bill import payload")
		return
	}
	if len(payloads) == 0 {
		writePurchaseBillBulkImportError(w, http.StatusBadRequest, "purchase bill import must contain at least one bill")
		return
	}

	result := bulkImportResponse{
		Total:   len(payloads),
		Results: make([]bulkImportLineResult, 0, len(payloads)),
	}
	for i := range payloads {
		itemResult := bulkImportLineResult{Index: i}
		if err := validatePurchaseBillImportPayload(&payloads[i]); err != nil {
			itemResult.Error = err.Error()
			result.Failed++
			result.Results = append(result.Results, itemResult)
			continue
		}

		if err := postImportedBill(token, "/api/v2/purchase_bill", payloads[i]); err != nil {
			itemResult.Error = err.Error()
			result.Failed++
		} else {
			itemResult.OK = true
			result.Success++
		}
		result.Results = append(result.Results, itemResult)
	}

	if result.Success > 0 {
		helpers.APICache.Delete("purchase_bills")
	}
	writeBulkImportJSON(w, http.StatusOK, result)
}

func writePurchaseBillBulkImportError(w http.ResponseWriter, status int, message string) {
	writeBulkImportJSON(w, status, bulkImportResponse{
		Error:   message,
		Results: []bulkImportLineResult{},
	})
}

func validatePurchaseBillImportPayload(payload *models.PurchaseBillPayload) error {
	if payload.StoreID <= 0 {
		return fmt.Errorf("store_id must be positive")
	}
	if payload.SupplierID <= 0 {
		return fmt.Errorf("supplier_id must be positive")
	}
	if payload.SupplierSequenceNumber == 0 {
		return fmt.Errorf("supplier_sequence_number must be positive")
	}
	if payload.PaymentMethod <= 0 {
		return fmt.Errorf("payment_method must be positive")
	}

	effectiveDate := helpers.ToBackendDate(payload.EffectiveDate)
	if effectiveDate == "" {
		return fmt.Errorf("effective_date must be a valid date")
	}
	payload.EffectiveDate = effectiveDate

	if len(payload.ManualProducts) == 0 {
		return fmt.Errorf("at least one manual product is required")
	}
	for i := range payload.ManualProducts {
		product := &payload.ManualProducts[i]
		if strings.TrimSpace(product.PartName) == "" {
			return fmt.Errorf("manual product %d name is required", i+1)
		}
		if err := validatePurchaseImportNumber(product.Quantity, true); err != nil {
			return fmt.Errorf("manual product %d quantity must be positive", i+1)
		}
		if err := validatePurchaseImportNumber(product.Price, false); err != nil {
			return fmt.Errorf("manual product %d price must be a non-negative number", i+1)
		}
		if strings.TrimSpace(product.CostPrice) == "" {
			return fmt.Errorf("manual product %d cost_price is required", i+1)
		}
		if err := validatePurchaseImportNumber(product.CostPrice, false); err != nil {
			return fmt.Errorf("manual product %d cost_price must be a non-negative number", i+1)
		}
		product.PartName = strings.TrimSpace(product.PartName)
		product.Price = strings.TrimSpace(product.Price)
		product.Quantity = strings.TrimSpace(product.Quantity)
		product.CostPrice = strings.TrimSpace(product.CostPrice)
	}

	payload.MerchantID = payload.SupplierID
	payload.State = 1
	return nil
}

func validatePurchaseImportNumber(value string, positive bool) error {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fmt.Errorf("invalid number")
	}
	if positive {
		if parsed <= 0 {
			return fmt.Errorf("number must be positive")
		}
	} else if parsed < 0 {
		return fmt.Errorf("number must not be negative")
	}
	return nil
}

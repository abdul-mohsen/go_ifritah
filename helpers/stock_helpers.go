package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"afrita/config"
	"afrita/models"
)

// SubmitStockAdjustment sends a stock adjustment to the backend.
func SubmitStockAdjustment(token string, adj models.StockAdjustRequest) error {
	payload, _ := json.Marshal(adj)
	log.Printf("🔵 [API REQUEST] POST %s/api/v2/stock/adjust | payload: %s", config.BackendDomain, string(payload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/stock/adjust", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// FetchStockMovements fetches stock movement history for a product.
func FetchStockMovements(token string, productID string) ([]models.StockMovement, error) {
	log.Printf("🔵 [API REQUEST] GET %s/api/v2/stock/movements/%s", config.BackendDomain, productID)

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/stock/movements/"+productID, nil)
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

	var movements []models.StockMovement
	if err := json.Unmarshal(bodyBytes, &movements); err != nil {
		// Try wrapper
		var wrapper map[string]json.RawMessage
		if json.Unmarshal(bodyBytes, &wrapper) == nil {
			if raw, ok := wrapper["data"]; ok {
				json.Unmarshal(raw, &movements)
			}
		}
	}
	if movements == nil {
		movements = []models.StockMovement{}
	}
	return movements, nil
}

// CheckStockAvailability checks stock for invoice items.
func CheckStockAvailability(token string, items []models.StockCheckItem) (models.StockCheckResponse, error) {
	payload, _ := json.Marshal(map[string]interface{}{"items": items})
	log.Printf("🔵 [API REQUEST] POST %s/api/v2/stock/check", config.BackendDomain)

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/stock/check", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return models.StockCheckResponse{}, err
	}
	defer resp.Body.Close()

	var result models.StockCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return models.StockCheckResponse{}, err
	}
	return result, nil
}

// FetchStockEnforcement fetches the current stock enforcement mode.
func FetchStockEnforcement(token string) (string, error) {
	log.Printf("🔵 [API REQUEST] GET %s/api/v2/stock/enforcement", config.BackendDomain)

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/stock/enforcement", nil)
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return models.StockEnforcementDisable, err
	}
	defer resp.Body.Close()

	var result models.StockEnforcementResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return models.StockEnforcementDisable, err
	}
	return result.Mode, nil
}

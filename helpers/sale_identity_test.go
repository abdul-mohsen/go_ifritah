package helpers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sentSaleIdentity returns the pos_id and bill_request_id that BuildBillPayload puts in the body of a sale.
func sentSaleIdentity(t *testing.T, form url.Values) (string, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := json.Marshal(BuildBillPayload(req))
	if err != nil {
		t.Fatalf("failed to marshal BillPayload: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("failed to unmarshal BillPayload: %v", err)
	}
	posID, _ := raw["pos_id"].(string)
	requestID, _ := raw["bill_request_id"].(string)
	return posID, requestID
}

// TestBuildBillPayload_SaleIdentity checks that a sale sends the pos_id and bill_request_id of its form. A retry of the
// same attempt sends the same bill_request_id, and a new sale sends the one it was given.
func TestBuildBillPayload_SaleIdentity(t *testing.T) {
	const posID = "3f0c6a1e-8d2b-4c4e-9a7f-1b2c3d4e5f60"
	const attempt = "9b1d2c3e-4f5a-4b6c-8d7e-0f1a2b3c4d5e"
	const nextAttempt = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"

	sale := func(requestID string) url.Values {
		return url.Values{
			"store_id":            {"1"},
			"branch_id":           {"1"},
			"state":               {"1"},
			"products_product_id": {"6"},
			"products_price":      {"100"},
			"products_quantity":   {"1"},
			"total_amount":        {"115"},
			"pos_id":              {posID},
			"bill_request_id":     {requestID},
		}
	}

	gotPosID, gotAttempt := sentSaleIdentity(t, sale(attempt))
	if gotPosID != posID {
		t.Errorf("pos_id = %q, want %q", gotPosID, posID)
	}
	if gotAttempt != attempt {
		t.Errorf("bill_request_id = %q, want %q", gotAttempt, attempt)
	}

	_, retried := sentSaleIdentity(t, sale(attempt))
	if retried != attempt {
		t.Errorf("retry sent bill_request_id %q, want the same %q", retried, attempt)
	}

	_, next := sentSaleIdentity(t, sale(nextAttempt))
	if next != nextAttempt {
		t.Errorf("new sale sent bill_request_id %q, want %q", next, nextAttempt)
	}
}

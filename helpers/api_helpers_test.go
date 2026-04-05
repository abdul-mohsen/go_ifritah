package helpers

import (
	"afrita/models"
	"testing"
)

func TestDecodeListResponseArray(t *testing.T) {
	body := []byte(`[{"id":1},{"id":2}]`)

	invoices, err := decodeListResponse[models.Invoice](body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invoices) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(invoices))
	}
}

func TestDecodeListResponseWrapped(t *testing.T) {
	body := []byte(`{"data":[{"id":1},{"id":2}]}`)

	invoices, err := decodeListResponse[models.Invoice](body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invoices) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(invoices))
	}
}

func TestDecodeListResponseOrdersWrapped(t *testing.T) {
	body := []byte(`{"orders":[{"client":"Acme"},{"client":"Beta"}]}`)

	orders, err := decodeListResponse[map[string]interface{}](body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}
}

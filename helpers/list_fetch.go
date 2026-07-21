package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"afrita/config"
	"afrita/models"
)

// ListOpts carries the search/filter/pagination params that are sent
// to the backend on list-page requests. Empty fields are omitted from the
// JSON payload so the BE applies its defaults.
//
// Sort is FE-only (BE returns rows in canonical keyset order — see
// search-and-filters mailbox msg #10). Header-click sort happens
// client-side in static/js/script.js.
type ListOpts struct {
	Page        int    // 0-based page number
	PerPage     int    // page size; 0 means default
	Query       string // free-text search; BE matches against indexed fields
	State       string // textual int: "0", "1", "2", "3" — applied as int filter
	Stock       string // "in" / "out" (products only)
	VoucherType string // "disbursement" / "receipt" / "cash_box" (cash vouchers only)
	Role        string // "admin" / "manager" / "employee" (users only)

	// Typed carries per-resource typed-field filters (phone, vat_number,
	// commercial_registration, sequence_number, supplier_sequence_number,
	// part_number, barcode, vin, email). Keys are the canonical BE param
	// names locked in mailbox #19. Values are forwarded as top-level JSON
	// keys; the BE applies prefix-LIKE on indexed columns (or a digits-only
	// generated stored column for `phone`).
	Typed map[string]string
}

// MarshalListPayload builds the JSON request body for the BE list endpoints.
// Only set fields are included so the BE can apply its defaults.
func (o ListOpts) MarshalListPayload() []byte {
	page := o.Page
	if page < 0 {
		page = 0
	}
	perPage := o.PerPage
	if perPage <= 0 {
		perPage = 10
	}
	m := map[string]interface{}{
		"page_number": page,
		"page_size":   perPage,
	}
	if o.Query != "" {
		m["query"] = o.Query
	}
	if o.State != "" {
		if v, err := strconv.Atoi(o.State); err == nil {
			m["state"] = v
		}
	}
	if o.Stock != "" {
		m["stock"] = o.Stock
	}
	if o.VoucherType != "" {
		m["voucher_type"] = o.VoucherType
	}
	if o.Role != "" {
		m["role"] = o.Role
	}
	for k, v := range o.Typed {
		if k == "" || v == "" {
			continue
		}
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

// ListMeta is the pagination envelope returned by list endpoints. Empty
// fields mean the BE didn't supply that value; FE renders accordingly.
type ListMeta struct {
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
}

// postListBytes does a single POST against a BE list endpoint and returns
// the raw response bytes. No caching — opts vary per request.
func postListBytes(token, endpoint string, opts ListOpts) ([]byte, error) {
	url := config.BackendDomain + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(opts.MarshalListPayload()))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d for %s", resp.StatusCode, endpoint)
	}
	return io.ReadAll(resp.Body)
}

// FetchBranchesList — backend-driven branches list page.
func FetchBranchesList(token string, opts ListOpts) ([]models.Branch, error) {
	body, err := postListBytes(token, "/api/v2/branch/all", opts)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[models.Branch](body)
}

// FetchClientsList — backend-driven clients list page.
func FetchClientsList(token string, opts ListOpts) ([]models.Client, error) {
	body, err := postListBytes(token, "/api/v2/client/all", opts)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[models.Client](body)
}

// FetchSuppliersList — backend-driven suppliers list page.
func FetchSuppliersList(token string, opts ListOpts) ([]models.Supplier, error) {
	body, err := postListBytes(token, "/api/v2/supplier/all", opts)
	if err != nil {
		return nil, err
	}
	return decodeSupplierList(body)
}

// FetchStoresList — backend-driven stores list page. The /stores/all endpoint
// is GET-only today; pass page_number / page_size / query as URL params.
func FetchStoresList(token string, opts ListOpts) ([]models.Store, error) {
	url := config.BackendDomain + "/api/v2/stores/all"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	q := req.URL.Query()
	if opts.Query != "" {
		q.Set("query", opts.Query)
	}
	if opts.PerPage > 0 {
		q.Set("page_size", strconv.Itoa(opts.PerPage))
	}
	q.Set("page_number", strconv.Itoa(opts.Page))
	req.URL.RawQuery = q.Encode()
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[models.Store](body)
}

// FetchProductsList — backend-driven products list page.
func FetchProductsList(token string, opts ListOpts) ([]models.Product, error) {
	body, err := postListBytes(token, "/api/v2/product/all", opts)
	if err != nil {
		return nil, err
	}
	products, err := decodeListResponse[models.Product](body)
	if err != nil {
		// Fallback: try the loose-shape decode (some BE versions return
		// raw maps with article_id instead of id).
		items, itemErr := decodeListResponse[map[string]interface{}](body)
		if itemErr != nil {
			return nil, err
		}
		products = make([]models.Product, 0, len(items))
		for _, item := range items {
			id := 0
			if v, ok := CoerceFloat(item["article_id"]); ok {
				id = int(v)
			}
			if id == 0 {
				if v, ok := CoerceFloat(item["id"]); ok {
					id = int(v)
				}
			}
			qty := ""
			if v, ok := item["quantity"].(string); ok {
				qty = v
			} else if v, ok := CoerceFloat(item["quantity"]); ok {
				// %g switches to scientific for >=1e6, which is wrong for
				// inventory quantities; FormatFloat with prec=-1 keeps decimal
				// form and the shortest representation that round-trips.
				qty = strconv.FormatFloat(v, 'f', -1, 64)
			}
			price := ""
			if v, ok := item["price"].(string); ok {
				price = v
			} else if v, ok := CoerceFloat(item["price"]); ok {
				price = strconv.FormatFloat(v, 'f', -1, 64)
			}
			partName := ""
			if v, ok := item["part_name"].(string); ok {
				partName = v
			}
			if partName == "" {
				if v, ok := item["name"].(string); ok {
					partName = v
				}
			}
			shelfNumber := ""
			if v, ok := item["shelf_number"].(string); ok {
				shelfNumber = v
			}
			costPrice := ""
			if v, ok := item["cost_price"].(string); ok {
				costPrice = v
			} else if v, ok := CoerceFloat(item["cost_price"]); ok {
				costPrice = strconv.FormatFloat(v, 'f', -1, 64)
			}
			storeID := 0
			if v, ok := CoerceFloat(item["store_id"]); ok {
				storeID = int(v)
			}
			products = append(products, models.Product{
				ID: id, PartName: partName, Quantity: qty, Price: price,
				CostPrice: costPrice, ShelfNumber: shelfNumber, StoreID: storeID,
			})
		}
	}
	for i, p := range products {
		if p.ID == 0 && p.PartID > 0 {
			products[i].ID = p.PartID
		}
		if products[i].PartName == "" && products[i].Name != "" {
			products[i].PartName = products[i].Name
		}
	}
	return products, nil
}

// FetchOrdersList — backend-driven orders list page.
func FetchOrdersList(token string, opts ListOpts) ([]map[string]interface{}, error) {
	body, err := postListBytes(token, "/api/v2/order/all", opts)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[map[string]interface{}](body)
}

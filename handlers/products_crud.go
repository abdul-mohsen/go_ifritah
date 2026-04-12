package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strconv"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

// unmarshalProduct decodes backend JSON into a Product, applying name fallbacks.
func unmarshalProduct(data []byte) models.Product {
	var p models.Product
	_ = json.Unmarshal(data, &p)
	if p.ID == 0 && p.PartID > 0 {
		p.ID = p.PartID
	}
	if p.PartName == "" && p.Name != "" {
		p.PartName = p.Name
	}
	return p
}

// HandleProducts displays the products list page
func HandleProducts(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	products, err := helpers.FetchProducts(token)
	if err != nil {
		products = []models.Product{}
	}
	helpers.EnrichProductPartNames(products, token)

	stockFilter := r.URL.Query().Get("stock")
	query := r.URL.Query().Get("q")

	// Filter by search query (ID or part name)
	if query != "" {
		filtered := make([]models.Product, 0)
		for _, p := range products {
			idStr := fmt.Sprintf("%d", p.ID)
			if helpers.ContainsInsensitive(idStr, query) ||
				helpers.ContainsInsensitive(p.PartName, query) {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}

	// Filter by stock
	if stockFilter == "in" {
		filtered := make([]models.Product, 0)
		for _, p := range products {
			if helpers.ParseIntValue(p.Quantity) > 0 {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	} else if stockFilter == "out" {
		filtered := make([]models.Product, 0)
		for _, p := range products {
			if helpers.ParseIntValue(p.Quantity) <= 0 {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedProducts, pagination := helpers.PaginateSlice(products, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "products", map[string]interface{}{
		"title":      "المنتجات",
		"products":   pagedProducts,
		"stock":      stockFilter,
		"query":      query,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
	})
}

// HandleAddProduct displays the add product form
func HandleAddProduct(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stores, err := helpers.FetchStores(token)
	if err != nil {
		stores = []models.Store{}
	}

	helpers.Render(w, r, "add-product", map[string]interface{}{
		"title":  "إضافة منتج",
		"stores": stores,
	})
}

// HandleProductDetail displays product details
func HandleProductDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	var product models.Product
	found := false
	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/product/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err == nil && resp.StatusCode == 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		product = unmarshalProduct(bodyBytes)
		if product.ID > 0 {
			found = true
		}
	} else {
		if resp != nil {
			resp.Body.Close()
		}
	}

	if !found {
		prodID := helpers.ParseIntValue(id)
		products, _ := helpers.FetchProducts(token)
		for _, p := range products {
			if p.ID == prodID || p.PartID == prodID {
				product = p
				found = true
				break
			}
		}
		if !found {
			product = models.Product{ID: prodID}
		}
	}

	// Enrich part name from parts API if not set
	if product.PartName == "" && product.PartID > 0 {
		if nameMap, err := helpers.FetchPartNames(token); err == nil {
			if name, ok := nameMap[product.PartID]; ok {
				product.PartName = name
			}
		}
	}

	storeName := ""
	if product.StoreID > 0 {
		stores, _ := helpers.FetchStores(token)
		for _, s := range stores {
			if s.ID == product.StoreID {
				storeName = s.Name
				break
			}
		}
	}

	helpers.Render(w, r, "product-detail", map[string]interface{}{
		"title":      "تفاصيل المنتج",
		"product":    product,
		"store_name": storeName,
	})
}

// HandleEditProduct displays the edit product form
func HandleEditProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stores, _ := helpers.FetchStores(token)
	if stores == nil {
		stores = []models.Store{}
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/product/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	var product models.Product
	if err == nil && resp.StatusCode == 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		product = unmarshalProduct(bodyBytes)
	} else {
		if resp != nil {
			resp.Body.Close()
		}
		product = models.Product{ID: helpers.ParseIntValue(id)}
	}

	// Enrich part name
	if product.PartName == "" && product.PartID > 0 {
		if nameMap, err := helpers.FetchPartNames(token); err == nil {
			if name, ok := nameMap[product.PartID]; ok {
				product.PartName = name
			}
		}
	}

	helpers.Render(w, r, "edit-product", map[string]interface{}{
		"title":   "تعديل المنتج",
		"id":      id,
		"product": product,
		"stores":  stores,
	})
}

// HandleCreateProduct creates new products (supports multiple at once).
// Backend expects: {"store_id": int, "products": [{id, quantity, price, shelf_number, cost_price}, ...]}
func HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	storeIDStr := r.FormValue("store_id")
	quantities := r.Form["quantity[]"]
	prices := r.Form["price[]"]
	costPrices := r.Form["cost_price[]"]
	shelfNumbers := r.Form["shelf_number[]"]
	partNames := r.Form["part_name[]"]

	// Validate: need at least store_id and one product row
	if storeIDStr == "" || len(quantities) == 0 {
		stores, _ := helpers.FetchStores(token)
		if stores == nil {
			stores = []models.Store{}
		}
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":  "إضافة منتج",
			"stores": stores,
		}, map[string]string{"store_id": "يرجى اختيار المخزن"}, nil)
		helpers.Render(w, r, "add-product", data)
		return
	}

	// Validate each product row has quantity, price, and part name
	for i := range quantities {
		partName := ""
		if i < len(partNames) {
			partName = partNames[i]
		}
		if quantities[i] == "" || (i < len(prices) && prices[i] == "") || partName == "" {
			stores, _ := helpers.FetchStores(token)
			if stores == nil {
				stores = []models.Store{}
			}
			data := helpers.RenderFormWithErrors(map[string]interface{}{
				"title":  "إضافة منتج",
				"stores": stores,
			}, map[string]string{"products": "يرجى اختيار القطعة وتعبئة الكمية والسعر لكل منتج"}, nil)
			helpers.Render(w, r, "add-product", data)
			return
		}
	}

	storeID, _ := strconv.Atoi(storeIDStr)

	// Build products array
	products := make([]map[string]interface{}, 0, len(quantities))
	for i := range quantities {
		bigN, _ := rand.Int(rand.Reader, big.NewInt(900000))
		id := int(bigN.Int64()) + 100000 // cryptographically random 6-digit ID
		qty := 0
		price := 0
		costPrice := 0
		shelfNum := ""
		if i < len(quantities) {
			qty, _ = strconv.Atoi(quantities[i])
		}
		if i < len(prices) {
			price, _ = strconv.Atoi(prices[i])
		}
		if i < len(costPrices) {
			costPrice, _ = strconv.Atoi(costPrices[i])
		}
		if i < len(shelfNumbers) {
			shelfNum = shelfNumbers[i]
		}
		partName := ""
		if i < len(partNames) {
			partName = partNames[i]
		}
		products = append(products, map[string]interface{}{
			"product_id":   id,
			"quantity":     qty,
			"price":        price,
			"cost_price":   costPrice,
			"shelf_number": shelfNum,
			"name":         partName,
		})
	}

	payloadMap := map[string]interface{}{
		"store_id": storeID,
		"products": products,
	}
	jsonPayload, _ := json.Marshal(payloadMap)
	log.Printf("[CREATE PRODUCT] OEM parts: %v", partNames)
	log.Printf("[CREATE PRODUCT] Payload: %s", string(jsonPayload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/product", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[CREATE PRODUCT] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في إنشاء المنتج")
		return
	}

	helpers.APICache.Delete("products")
	helpers.WriteSuccessRedirect(w, "/dashboard/products", "تم إنشاء المنتج بنجاح")
}

// HandleUpdateProduct updates an existing product
func HandleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "price", Value: r.FormValue("price"), Required: true, Label: "سعر القطعة"},
		{Field: "store_id", Value: r.FormValue("store_id"), Required: true, Label: "المخزن"},
	})
	if errs != nil {
		stores, _ := helpers.FetchStores(token)
		if stores == nil {
			stores = []models.Store{}
		}
		oldValues := helpers.OldValues([]string{"price", "store_id"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":   "تعديل المنتج",
			"id":      id,
			"product": models.Product{ID: helpers.ParseIntValue(id)},
			"stores":  stores,
		}, errs, oldValues)
		helpers.Render(w, r, "edit-product", data)
		return
	}

	storeID, _ := strconv.Atoi(r.FormValue("store_id"))
	productID := helpers.ParseIntValue(id)
	price, _ := strconv.Atoi(r.FormValue("price"))
	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	costPrice, _ := strconv.Atoi(r.FormValue("cost_price"))
	shelfNumber := r.FormValue("shelf_number")
	if qty == 0 {
		qty = 1
	}

	payloadMap := map[string]interface{}{
		"store_id": storeID,
		"products": []map[string]interface{}{
			{
				"product_id":   productID,
				"quantity":     qty,
				"price":        price,
				"cost_price":   costPrice,
				"shelf_number": shelfNumber,
			},
		},
	}
	jsonPayload, _ := json.Marshal(payloadMap)

	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/product/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في تحديث المنتج")
		return
	}

	helpers.APICache.Delete("products")
	helpers.WriteSuccessRedirect(w, "/dashboard/products", "تم تحديث المنتج بنجاح")
}

// HandleDeleteProduct deletes a product
func HandleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/product/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في حذف المنتج")
		return
	}

	helpers.APICache.Delete("products")
	helpers.WriteSuccessRedirect(w, "/dashboard/products", "تم حذف المنتج بنجاح")
}

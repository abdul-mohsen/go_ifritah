package handlers

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ImportedItem struct {
	PartName     string  `json:"partName"`
	Quantity     int     `json:"quantity"`
	PurchasePrice float64 `json:"purchasePrice"`
	CostPrice    float64 `json:"costPrice"`
	ShelfNumber  string  `json:"shelfNumber"`
}

// HandleDownloadPurchaseBillTemplate generates a CSV template for import
func HandleDownloadPurchaseBillTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=purchase-bill-template.csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"اسم القطعة", "الكمية", "سعر الشراء", "سعر التكلفة", "رقم الرف"})

	// Write example row
	writer.Write([]string{"مثال", "10", "100.00", "90.00", "A1"})
}

// HandleParseCSVItems parses uploaded CSV and returns items
func HandleParseCSVItems(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Parse CSV
	reader := csv.NewReader(file)
	var items []ImportedItem
	rowNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to parse file"})
			return
		}

		rowNum++
		if rowNum == 1 {
			continue // Skip header
		}

		if len(record) < 5 {
			continue
		}

		qty, qtyErr := strconv.Atoi(strings.TrimSpace(record[1]))
		if qtyErr != nil {
			continue
		}

		purchasePrice, priceErr := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
		if priceErr != nil {
			continue
		}

		costPrice, costErr := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
		if costErr != nil {
			continue
		}

		items = append(items, ImportedItem{
			PartName:      strings.TrimSpace(record[0]),
			Quantity:      qty,
			PurchasePrice: purchasePrice,
			CostPrice:     costPrice,
			ShelfNumber:   strings.TrimSpace(record[4]),
		})
	}

	json.NewEncoder(w).Encode(items)
}

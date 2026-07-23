package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ImportedItem struct {
	PartName      string  `json:"partName"`
	Quantity      int     `json:"quantity"`
	PurchasePrice float64 `json:"purchasePrice"`
	CostPrice     float64 `json:"costPrice"`
	ShelfNumber   string  `json:"shelfNumber"`
	// ProductID is an optional existing-catalog-product reference. When set,
	// the frontend fetches that product's current shelf/cost/selling price
	// for the user to review before submit. A blank/missing value means
	// "this is a new item" — the frontend still checks by name and warns
	// (without auto-linking) if a product with the same name already
	// exists, since that's a common data-entry mistake in import sheets.
	ProductID *int `json:"productId,omitempty"`
}

var purchaseBillImportTemplateHeader = []string{
	"اسم القطعة", "الكمية", "سعر الشراء", "رقم الرف", "معرف المنتج (اختياري)",
}

const contentTypeHeader = "Content-Type"

// HandleDownloadPurchaseBillTemplate generates the purchase-bill CSV import template.
func HandleDownloadPurchaseBillTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=purchase-bill-template.csv")
	w.Header().Set(contentTypeHeader, "text/csv; charset=utf-8")

	if _, err := w.Write(buildPurchaseBillImportCSVTemplate()); err != nil {
		http.Error(w, "failed to write template", http.StatusInternalServerError)
		return
	}
}

// HandleDownloadPurchaseBillExcelTemplate keeps the Excel template endpoint working.
func HandleDownloadPurchaseBillExcelTemplate(w http.ResponseWriter, r *http.Request) {
	workbook, err := buildPurchaseBillImportWorkbook([][]string{
		purchaseBillImportTemplateHeader,
		{"مثال", "10", "100.00", "A1", ""},
	})
	if err != nil {
		http.Error(w, "failed to create template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=purchase-bill-template.xlsx")
	w.Header().Set(contentTypeHeader, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	if _, err := w.Write(workbook); err != nil {
		http.Error(w, "failed to write template", http.StatusInternalServerError)
		return
	}
}

// HandleParseExcelItems parses the uploaded import file and returns purchase-bill items.
func HandleParseExcelItems(w http.ResponseWriter, r *http.Request) {
	handleParseImportedItems(w, r)
}

// HandleParseCSVItems keeps the previous route working while parsing the same import payload.
func HandleParseCSVItems(w http.ResponseWriter, r *http.Request) {
	handleParseImportedItems(w, r)
}

func handleParseImportedItems(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, "application/json")

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeImportError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeImportError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	payload, err := io.ReadAll(file)
	if err != nil {
		writeImportError(w, http.StatusBadRequest, "Failed to read file")
		return
	}

	items, err := parseImportedItems(payload, header.Filename)
	if err != nil {
		writeImportError(w, http.StatusBadRequest, "Failed to parse file")
		return
	}

	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func parseImportedItems(payload []byte, filename string) ([]ImportedItem, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".csv":
		return parseCSVItems(bytes.NewReader(payload))
	case ".xlsx":
		return parseXLSXItems(payload)
	default:
		items, err := parseXLSXItems(payload)
		if err == nil {
			return items, nil
		}
		return parseCSVItems(bytes.NewReader(payload))
	}
}

func parseXLSXItems(payload []byte) ([]ImportedItem, error) {
	readerAt := bytes.NewReader(payload)
	workbook, err := zip.NewReader(readerAt, int64(len(payload)))
	if err != nil {
		return nil, err
	}

	sharedStrings, err := readSharedStrings(workbook)
	if err != nil {
		return nil, err
	}

	worksheetXML, err := readFirstWorksheet(workbook)
	if err != nil {
		return nil, err
	}

	var worksheet xlsxWorksheet
	if err := xml.Unmarshal(worksheetXML, &worksheet); err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(worksheet.SheetData.Rows))
	for _, row := range worksheet.SheetData.Rows {
		rowValues := make([]string, 6)
		for _, cell := range row.Cells {
			column := excelColumnIndex(cell.Reference)
			if column < 0 || column >= len(rowValues) {
				continue
			}
			rowValues[column] = cell.value(sharedStrings)
		}
		rows = append(rows, rowValues)
	}

	return parseImportedRows(rows), nil
}

func parseCSVItems(reader io.Reader) ([]ImportedItem, error) {
	csvReader := csv.NewReader(reader)
	// Allow rows with fewer fields than the header (e.g. sheets written
	// before the optional trailing product-id column existed, or hand-
	// edited exports that trim empty trailing cells). trimmedCell already
	// treats an out-of-range index as blank, so a short row just means
	// "no product id" rather than a hard parse failure.
	csvReader.FieldsPerRecord = -1
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	return parseImportedRows(rows), nil
}

func parseImportedRows(rows [][]string) []ImportedItem {
	items := make([]ImportedItem, 0, len(rows))
	for index, row := range rows {
		if index == 0 {
			continue
		}

		partName := trimmedCell(row, 0)
		qty, err := strconv.Atoi(trimmedCell(row, 1))
		if partName == "" || err != nil {
			continue
		}

		purchasePrice, err := parseOptionalFloat(trimmedCell(row, 2))
		if err != nil {
			continue
		}

		items = append(items, ImportedItem{
			PartName:      partName,
			Quantity:      qty,
			PurchasePrice: purchasePrice,
			CostPrice:     purchasePrice, // single price field: same value for both
			ShelfNumber:   trimmedCell(row, 3),
			ProductID:     parseOptionalProductID(trimmedCell(row, 4)),
		})
	}

	return items
}

// parseOptionalProductID parses the optional "existing product id" import
// column. Per product requirements: a blank value means "this is a new
// item" (not an error); an unparseable value is treated the same way
// rather than failing the whole row, since the frontend will still run its
// own by-name existing-item check for items with no usable id.
func parseOptionalProductID(value string) *int {
	if value == "" {
		return nil
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}

func trimmedCell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseOptionalFloat(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func writeImportError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func buildPurchaseBillImportCSVTemplate() []byte {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buffer)
	writer.UseCRLF = true
	_ = writer.Write(purchaseBillImportTemplateHeader)
	_ = writer.Write([]string{"مثال", "10", "100.00", "A1", ""})
	writer.Flush()
	return buffer.Bytes()
}

func buildPurchaseBillImportWorkbook(rows [][]string) ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)

	files := map[string]string{
		"[Content_Types].xml":        contentTypesXML,
		"_rels/.rels":                rootRelsXML,
		"xl/workbook.xml":            workbookXML,
		"xl/_rels/workbook.xml.rels": workbookRelsXML,
		"xl/worksheets/sheet1.xml":   buildWorksheetXML(rows),
	}

	for name, content := range files {
		fileWriter, err := zipWriter.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := fileWriter.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func buildWorksheetXML(rows [][]string) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)

	for rowIndex, row := range rows {
		builder.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex+1))
		for columnIndex, value := range row {
			if value == "" {
				continue
			}
			cellRef := excelCellReference(columnIndex, rowIndex+1)
			if _, err := strconv.ParseFloat(value, 64); err == nil {
				builder.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, cellRef, xmlEscape(value)))
				continue
			}
			builder.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, cellRef, xmlEscape(value)))
		}
		builder.WriteString(`</row>`)
	}

	builder.WriteString(`</sheetData></worksheet>`)
	return builder.String()
}

func excelCellReference(columnIndex, rowNumber int) string {
	return string(rune('A'+columnIndex)) + strconv.Itoa(rowNumber)
}

func excelColumnIndex(reference string) int {
	column := 0
	for _, r := range reference {
		if r < 'A' || r > 'Z' {
			break
		}
		column = column*26 + int(r-'A'+1)
	}
	if column == 0 {
		return -1
	}
	return column - 1
}

func xmlEscape(value string) string {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		return value
	}
	return buffer.String()
}

func readFirstWorksheet(workbook *zip.Reader) ([]byte, error) {
	names := make([]string, 0)
	for _, file := range workbook.File {
		if strings.HasPrefix(file.Name, "xl/worksheets/") && strings.HasSuffix(file.Name, ".xml") {
			names = append(names, file.Name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("workbook has no worksheets")
	}
	sort.Strings(names)
	return readZipFile(workbook, names[0])
}

func readSharedStrings(workbook *zip.Reader) ([]string, error) {
	content, err := readZipFile(workbook, "xl/sharedStrings.xml")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}

	var sharedStrings xlsxSharedStrings
	if err := xml.Unmarshal(content, &sharedStrings); err != nil {
		return nil, err
	}

	values := make([]string, 0, len(sharedStrings.Items))
	for _, item := range sharedStrings.Items {
		if item.Text != "" {
			values = append(values, item.Text)
			continue
		}
		var builder strings.Builder
		for _, run := range item.Runs {
			builder.WriteString(run.Text)
		}
		values = append(values, builder.String())
	}
	return values, nil
}

func readZipFile(workbook *zip.Reader, name string) ([]byte, error) {
	for _, file := range workbook.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}
	return nil, fmt.Errorf("zip entry %s not found", name)
}

type xlsxWorksheet struct {
	SheetData struct {
		Rows []xlsxRow `xml:"row"`
	} `xml:"sheetData"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Reference string `xml:"r,attr"`
	Type      string `xml:"t,attr"`
	Value     string `xml:"v"`
	InlineStr struct {
		Text string `xml:"t"`
	} `xml:"is"`
}

func (cell xlsxCell) value(sharedStrings []string) string {
	switch cell.Type {
	case "inlineStr":
		return strings.TrimSpace(cell.InlineStr.Text)
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err != nil || index < 0 || index >= len(sharedStrings) {
			return ""
		}
		return strings.TrimSpace(sharedStrings[index])
	default:
		return strings.TrimSpace(cell.Value)
	}
}

type xlsxSharedStrings struct {
	Items []xlsxSharedStringItem `xml:"si"`
}

type xlsxSharedStringItem struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const workbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Template" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`

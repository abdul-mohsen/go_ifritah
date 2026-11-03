package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// importCheckChunk is the most rows that one backend check takes. The backend refuses a larger batch.
const importCheckChunk = 500

// The codes that a row gets before the backend checks it. Such a row is blocked. The backend's codes pass through unchanged.
const (
	importCodeNoProducts = "no_products"
	importCodeInvalidRow = "invalid_row"
	importCodeNoAnswer   = "no_answer"
)

// previewIssue is one error or warning of a row. Code names the rule, and Message is the text that the user reads.
type previewIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// previewRow is one row of a sales import, as the preview shows it. Row is the number of the row in the import, which is also
// the number in its identity.
type previewRow struct {
	Row         int            `json:"row"`
	Customer    string         `json:"customer"`
	Date        string         `json:"date"`
	Total       string         `json:"total"`
	Valid       bool           `json:"valid"`
	Errors      []previewIssue `json:"errors"`
	Warnings    []previewIssue `json:"warnings"`
	DuplicateOf *uint64        `json:"duplicate_of"`
}

// previewResponse is the answer of a check.
type previewResponse struct {
	Total int          `json:"total"`
	Rows  []previewRow `json:"rows"`
	Error string       `json:"error,omitempty"`
}

// backendCheckRow is one row of the backend's answer to a check. Index is the position of the row in the request.
type backendCheckRow struct {
	Index       int            `json:"index"`
	Valid       bool           `json:"valid"`
	Errors      []previewIssue `json:"errors"`
	Warnings    []previewIssue `json:"warnings"`
	DuplicateOf *uint64        `json:"duplicate_of"`
}

// salesCandidate is one row of a sales import before its check. A row that can become a sale has a payload, and the backend
// checks it. A row that cannot has issues, and it is blocked.
type salesCandidate struct {
	row      int
	payload  *models.BillPayload
	issues   []previewIssue
	customer string
	date     string
	total    string
}

// salesOutcome is what a commit did with one selected row. billID is the bill that the backend answered with, and duplicateOf is the
// bill that the check of the row named as its duplicate, when there was one.
type salesOutcome struct {
	row         int
	ok          bool
	billID      uint64
	duplicateOf *uint64
	err         string
}

// checkSalesCandidates checks the candidates that can become sales, with the backend, in chunks of importCheckChunk, and answers
// every candidate in order. A row that the backend does not answer is blocked, so no row is ok without an answer.
func checkSalesCandidates(token string, candidates []salesCandidate) ([]previewRow, error) {
	rows := make([]previewRow, len(candidates))
	var sent []int
	for i, candidate := range candidates {
		rows[i] = newPreviewRow(candidate)
		if candidate.payload != nil {
			rows[i].Valid = true
			sent = append(sent, i)
		}
	}
	for start := 0; start < len(sent); start += importCheckChunk {
		if err := checkChunk(token, candidates, rows, sent[start:min(start+importCheckChunk, len(sent))]); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// newPreviewRow starts the answer for a candidate. A candidate that cannot become a sale is blocked with its own issues.
func newPreviewRow(candidate salesCandidate) previewRow {
	return previewRow{
		Row: candidate.row, Customer: candidate.customer, Date: candidate.date, Total: candidate.total,
		Errors: nonNilIssues(candidate.issues), Warnings: []previewIssue{},
	}
}

// checkChunk checks the candidates at batch with the backend, and writes the answer of each one into rows.
func checkChunk(token string, candidates []salesCandidate, rows []previewRow, batch []int) error {
	payloads := make([]models.BillPayload, 0, len(batch))
	for _, i := range batch {
		payloads = append(payloads, *candidates[i].payload)
	}
	answers, err := postImportCheck(token, payloads)
	if err != nil {
		return err
	}
	answered := make([]bool, len(batch))
	for _, answer := range answers {
		if answer.Index < 0 || answer.Index >= len(batch) {
			return fmt.Errorf("the check answered row %d, which it was not sent", answer.Index)
		}
		answered[answer.Index] = true
		applyCheckAnswer(&rows[batch[answer.Index]], answer)
	}
	for j, ok := range answered {
		if !ok {
			row := &rows[batch[j]]
			row.Valid = false
			row.Errors = append(row.Errors, previewIssue{Code: importCodeNoAnswer, Message: "the backend did not check this row"})
		}
	}
	return nil
}

// applyCheckAnswer writes the backend's answer for one row. A row is valid only when the backend says so and gives no error.
func applyCheckAnswer(row *previewRow, answer backendCheckRow) {
	row.Valid = answer.Valid && len(answer.Errors) == 0
	row.Errors = nonNilIssues(answer.Errors)
	row.Warnings = nonNilIssues(answer.Warnings)
	row.DuplicateOf = answer.DuplicateOf
}

// postImportCheck sends sale requests to the backend check, and returns its answer for them.
func postImportCheck(token string, payloads []models.BillPayload) ([]backendCheckRow, error) {
	body, err := json.Marshal(struct {
		Rows []models.BillPayload `json:"rows"`
	}{Rows: payloads})
	if err != nil {
		return nil, fmt.Errorf("prepare check")
	}
	req, err := http.NewRequest(http.MethodPost, config.BackendDomain+"/api/v2/bill/import/check", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("prepare check")
	}
	req.Header.Set(headerContentType, mimeJSON)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		return nil, fmt.Errorf("backend check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("backend check failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	var answer struct {
		Rows []backendCheckRow `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("backend check answered an unreadable body")
	}
	return answer.Rows, nil
}

// commitSales posts the selected candidates. The check covers only the selected candidates, and a row that it blocks is not
// posted: it is reported as failed with its messages. The outcomes are in the order of the candidates.
func commitSales(token string, candidates []salesCandidate, selected map[int]bool) ([]salesOutcome, error) {
	var chosen []salesCandidate
	for _, candidate := range candidates {
		if selected[candidate.row] {
			chosen = append(chosen, candidate)
		}
	}
	checked, err := checkSalesCandidates(token, chosen)
	if err != nil {
		return nil, err
	}
	outcomes := make([]salesOutcome, 0, len(chosen))
	for i, candidate := range chosen {
		outcome := salesOutcome{row: candidate.row}
		if candidate.payload == nil || previewBlocked(checked[i]) {
			outcome.err = joinIssueMessages(checked[i].Errors)
		} else if billID, err := postSaleBill(token, *candidate.payload); err != nil {
			outcome.err = err.Error()
		} else {
			outcome.ok = true
			outcome.billID = billID
			outcome.duplicateOf = checked[i].DuplicateOf
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}

// postSaleBill posts one sale of an import, and returns the id of the bill that the backend answered with. A replay is answered
// with the bill of the first attempt, as a create is.
func postSaleBill(token string, payload models.BillPayload) (uint64, error) {
	body, err := postImportedBillBody(token, "/api/v2/bill", payload)
	if err != nil {
		return 0, err
	}
	billID, err := strconv.ParseUint(strings.TrimSpace(string(body)), 10, 64)
	if err != nil {
		return 0, nil
	}
	return billID, nil
}

// previewBlocked reports whether a row may not be posted: it has an error, or the backend did not say it was valid.
func previewBlocked(row previewRow) bool {
	return !row.Valid || len(row.Errors) > 0
}

// joinIssueMessages joins the messages of some issues, for a row that failed.
func joinIssueMessages(issues []previewIssue) string {
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	return strings.Join(messages, "; ")
}

// nonNilIssues returns the issues, or an empty list, so that the answer carries [] and not null.
func nonNilIssues(issues []previewIssue) []previewIssue {
	if issues == nil {
		return []previewIssue{}
	}
	return issues
}

// salesPreviewFields returns the customer, date and total that the preview shows for a sale.
func salesPreviewFields(payload *models.BillPayload) (string, string, string) {
	date := ""
	if payload.EffectiveDate != nil && len(*payload.EffectiveDate) >= 10 {
		date = (*payload.EffectiveDate)[:10]
	}
	return payload.UserName, date, strconv.FormatFloat(payload.TotalAmount, 'f', 2, 64)
}

// selectedRowsOf reads the rows that a sales import posts. A sales import posts only the rows that it names, so a request
// without any row is refused.
func selectedRowsOf(values []string) (map[int]bool, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("choose the rows to import")
	}
	selected := make(map[int]bool, len(values))
	for _, value := range values {
		row, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || row <= 0 {
			return nil, fmt.Errorf("%q is not a row number", value)
		}
		selected[row] = true
	}
	return selected, nil
}

// selectedRowsOfInts is selectedRowsOf for the rows of a JSON request.
func selectedRowsOfInts(rows []int) (map[int]bool, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("choose the rows to import")
	}
	selected := make(map[int]bool, len(rows))
	for _, row := range rows {
		if row <= 0 {
			return nil, fmt.Errorf("%d is not a row number", row)
		}
		selected[row] = true
	}
	return selected, nil
}

// xlsxSalesCandidates turns the bills of a workbook into the candidates of a sales import. Row n is the n-th bill of the
// workbook, as before, so a bill without products keeps its number and is blocked.
func xlsxSalesCandidates(session string, bills []spreadsheetBill, products map[string][]spreadsheetProduct) []salesCandidate {
	candidates := make([]salesCandidate, 0, len(bills))
	for index, bill := range bills {
		candidate := salesCandidate{row: index + 1, customer: bill.values["customer_name"], date: bill.values["date"]}
		items := products[bill.reference]
		switch {
		case len(items) == 0:
			candidate.issues = []previewIssue{{Code: importCodeNoProducts, Message: "Bill Reference has no products"}}
		default:
			payload, err := salesPayload(session, candidate.row, bill, items)
			if err != nil {
				candidate.issues = []previewIssue{{Code: importCodeInvalidRow, Message: err.Error()}}
				break
			}
			candidate.payload = &payload
			candidate.customer, candidate.date, candidate.total = salesPreviewFields(&payload)
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

// salesPayload builds the sale request of one bill of a sales import, with the identity of its row.
func salesPayload(session string, row int, bill spreadsheetBill, products []spreadsheetProduct) (models.BillPayload, error) {
	items, total := importSalesItems(products)
	effective := helpers.ToBackendDate(bill.values["date"])
	if effective == "" {
		return models.BillPayload{}, fmt.Errorf("invalid Date")
	}
	var customerID *int
	if id := parsePositiveInt(bill.values["customer_id"]); id > 0 {
		customerID = &id
	}
	branchID := parsePositiveInt(bill.values["branch_id"])
	if branchID == 0 {
		branchID = 1
	}
	payload := models.BillPayload{
		StoreID:         parsePositiveInt(bill.values["store_id"]),
		Products:        []models.BillProductItem{},
		ManualProducts:  items,
		TotalAmount:     total,
		Discount:        importNumber(bill.values["discount"]),
		MaintenanceCost: "0",
		State:           3,
		VIN:             bill.values["vin"],
		UserName:        bill.values["customer_name"],
		UserPhoneNumber: bill.values["customer_phone"],
		Note:            bill.values["note"],
		EffectiveDate:   &effective,
		PaymentMethod:   10,
		ClientID:        customerID,
		BranchID:        branchID,
	}
	payload.PosID, payload.BillRequestID = importSaleIdentity(session, row)
	return payload, nil
}

// billResultsOf answers the rows of a sales commit in the shape of the CSV and data import results.
func billResultsOf(outcomes []salesOutcome) ([]billResult, int, int) {
	results := make([]billResult, 0, len(outcomes))
	success, failed := 0, 0
	for _, outcome := range outcomes {
		results = append(results, billResult{Row: outcome.row, OK: outcome.ok, BillID: outcome.billID, DuplicateOf: outcome.duplicateOf, Error: outcome.err})
		if outcome.ok {
			success++
		} else {
			failed++
		}
	}
	return results, success, failed
}

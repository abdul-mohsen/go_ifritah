package helpers

import (
	"fmt"
	"strings"

	"afrita/models"
)

// ProjectBillState is the single frontend state projection used by invoice
// lists, invoice detail/preview, dashboard rows, and the ZATCA monitor.
//
// A durable projection is authoritative. Numeric Invoice.State values are
// consulted only when the backend did not send any durable state fields.
func ProjectBillState(inv models.Invoice) models.BillStateProjection {
	if inv.StateProjection != nil && projectionHasDurableState(*inv.StateProjection) {
		return normalizeBillState(*inv.StateProjection)
	}

	if hasInvoiceDurableFields(inv) {
		return normalizeBillState(models.BillStateProjection{
			SaleState:     inv.SaleState,
			SaleStatus:    inv.SaleStatus,
			PaymentState:  inv.PaymentState,
			PaymentStatus: inv.PaymentStatus,
			InvoiceState:  inv.InvoiceState,
			InvoiceStatus: inv.InvoiceStatus,
			ZatcaState:    inv.ZatcaState,
			ZatcaStatus:   inv.ZatcaStatus,
			Durable:       true,
		})
	}

	return legacyBillState(inv)
}

// SetBillStateProjection attaches the durable projection found in a raw bill
// response to the typed invoice. It is intentionally a no-op for legacy-only
// responses so callers can distinguish an absent durable contract.
func SetBillStateProjection(inv *models.Invoice, raw map[string]interface{}) {
	if inv == nil {
		return
	}
	projection := BillStateProjectionFromRaw(raw)
	if projection == nil {
		return
	}
	inv.StateProjection = projection
	inv.SaleState = projection.SaleState
	inv.SaleStatus = projection.SaleStatus
	inv.PaymentState = projection.PaymentState
	inv.PaymentStatus = projection.PaymentStatus
	inv.InvoiceState = projection.InvoiceState
	inv.InvoiceStatus = projection.InvoiceStatus
	inv.ZatcaState = projection.ZatcaState
	inv.ZatcaStatus = projection.ZatcaStatus
}

// BillStateProjectionFromRaw extracts both nested and flattened versions of
// the clean backend state contract. The accepted aliases keep the frontend
// tolerant of snake_case/camelCase serialization without reviving numeric
// state fallbacks.
func BillStateProjectionFromRaw(raw map[string]interface{}) *models.BillStateProjection {
	if len(raw) == 0 {
		return nil
	}

	maps := []map[string]interface{}{raw}
	for _, key := range []string{"state_projection", "bill_state", "states", "statuses", "state"} {
		if nested, ok := rawMap(raw[key]); ok {
			maps = append(maps, nested)
		}
	}

	var submissionMaps []map[string]interface{}
	for _, key := range []string{"zatca_submission", "submission", "zatca"} {
		if nested, ok := rawMap(raw[key]); ok {
			submissionMaps = append(submissionMaps, nested)
			maps = append(maps, nested)
		}
	}

	sale, saleOK := readStateValue(maps, "sale_state", "sale_status", "sale")
	payment, paymentOK := readStateValue(maps, "payment_state", "payment_status", "payment")
	invoice, invoiceOK := readStateValue(maps, "invoice_state", "invoice_status", "invoice")
	zatca, zatcaOK := readStateValue(submissionMaps, "status", "state")
	if !zatcaOK {
		zatca, zatcaOK = readStateValue(maps, "zatca_state", "zatca_status", "zatca", "submission_status")
	}
	overall, overallOK := readStateValue(maps, "overall_state", "overall_status")
	if !overallOK {
		overall, overallOK = readStateValue([]map[string]interface{}{raw}, "status", "status_label")
	}

	projection := &models.BillStateProjection{
		SaleState:     sale,
		SaleStatus:    sale,
		PaymentState:  payment,
		PaymentStatus: payment,
		InvoiceState:  invoice,
		InvoiceStatus: invoice,
		ZatcaState:    zatca,
		ZatcaStatus:   zatca,
		OverallState:  overall,
		OverallStatus: overall,
		Reason:        firstStateString(maps, "reason", "error", "last_error", "rejection_msg", "rejection_message"),
		Warning:       firstStateString(maps, "warning", "warning_msg", "warning_message"),
		Durable:       saleOK || paymentOK || invoiceOK || zatcaOK || overallOK,
	}

	if boolState(maps, "result_unknown", "result-unknown", "unknown_result") {
		projection.OverallState = "result_unknown"
		projection.OverallStatus = "result_unknown"
		projection.Durable = true
	}
	if boolState(maps, "action_required", "requires_action", "needs_action") {
		projection.OverallState = "action_required"
		projection.OverallStatus = "action_required"
		projection.Durable = true
	}
	if boolState(maps, "retrying", "is_retrying") {
		projection.OverallState = "retrying"
		projection.OverallStatus = "retrying"
		projection.Durable = true
	}

	if !projection.Durable {
		return nil
	}
	return projection
}

// ProjectZatcaMonitorState creates the same projection used for a monitor row.
// The durable textual status always wins over a legacy numeric status code.
func ProjectZatcaMonitorState(status string, statusCode int) models.BillStateProjection {
	zatca := normalizeState(status)
	if zatca == "" || zatca == "unknown" {
		switch statusCode {
		case 2:
			zatca = "accepted"
		case 3:
			zatca = "rejected"
		case 4:
			zatca = "accepted_with_warning"
		case 0, 1:
			zatca = "pending"
		default:
			zatca = "unknown"
		}
	}
	return normalizeBillState(models.BillStateProjection{
		ZatcaState:  zatca,
		ZatcaStatus: zatca,
		Durable:     true,
	})
}

// InvoiceStatus returns the display status from the canonical projection.
func InvoiceStatus(inv models.Invoice) (string, string) {
	state := ProjectBillState(inv)
	if !state.Durable && inv.CreditState >= 1 {
		switch inv.CreditState {
		case 1:
			return "إشعار دائن قيد المعالجة", "badge-processing"
		case 2:
			return "إشعار دائن تمت المعالجة", "badge-processing"
		case 3:
			return "إشعار دائن صادرة", "badge-processing"
		default:
			return "ERROR", "badge-danger"
		}
	}
	return state.OverallLabel, state.OverallClass
}

func projectionHasDurableState(projection models.BillStateProjection) bool {
	return projection.Durable || hasAnyState(
		projection.SaleState,
		projection.SaleStatus,
		projection.PaymentState,
		projection.PaymentStatus,
		projection.InvoiceState,
		projection.InvoiceStatus,
		projection.ZatcaState,
		projection.ZatcaStatus,
		projection.OverallState,
		projection.OverallStatus,
	)
}

func hasInvoiceDurableFields(inv models.Invoice) bool {
	return hasAnyState(
		inv.SaleState,
		inv.SaleStatus,
		inv.PaymentState,
		inv.PaymentStatus,
		inv.InvoiceState,
		inv.InvoiceStatus,
		inv.ZatcaState,
		inv.ZatcaStatus,
	)
}

func legacyBillState(inv models.Invoice) models.BillStateProjection {
	state := "unknown"
	switch inv.State {
	case 0:
		state = "draft"
	case 1:
		state = "pending"
	case 2:
		state = "processed"
	case 3:
		state = "issued"
	}
	projection := normalizeBillState(models.BillStateProjection{
		InvoiceState: state,
		OverallState: state,
		Legacy:       true,
	})
	projection.Durable = false
	projection.Legacy = true
	projection.Issued = inv.State >= 2 && inv.State <= 3
	projection.CanReplace = inv.CreditState == 0 && (inv.State == 1 || inv.State == 3)
	projection.CanDuplicate = inv.State != 1
	return projection
}

func normalizeBillState(projection models.BillStateProjection) models.BillStateProjection {
	projection.Durable = projection.Durable || !projection.Legacy
	projection.Legacy = !projection.Durable

	projection.SaleState = normalizeState(firstNonEmpty(projection.SaleState, projection.SaleStatus))
	projection.SaleStatus = projection.SaleState
	projection.PaymentState = normalizeState(firstNonEmpty(projection.PaymentState, projection.PaymentStatus))
	projection.PaymentStatus = projection.PaymentState
	projection.InvoiceState = normalizeState(firstNonEmpty(projection.InvoiceState, projection.InvoiceStatus))
	projection.InvoiceStatus = projection.InvoiceState
	projection.ZatcaState = normalizeState(firstNonEmpty(projection.ZatcaState, projection.ZatcaStatus))
	projection.ZatcaStatus = projection.ZatcaState
	explicitOverall := normalizeState(firstNonEmpty(projection.OverallState, projection.OverallStatus))

	states := []string{projection.SaleState, projection.PaymentState, projection.InvoiceState, projection.ZatcaState}
	for _, state := range states {
		switch state {
		case "result_unknown":
			projection.ResultUnknown = true
		case "action_required":
			projection.ActionRequired = true
		case "rejected":
			projection.Rejected = true
		}
	}

	switch {
	case projection.ResultUnknown:
		projection.OverallState = "result_unknown"
	case projection.ActionRequired:
		projection.OverallState = "action_required"
	case projection.Rejected:
		projection.OverallState = "rejected"
	case hasState(states, "retrying"):
		projection.OverallState = "retrying"
	case hasState(states, "pending"):
		projection.OverallState = "pending"
	case projection.InvoiceState == "draft" || projection.SaleState == "draft":
		projection.OverallState = "draft"
	case isAcceptedState(projection.ZatcaState) &&
		(projection.InvoiceState == "" || isInvoiceComplete(projection.InvoiceState)):
		if projection.ZatcaState == "accepted_with_warning" {
			projection.OverallState = "issued_with_warning"
		} else {
			projection.OverallState = "issued"
		}
	case explicitOverall != "" && explicitOverall != "accepted" && explicitOverall != "issued":
		projection.OverallState = explicitOverall
	case projection.ZatcaState == "accepted" || projection.ZatcaState == "accepted_with_warning":
		projection.OverallState = "accepted"
	case explicitOverall != "":
		projection.OverallState = explicitOverall
	default:
		projection.OverallState = "unknown"
	}
	projection.OverallStatus = projection.OverallState

	projection.IsDraft = projection.OverallState == "draft"
	projection.Issued = projection.OverallState == "issued" || projection.OverallState == "issued_with_warning"
	projection.CanRetry = projection.ResultUnknown || projection.ActionRequired ||
		projection.OverallState == "retrying" || projection.OverallState == "rejected"
	if projection.Legacy {
		projection.CanDuplicate = !projection.ResultUnknown && projection.OverallState != "pending"
		projection.CanReplace = projection.CanDuplicate
	} else {
		projection.CanDuplicate = projection.Issued
		projection.CanReplace = projection.Issued
	}

	projection.SaleLabel, projection.SaleClass = stateDisplay(projection.SaleState, "sale")
	projection.PaymentLabel, projection.PaymentClass = stateDisplay(projection.PaymentState, "payment")
	projection.InvoiceLabel, projection.InvoiceClass = stateDisplay(projection.InvoiceState, "invoice")
	projection.ZatcaLabel, projection.ZatcaClass = stateDisplay(projection.ZatcaState, "zatca")
	projection.OverallLabel, projection.OverallClass = stateDisplay(projection.OverallState, "overall")
	return projection
}

func normalizeState(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer("-", "_", " ", "_").Replace(value)
	value = strings.Trim(value, "_")
	switch value {
	case "bill_is_under_process", "in_progress", "processing", "queued", "submitted", "pending":
		return "pending"
	case "retry", "retryable", "retrying", "backoff":
		return "retrying"
	case "ambiguous", "unknown", "result_unknown", "timeout", "timed_out", "provider_timeout":
		return "result_unknown"
	case "action_required", "requires_action", "needs_action", "manual_review", "needs_review":
		return "action_required"
	case "rejected", "failed", "failure", "terminal_failure", "declined", "error":
		return "rejected"
	case "accepted_with_warning", "accepted_with_warnings", "warning", "warning_accepted":
		return "accepted_with_warning"
	case "accepted", "success", "ok", "cleared", "reported":
		return "accepted"
	case "bill_is_issued_to_zatca", "credit_is_issued_to_zatca":
		return "issued"
	case "bill_is_processed", "processed":
		return "processed"
	default:
		return value
	}
}

func stateDisplay(state, domain string) (string, string) {
	switch state {
	case "draft":
		return "مسودة", "badge-draft"
	case "pending":
		return "قيد المعالجة", "badge-pending"
	case "retrying":
		return "إعادة المحاولة", "badge-pending"
	case "result_unknown":
		return "نتيجة غير معروفة", "badge-danger"
	case "action_required":
		return "يتطلب إجراء", "badge-warning"
	case "rejected":
		return "مرفوضة", "badge-danger"
	case "accepted":
		return "مقبولة", "badge-issued"
	case "accepted_with_warning":
		return "مقبولة مع تحذير", "badge-warning"
	case "issued":
		return "صادرة", "badge-issued"
	case "issued_with_warning":
		return "صادرة مع تحذير", "badge-warning"
	case "processed":
		return "تمت المعالجة", "badge-issued"
	case "completed":
		return "مكتملة", "badge-issued"
	case "paid":
		return "مدفوعة", "badge-issued"
	case "partially_paid":
		return "مدفوعة جزئياً", "badge-warning"
	case "unpaid":
		return "غير مدفوعة", "badge-pending"
	case "overdue":
		return "متأخرة", "badge-danger"
	case "cancelled":
		return "ملغاة", "badge-danger"
	case "unknown", "":
		return "غير معروفة", "badge-danger"
	default:
		if domain == "payment" {
			return state, "badge-pending"
		}
		return state, "badge-gray"
	}
}

func isAcceptedState(state string) bool {
	switch state {
	case "accepted", "accepted_with_warning", "issued", "processed", "completed", "posted":
		return true
	default:
		return false
	}
}

func isInvoiceComplete(state string) bool {
	return isAcceptedState(state)
}

func hasState(states []string, want string) bool {
	for _, state := range states {
		if state == want {
			return true
		}
	}
	return false
}

func hasAnyState(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func rawMap(value interface{}) (map[string]interface{}, bool) {
	m, ok := value.(map[string]interface{})
	return m, ok
}

func readStateValue(maps []map[string]interface{}, keys ...string) (string, bool) {
	for _, m := range maps {
		for key, value := range m {
			for _, candidate := range keys {
				if equivalentStateKey(key, candidate) {
					if text := stateString(value); text != "" {
						return text, true
					}
				}
			}
		}
	}
	return "", false
}

func firstStateString(maps []map[string]interface{}, keys ...string) string {
	value, _ := readStateValue(maps, keys...)
	return value
}

func stateString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case map[string]interface{}:
		for _, key := range []string{"status", "state", "code", "name", "label", "value"} {
			if text := stateString(v[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func equivalentStateKey(left, right string) bool {
	normalize := func(value string) string {
		value = strings.ToLower(value)
		value = strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)
		return value
	}
	return normalize(left) == normalize(right)
}

func boolState(maps []map[string]interface{}, keys ...string) bool {
	for _, m := range maps {
		for key, value := range m {
			for _, candidate := range keys {
				if !equivalentStateKey(key, candidate) {
					continue
				}
				switch v := value.(type) {
				case bool:
					if v {
						return true
					}
				case string:
					if strings.EqualFold(strings.TrimSpace(v), "true") || v == "1" {
						return true
					}
				case float64:
					if v == 1 {
						return true
					}
				}
			}
		}
	}
	return false
}

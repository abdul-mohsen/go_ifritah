package helpers

import (
	"testing"

	"afrita/models"
)

func TestBillStateProjectionFromRawCoversDurableStates(t *testing.T) {
	tests := []struct {
		name        string
		zatca       string
		wantOverall string
	}{
		{name: "pending", zatca: "pending", wantOverall: "pending"},
		{name: "retrying", zatca: "retrying", wantOverall: "retrying"},
		{name: "result unknown", zatca: "result_unknown", wantOverall: "result_unknown"},
		{name: "action required", zatca: "action_required", wantOverall: "action_required"},
		{name: "rejected", zatca: "rejected", wantOverall: "rejected"},
		{name: "accepted", zatca: "accepted", wantOverall: "issued"},
		{name: "accepted with warning", zatca: "accepted_with_warning", wantOverall: "issued_with_warning"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projection := BillStateProjectionFromRaw(map[string]interface{}{
				"state": 3,
				"state_projection": map[string]interface{}{
					"sale":    "completed",
					"payment": "paid",
					"invoice": "issued",
					"zatca":   tc.zatca,
				},
			})
			if projection == nil {
				t.Fatal("expected durable projection")
			}
			state := normalizeBillState(*projection)
			if state.OverallState != tc.wantOverall {
				t.Fatalf("overall state = %q, want %q", state.OverallState, tc.wantOverall)
			}
		})
	}
}

func TestProjectBillStatePrefersDurableRejectedOverLegacyIssued(t *testing.T) {
	invoice := models.Invoice{
		State: 3,
		StateProjection: &models.BillStateProjection{
			InvoiceStatus: "issued",
			ZatcaStatus:   "rejected",
			Durable:       true,
		},
	}

	state := ProjectBillState(invoice)
	if state.OverallState != "rejected" {
		t.Fatalf("overall state = %q, want rejected", state.OverallState)
	}
	if state.Issued {
		t.Fatal("rejected durable state must not be issued")
	}
	if state.CanDuplicate || state.CanReplace {
		t.Fatal("replacement actions must remain disabled for rejected durable state")
	}
}

func TestProjectBillStateUsesLegacyNumericOnlyWithoutDurableFields(t *testing.T) {
	state := ProjectBillState(models.Invoice{State: 3})
	if !state.Legacy || state.Durable {
		t.Fatalf("legacy projection flags = legacy:%v durable:%v", state.Legacy, state.Durable)
	}
	if state.OverallState != "issued" || !state.Issued {
		t.Fatalf("legacy state = %#v, want issued", state)
	}
}

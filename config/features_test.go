package config

import "testing"

func TestGetFeature(t *testing.T) {
	feature := GetFeature(FeatureSupplierLedger)
	if feature == nil {
		t.Fatal("GetFeature returned nil for supplier_ledger")
	}
	if feature.MinPlan != PlanGrowth {
		t.Fatalf("supplier_ledger minimum plan = %q, want %q", feature.MinPlan, PlanGrowth)
	}

	monitor := GetFeature(FeatureZATCAMonitor)
	if monitor == nil {
		t.Fatal("GetFeature returned nil for zatca_monitor")
	}
	if monitor.MinPlan != PlanBusiness {
		t.Fatalf("zatca_monitor minimum plan = %q, want %q", monitor.MinPlan, PlanBusiness)
	}

	if GetFeature("missing_feature") != nil {
		t.Fatal("GetFeature returned an entry for an unknown feature")
	}
}

func TestPlanLevel(t *testing.T) {
	tests := []struct {
		name  string
		plan  string
		level int
	}{
		{name: "solo", plan: PlanSolo, level: 1},
		{name: "growth", plan: PlanGrowth, level: 2},
		{name: "business", plan: PlanBusiness, level: 3},
		{name: "enterprise", plan: PlanEnterprise, level: 4},
		{name: "unknown", plan: "unknown", level: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlanLevel(tt.plan); got != tt.level {
				t.Fatalf("PlanLevel(%q) = %d, want %d", tt.plan, got, tt.level)
			}
		})
	}
}

package config

import "testing"

func TestIsEnabledTemplateFuncUsesRegisteredChecker(t *testing.T) {
	previous := featureChecker
	t.Cleanup(func() { featureChecker = previous })

	RegisterFeatureChecker(func(tenantID, featureID string) bool {
		return tenantID == "growth-tenant" && featureID == FeatureSupplierLedger
	})

	check, ok := templateFuncs["isEnabled"].(func(string, string) bool)
	if !ok {
		t.Fatal("isEnabled template function is not registered")
	}
	if !check("growth-tenant", FeatureSupplierLedger) {
		t.Fatal("isEnabled returned false for an enabled feature")
	}
	if check("solo-tenant", FeatureSupplierLedger) {
		t.Fatal("isEnabled returned true for a disabled feature")
	}
}

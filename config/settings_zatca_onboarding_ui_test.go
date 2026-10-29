package config

import (
	"strings"
	"testing"
)

func TestT012ZatcaOnboardingStateUIContract(t *testing.T) {
	source := settingsTemplateSource(t)

	requiredMarkers := []struct {
		name   string
		marker string
	}{
		{"durable metadata map", "zatcaOnboardingByBranch"},
		{"durable onboarding state", "onboard_state"},
		{"durable onboarding step", "onboard_step"},
		{"retry count", "onboard_retry_count"},
		{"error code", "onboard_error_code"},
		{"invalid OTP localization", "invalid_otp"},
		{"provider failure localization", "provider_unavailable"},
		{"retry action", "retryZatcaOnboarding"},
		{"poll registry", "zatcaOnboardPolls"},
		{"poll cleanup helper", "stopZatcaOnboardPolling"},
		{"terminal failure handling", "state === 'failed'"},
		{"terminal completion handling", "state === 'done'"},
		{"localized progress labels", "zatca_step_production"},
	}

	for _, marker := range requiredMarkers {
		if !strings.Contains(source, marker.marker) {
			t.Errorf("%s marker %q is missing from settings template", marker.name, marker.marker)
		}
	}

	if !strings.Contains(source, "onboardingState !== 'done'") &&
		!strings.Contains(source, "state !== 'done'") {
		t.Error("active status must be guarded by durable done state")
	}
	if !strings.Contains(source, "clearInterval(zatcaOnboardPolls[branchId])") {
		t.Error("terminal onboarding state must clear the branch polling interval")
	}
	if !strings.Contains(source, "zatca-otp-retry") {
		t.Error("terminal failure must expose a retry control")
	}
}

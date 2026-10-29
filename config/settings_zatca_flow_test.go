package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func settingsTemplateSource(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Dir(filepath.Dir(filename))
	source, err := os.ReadFile(filepath.Join(root, "templates", "settings.html"))
	if err != nil {
		t.Fatalf("read settings template: %v", err)
	}
	return string(source)
}

func javascriptSection(t *testing.T, source, start, end string) string {
	t.Helper()

	startAt := strings.Index(source, start)
	if startAt == -1 {
		t.Fatalf("settings template is missing %q", start)
	}
	endAt := strings.Index(source[startAt+len(start):], end)
	if endAt == -1 {
		t.Fatalf("settings template is missing section terminator %q", end)
	}
	return source[startAt : startAt+len(start)+endAt]
}

func TestT028ZatcaOnboardingWaitsForCompleteSave(t *testing.T) {
	source := settingsTemplateSource(t)
	saveFn := javascriptSection(
		t,
		source,
		"window.saveZatcaConfig = function()",
		"window.switchZatcaBranch = function",
	)
	openFn := javascriptSection(
		t,
		source,
		"window.openZatcaOtpModal = function()",
		"window.closeZatcaOtpModal = function",
	)
	submitFn := javascriptSection(
		t,
		source,
		"window.submitZatcaOtp = function()",
		"// Poll backend GET",
	)
	readinessFn := javascriptSection(
		t,
		source,
		"function checkZatcaConnectReady()",
		"function updateStepperIcons",
	)

	if !strings.Contains(saveFn, "zatcaSavePending") {
		t.Fatal("ZATCA save must expose a pending state to readiness checks")
	}
	if !strings.Contains(saveFn, "return saveBranchStoreCity") {
		t.Fatal("ZATCA save must await and return the linked store-address save")
	}
	if !strings.Contains(saveFn, "if (!storeSaved) throw new Error") {
		t.Fatal("linked store-address failures must reject the complete save")
	}
	if !strings.Contains(saveFn, "otpSubmitBtn.disabled = true") {
		t.Fatal("OTP submit control must stay disabled while the prerequisite save is pending")
	}
	otpSubmitAt := strings.Index(source, `id="zatca-otp-submit"`)
	if otpSubmitAt == -1 || !strings.Contains(source[otpSubmitAt:otpSubmitAt+300], "disabled") {
		t.Fatal("OTP submit control must be disabled until the prerequisite flow opens it")
	}
	if !strings.Contains(readinessFn, "&& !zatcaSavePending") {
		t.Fatal("Connect readiness must include the pending-save guard")
	}
	if !strings.Contains(submitFn, "if (zatcaSavePending)") {
		t.Fatal("OTP submission must guard against a pending prerequisite save")
	}

	storeSaveAt := strings.Index(saveFn, "saveBranchStoreCity")
	savedStateAt := strings.Index(saveFn, "zatcaConfigSaved = true")
	if storeSaveAt == -1 || savedStateAt == -1 || storeSaveAt > savedStateAt {
		t.Fatal("ZATCA config cannot be marked saved before the store-address save")
	}

	saveWaitAt := strings.Index(openFn, "saveZatcaConfig().then")
	modalOpenAt := strings.Index(openFn, "if (saved) openModal()")
	if saveWaitAt == -1 || modalOpenAt == -1 || saveWaitAt > modalOpenAt {
		t.Fatal("OTP modal must open only after the complete ZATCA save resolves")
	}
}

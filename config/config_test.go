package config

import (
	"os"
	"path/filepath"
	"testing"
)

const releaseChannel = "release"

func TestLoadAppVersionPrefersEnv(t *testing.T) {
	t.Setenv("APP_IMAGE_VERSION", "")
	t.Setenv("APP_VERSION", "v2.3.4")

	if got := loadAppVersion(); got != "v2.3.4" {
		t.Fatalf("loadAppVersion() = %q, want v2.3.4", got)
	}
}

func TestLoadAppVersionSeparatesImageTag(t *testing.T) {
	t.Setenv("APP_VERSION", "v2.3.4")
	t.Setenv("APP_IMAGE_VERSION", defaultAppChannel)
	t.Setenv("APP_IMAGE_TAG", defaultAppChannel)

	if got := loadAppVersion(); got != "v2.3.4" {
		t.Fatalf("loadAppVersion() = %q, want semantic version", got)
	}
	if got := loadAppImageTag(); got != defaultAppChannel {
		t.Fatalf("loadAppImageTag() = %q, want selected Docker tag", got)
	}
}

func TestLoadAppVersionReadsVersionFile(t *testing.T) {
	t.Setenv("APP_IMAGE_VERSION", "")
	t.Setenv("APP_VERSION", "")

	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "VERSION"), []byte("v1.2.3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	chdir(t, tempDir)

	if got := loadAppVersion(); got != "v1.2.3" {
		t.Fatalf("loadAppVersion() = %q, want v1.2.3", got)
	}
}

func TestLoadAppVersionFallback(t *testing.T) {
	t.Setenv("APP_IMAGE_VERSION", "")
	t.Setenv("APP_VERSION", "")
	chdir(t, t.TempDir())

	if got := loadAppVersion(); got != defaultAppVersion {
		t.Fatalf("loadAppVersion() = %q, want %s", got, defaultAppVersion)
	}
}

func TestLoadAppChannel(t *testing.T) {
	t.Setenv("APP_IMAGE_CHANNEL", "")
	t.Setenv("APP_CHANNEL", releaseChannel)

	if got := loadAppChannel(); got != releaseChannel {
		t.Fatalf("loadAppChannel() = %q, want release", got)
	}
}

func TestLoadAppChannelFallback(t *testing.T) {
	t.Setenv("APP_IMAGE_CHANNEL", "")
	t.Setenv("APP_CHANNEL", "")

	if got := loadAppChannel(); got != defaultAppChannel {
		t.Fatalf("loadAppChannel() = %q, want %s", got, defaultAppChannel)
	}
}

func TestLoadAppCommit(t *testing.T) {
	t.Setenv("APP_IMAGE_COMMIT", "")
	t.Setenv("APP_COMMIT", "abc123")

	if got := loadAppCommit(); got != "abc123" {
		t.Fatalf("loadAppCommit() = %q, want abc123", got)
	}
}

func TestLoadAppCommitFallback(t *testing.T) {
	t.Setenv("APP_IMAGE_COMMIT", "")
	t.Setenv("APP_COMMIT", "")

	if got := loadAppCommit(); got != "unknown" {
		t.Fatalf("loadAppCommit() = %q, want unknown", got)
	}
}

func TestLoadAppCommitShortDerivesFromCommit(t *testing.T) {
	t.Setenv("APP_IMAGE_COMMIT_SHORT", "")
	t.Setenv("APP_COMMIT_SHORT", "")
	t.Setenv("APP_COMMIT", "abcdef0123456789")

	if got := loadAppCommitShort(); got != "abcdef0" {
		t.Fatalf("loadAppCommitShort() = %q, want abcdef0", got)
	}
}

func TestLoadAppDeploymentAliases(t *testing.T) {
	t.Setenv("APP_IMAGE_CHANNEL", "")
	t.Setenv("APP_CHANNEL", "")
	t.Setenv("APP_BUILD_CHANNEL", releaseChannel)
	t.Setenv("APP_WORKFLOW_RUN_ID", "")
	t.Setenv("APP_WORKFLOW_RUN", "https://github.com/acme/ifritah/actions/runs/42")
	t.Setenv("APP_WORKFLOW_RUN_URL", "")
	t.Setenv("APP_CREATED", "")
	t.Setenv("APP_BUILT_AT", "2026-09-07T12:00:00Z")

	if got := loadAppChannel(); got != releaseChannel {
		t.Fatalf("loadAppChannel() = %q, want release", got)
	}
	if got := loadAppWorkflowRun(); got != "42" {
		t.Fatalf("loadAppWorkflowRun() = %q, want numeric workflow ID", got)
	}
	if got := loadAppWorkflowURL(); got != "https://github.com/acme/ifritah/actions/runs/42" {
		t.Fatalf("loadAppWorkflowURL() = %q, want workflow URL", got)
	}
	if got := loadAppBuiltAt(); got != "2026-09-07T12:00:00Z" {
		t.Fatalf("loadAppBuiltAt() = %q, want build timestamp", got)
	}
}

func TestLoadAppCanonicalIdentity(t *testing.T) {
	t.Setenv("APP_VERSION", "v9.8.7")
	t.Setenv("APP_IMAGE_VERSION", defaultAppChannel)
	t.Setenv("APP_IMAGE_CHANNEL", releaseChannel)
	t.Setenv("APP_IMAGE_COMMIT", "canonical-commit")
	t.Setenv("APP_IMAGE_COMMIT_SHORT", "canonic")
	t.Setenv("APP_IMAGE_TAG", defaultAppChannel)
	t.Setenv("APP_WORKFLOW_RUN_ID", "456")
	t.Setenv("APP_WORKFLOW_RUN_URL", "https://example.test/actions/runs/456")
	t.Setenv("APP_BUILT_AT", "2026-09-07T12:00:00Z")

	if got := loadAppVersion(); got != "v9.8.7" {
		t.Fatalf("loadAppVersion() = %q, want canonical version", got)
	}
	if got := loadAppChannel(); got != releaseChannel {
		t.Fatalf("loadAppChannel() = %q, want canonical channel", got)
	}
	if got := loadAppCommit(); got != "canonical-commit" || loadAppCommitShort() != "canonic" {
		t.Fatalf("canonical commit identity = %q/%q", loadAppCommit(), loadAppCommitShort())
	}
	if got := loadAppWorkflowRun(); got != "456" || loadAppWorkflowURL() == "" {
		t.Fatalf("canonical workflow identity = %q/%q", got, loadAppWorkflowURL())
	}
	if got := loadAppImageTag(); got != defaultAppChannel {
		t.Fatalf("loadAppImageTag() = %q, want canonical image tag", got)
	}
}

func TestLoadAppImageTagFromImageRef(t *testing.T) {
	t.Setenv("APP_IMAGE_TAG", "")
	t.Setenv("APP_DOCKER_TAG", "")
	t.Setenv("APP_IMAGE_REF", "ssdawweq/ifritah-web:"+defaultAppChannel)

	if got := loadAppImageTag(); got != defaultAppChannel {
		t.Fatalf("loadAppImageTag() = %q, want %s", got, defaultAppChannel)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatal(err)
		}
	})
}

func TestFormatSAR_Zero(t *testing.T) {
	result := formatSAR(0)
	expected := "٠٫٠٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_Simple(t *testing.T) {
	result := formatSAR(123.45)
	expected := "١٢٣٫٤٥ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_Thousands(t *testing.T) {
	result := formatSAR(1234.50)
	expected := "١٬٢٣٤٫٥٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_LargeNumber(t *testing.T) {
	result := formatSAR(1234567.89)
	expected := "١٬٢٣٤٬٥٦٧٫٨٩ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_Negative(t *testing.T) {
	result := formatSAR(-500.00)
	expected := "-٥٠٠٫٠٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_WholeNumber(t *testing.T) {
	result := formatSAR(1000)
	expected := "١٬٠٠٠٫٠٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_SmallDecimal(t *testing.T) {
	result := formatSAR(0.99)
	expected := "٠٫٩٩ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestToFloat64_Conversion(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{float64(1.5), 1.5},
		{float32(2.5), 2.5},
		{int(10), 10.0},
		{int64(20), 20.0},
		{int32(30), 30.0},
		{"not a number", 0.0},
		{nil, 0.0},
	}
	for _, tc := range tests {
		result := toFloat64(tc.input)
		if result != tc.expected {
			t.Errorf("toFloat64(%v) = %f, want %f", tc.input, result, tc.expected)
		}
	}
}

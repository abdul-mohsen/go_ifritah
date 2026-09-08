package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployWorkflowPublishesDevAndImmutableTags(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "deploy.yml")

	assertContains(t, workflow, "IMAGE_NAME: ssdawweq/ifritah-web")
	assertContains(t, workflow, "type=raw,value=dev,enable=${{ github.ref == 'refs/heads/dev' }}")
	assertContains(t, workflow, "type=raw,value=${{ steps.build.outputs.short_sha }}")
	assertContains(t, workflow, "type=raw,value=${{ github.sha }}")
	assertContains(t, workflow, "echo \"docker_tag=${docker_tag}\"")
	assertContains(t, workflow, "echo \"workflow_run_id=${GITHUB_RUN_ID}\"")
	assertContains(t, workflow, "echo \"workflow_run_url=${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}\"")
	assertContains(t, workflow, "APP_IMAGE_TAG=${{ steps.build.outputs.docker_tag }}")
	assertContains(t, workflow, "APP_WORKFLOW_RUN_ID=${{ steps.build.outputs.workflow_run_id }}")
	assertContains(t, workflow, "APP_WORKFLOW_RUN_URL=${{ steps.build.outputs.workflow_run_url }}")
	assertContains(t, workflow, "if [[ \"${version}\" == \"v0.0.0\" ]]; then")
	assertNotContains(t, workflow, "APP_WORKFLOW_RUN_ID=${{ steps.build.outputs.workflow_run }}")
	assertNotContains(t, workflow, "com.ifritah.build.workflow_run_id=${{ steps.build.outputs.workflow_run }}")
}

func TestDockerfileCarriesSemanticVersionAndRuntimeImageTag(t *testing.T) {
	dockerfile := readRepoFile(t, "Dockerfile")

	assertContains(t, dockerfile, "ARG APP_IMAGE_TAG=dev")
	assertContains(t, dockerfile, "ENV APP_VERSION=${APP_VERSION}")
	assertContains(t, dockerfile, "ENV APP_IMAGE_TAG=${APP_IMAGE_TAG}")
	assertContains(t, dockerfile, "test \"$APP_VERSION\" != \"v0.0.0\"")
	assertContains(t, dockerfile, "test \"$APP_IMAGE_TAG\" != \"v0.0.0\"")
	assertContains(t, dockerfile, "LABEL org.opencontainers.image.version=${APP_VERSION}")
	assertContains(t, dockerfile, "LABEL org.opencontainers.image.title=\"ifritah-web\"")
	assertContains(t, dockerfile, "LABEL com.ifritah.build.version=${APP_VERSION}")
	assertContains(t, dockerfile, "LABEL com.ifritah.build.image_tag=${APP_IMAGE_TAG}")
	assertNotContains(t, dockerfile, "ENV APP_VERSION=${APP_IMAGE_VERSION}")
}

func readRepoFile(t *testing.T, pathParts ...string) string {
	t.Helper()

	fullPath := filepath.Join(append([]string{".."}, pathParts...)...)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read %s: %v", fullPath, err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected file to contain %q", needle)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if strings.Contains(haystack, needle) {
		t.Fatalf("expected file not to contain %q", needle)
	}
}

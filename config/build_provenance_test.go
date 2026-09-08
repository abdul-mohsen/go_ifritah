package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployWorkflowPublishesDevAndImmutableTags(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "deploy.yml")

	assertContains(t, workflow, "EXPECTED_IMAGE_NAME: ssdawweq/ifritah-web")
	assertContains(t, workflow, "DOCKERHUB_USERNAME: ${{ secrets.DOCKERHUB_USERNAME }}")
	assertContains(t, workflow, "image_name=\"${DOCKERHUB_USERNAME}/ifritah-web\"")
	assertContains(t, workflow, "if [[ \"${image_name}\" != \"${EXPECTED_IMAGE_NAME}\" ]]; then")
	assertContains(t, workflow, "echo \"image_name=${image_name}\"")
	assertContains(t, workflow, "images: ${{ steps.build.outputs.image_name }}")
	assertBefore(t, workflow, "name: Set build metadata", "uses: docker/login-action")
	assertBefore(t, workflow, "name: Set build metadata", "uses: docker/build-push-action")
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
	assertNotContains(t, workflow, "\n  IMAGE_NAME: ssdawweq/ifritah-web")
	assertNotContains(t, workflow, "APP_WORKFLOW_RUN_ID=${{ steps.build.outputs.workflow_run }}")
	assertNotContains(t, workflow, "com.ifritah.build.workflow_run_id=${{ steps.build.outputs.workflow_run }}")
	assertNotContains(t, workflow, "APP_IMAGE_DIGEST=")
	assertNotContains(t, workflow, "com.ifritah.build.digest=")
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
	assertContains(t, dockerfile, "ENV APP_IMAGE_DIGEST=${APP_IMAGE_DIGEST}")
	assertNotContains(t, dockerfile, "ENV APP_VERSION=${APP_IMAGE_VERSION}")
	assertNotContains(t, dockerfile, "LABEL com.ifritah.build.digest=")
}

func TestFrontendWorkflowsDoNotInjectEmptyImageDigest(t *testing.T) {
	for _, workflowName := range []string{"deploy.yml", "frontend-image.yml"} {
		t.Run(workflowName, func(t *testing.T) {
			workflow := readRepoFile(t, ".github", "workflows", workflowName)

			assertNotContains(t, workflow, "APP_IMAGE_DIGEST=")
			assertNotContains(t, workflow, "APP_IMAGE_DIGEST=''")
			assertNotContains(t, workflow, "com.ifritah.build.digest=")
		})
	}
}

func TestBranchWorkflowPublishesCanonicalIdentity(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "branch-image.yml")

	for _, needle := range []string{
		"APP_IMAGE_VERSION=${{ steps.branch_tag.outputs.version }}",
		"APP_IMAGE_TAG=${{ steps.branch_tag.outputs.tag }}",
		"APP_IMAGE_REF=${{ env.IMAGE_NAME }}:${{ steps.branch_tag.outputs.tag }}",
		"com.ifritah.build.channel=dev",
		"com.ifritah.build.tag=${{ steps.branch_tag.outputs.tag }}",
		"com.ifritah.build.image_ref=${{ env.IMAGE_NAME }}:${{ steps.branch_tag.outputs.tag }}",
		"com.ifritah.build.workflow_run_id=${{ github.run_id }}",
	} {
		assertContains(t, workflow, needle)
	}
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

func assertBefore(t *testing.T, haystack, before, after string) {
	t.Helper()

	beforeIndex := strings.Index(haystack, before)
	if beforeIndex < 0 {
		t.Fatalf("expected file to contain %q", before)
	}
	afterIndex := strings.Index(haystack, after)
	if afterIndex < 0 {
		t.Fatalf("expected file to contain %q", after)
	}
	if beforeIndex >= afterIndex {
		t.Fatalf("expected %q to appear before %q", before, after)
	}
}

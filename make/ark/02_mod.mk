GITHUB_OUTPUT ?= /dev/stderr
.PHONY: ark-release
## Publish all release artifacts (image + helm chart)
## @category CyberArk Discovery and Context
ark-release: oci_ark_image_name := $(OCI_BASE)/images/cyberark-disco-agent
ark-release: OCI_SIGN_ON_PUSH := false
ark-release: oci_platforms := linux/amd64
ark-release: helm_chart_source_dir := deploy/charts/cyberark-disco-agent
ark-release: helm_chart_image_name := $(OCI_BASE)/charts/cyberark-disco-agent
ark-release: helm_chart_version := $(helm_chart_version)
ark-release: oci_ark_image_digest_path := $(bin_dir)/scratch/image/oci-layout-ark.digests
ark-release: helm_digest_path := $(bin_dir)/scratch/helm/cyberark-disco-agent-$(helm_chart_version).digests
ark-release:
	$(MAKE) oci-push-ark helm-chart-oci-push \
		oci_ark_image_name="$(oci_ark_image_name)" \
		OCI_SIGN_ON_PUSH="$(OCI_SIGN_ON_PUSH)" \
		oci_platforms="$(oci_platforms)" \
		helm_image_name="$(oci_ark_image_name)" \
		helm_image_tag="$(oci_ark_image_tag)" \
		helm_chart_source_dir="$(helm_chart_source_dir)" \
		helm_chart_image_name="$(helm_chart_image_name)"

	@echo "RELEASE_OCI_IMAGE=$(oci_ark_image_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_IMAGE_TAG=$(oci_ark_image_tag)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_IMAGE_DIGEST=$$(head -1 $(oci_ark_image_digest_path))" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_CHART=$(helm_chart_image_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_CHART_TAG=$(helm_chart_version)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_CHART_DIGEST=$$(head -1 $(helm_digest_path))" >> "$(GITHUB_OUTPUT)"

	@echo "Release complete!"

.PHONY: ark-test-e2e
## Run a basic E2E test on a Kind cluster
## See `hack/ark/e2e.sh` for the full test script.
## @category CyberArk Discovery and Context
ark-test-e2e: $(NEEDS_KIND) $(NEEDS_KUBECTL) $(NEEDS_HELM)
	PATH="$(bin_dir)/tools:${PATH}" ./hack/ark/test-e2e.sh

.PHONY: ark-verify
## Verify the Helm chart
## @category CyberArk Discovery and Context
ark-verify:
	$(MAKE) verify-helm-lint verify-helm-values verify-pod-security-standards verify-helm-kubeconform\
		helm_chart_source_dir=deploy/charts/cyberark-disco-agent \
		helm_chart_image_name=$(OCI_BASE)/charts/cyberark-disco-agent

shared_verify_targets_dirty += ark-verify

.PHONY: ark-generate
## Generate Helm chart documentation and schema
## @category CyberArk Discovery and Context
ark-generate:
	$(MAKE) generate-helm-docs generate-helm-schema \
		helm_chart_source_dir=deploy/charts/cyberark-disco-agent

shared_generate_targets += ark-generate


include make/test-unit.mk

.PHONY: release
## Publish all release artifacts (image + helm chart)
## @category [shared] Release
release: $(helm_chart_archive)
	$(MAKE) oci-push-preflight

	@echo "RELEASE_OCI_preflight_IMAGE=$(oci_preflight_image_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_preflight_TAG=$(oci_preflight_image_tag)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_NAME=$(helm_chart_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_VERSION=$(helm_chart_version)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_TAR=$(helm_chart_archive)" >> "$(GITHUB_OUTPUT)"

	@echo "Release complete!"

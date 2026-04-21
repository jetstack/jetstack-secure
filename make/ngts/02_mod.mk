# Makefile targets for NGTS Discovery Agent

# The base OCI repository for all NGTS Discovery Agent artifacts
NGTS_OCI_BASE ?= quay.io/jetstack

# The OCI repository (without tag) for the NGTS Discovery Agent Docker image
# Can be overridden when calling `make ngts-release` to push to a different repository.
NGTS_IMAGE ?= $(NGTS_OCI_BASE)/discovery-agent

# The OCI repository (without tag) for the NGTS Discovery Agent Helm chart
# Can be overridden when calling `make ngts-release` to push to a different repository.
NGTS_CHART ?= $(NGTS_OCI_BASE)/charts/discovery-agent

# Used to output variables when running in GitHub Actions
GITHUB_OUTPUT ?= /dev/stderr

.PHONY: ngts-release
## Publish all release artifacts (image + helm chart)
## @category NGTS Discovery Agent
ngts-release: oci_ngts_image_digest_path := $(bin_dir)/scratch/image/oci-layout-ngts.digests
ngts-release: helm_digest_path := $(bin_dir)/scratch/helm/discovery-agent-$(helm_chart_version).digests
ngts-release:
	$(MAKE) oci-push-ngts helm-chart-oci-push \
		oci_ngts_image_name="$(NGTS_IMAGE)" \
		helm_image_name="$(NGTS_IMAGE)" \
		helm_image_tag="$(oci_ngts_image_tag)" \
		helm_chart_source_dir=deploy/charts/discovery-agent \
		helm_chart_image_name="$(NGTS_CHART)"

	@echo "NGTS_IMAGE=$(NGTS_IMAGE)" >> "$(GITHUB_OUTPUT)"
	@echo "NGTS_IMAGE_TAG=$(oci_ngts_image_tag)" >> "$(GITHUB_OUTPUT)"
	@echo "NGTS_IMAGE_DIGEST=$$(head -1 $(oci_ngts_image_digest_path))" >> "$(GITHUB_OUTPUT)"
	@echo "NGTS_CHART=$(NGTS_CHART)" >> "$(GITHUB_OUTPUT)"
	@echo "NGTS_CHART_TAG=$(helm_chart_version)" >> "$(GITHUB_OUTPUT)"
	@echo "NGTS_CHART_DIGEST=$$(head -1 $(helm_digest_path))" >> "$(GITHUB_OUTPUT)"

	@echo "Release complete!"

.PHONY: ngts-test-e2e
## Run a basic E2E test on a Kind cluster
## See `hack/ngts/e2e.sh` for the full test script.
## @category NGTS Discovery Agent
ngts-test-e2e: $(NEEDS_KIND) $(NEEDS_KUBECTL) $(NEEDS_HELM) $(NEEDS_YQ)
	PATH="$(bin_dir)/tools:${PATH}" ./hack/ngts/test-e2e.sh

.PHONY: ngts-verify
## Verify the Helm chart
## @category NGTS Discovery Agent
ngts-verify:
	INSTALL_OPTIONS="--set-string config.tsgID=1234123412 --set config.clusterName=foo" $(MAKE) verify-helm-lint verify-helm-values verify-pod-security-standards verify-helm-kubeconform \
		helm_chart_source_dir=deploy/charts/discovery-agent \
		helm_chart_image_name=$(NGTS_CHART)

shared_verify_targets += ngts-verify

.PHONY: ngts-generate
## Generate Helm chart documentation and schema
## @category NGTS Discovery Agent
ngts-generate:
	$(MAKE) generate-helm-docs generate-helm-schema \
		helm_chart_source_dir=deploy/charts/discovery-agent

shared_generate_targets += ngts-generate

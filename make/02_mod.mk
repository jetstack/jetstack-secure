include make/test-unit.mk
include make/ark/02_mod.mk

GITHUB_OUTPUT ?= /dev/stderr
.PHONY: release
## Publish all release artifacts (image + helm chart)
## @category [shared] Release
release:
	$(MAKE) oci-push-preflight
	$(MAKE) helm-chart-oci-push

	@echo "RELEASE_OCI_PREFLIGHT_IMAGE=$(oci_preflight_image_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_PREFLIGHT_TAG=$(oci_preflight_image_tag)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_IMAGE=$(helm_chart_image_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_VERSION=$(helm_chart_version)" >> "$(GITHUB_OUTPUT)"

	@echo "Release complete!"

.PHONY: generate-crds-venconn
## Pulls the VenafiConnection CRD from the venafi-connection-lib Go module.
## @category [shared] Generate/ Verify
#
# We aren't using "generate-crds" because "generate-crds" only work for projects
# from which controller-gen can be used to generate the plain CRDs (plain CRDs =
# the non-templated CRDs). In this project, we generate the plain CRDs using `go
# run ./make/connection_crd` instead.
generate-crds-venconn: $(addprefix $(helm_chart_source_dir)/templates/,venafi-connection-crd.yaml venafi-connection-crd.without-validations.yaml)

$(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml: go.mod | $(NEEDS_GO)
	echo "# DO NOT EDIT: Use 'make generate-crds-venconn' to regenerate." >$@
	$(GO) run ./make/connection_crd >>$@

$(helm_chart_source_dir)/templates/venafi-connection-crd.without-validations.yaml: $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml $(helm_chart_source_dir)/crd_bases/crd.header.yaml $(helm_chart_source_dir)/crd_bases/crd.footer.yaml | $(NEEDS_YQ)
	cat $(helm_chart_source_dir)/crd_bases/crd.header-without-validations.yaml >$@
	$(YQ) -I2 '{"spec": .spec}' $< | $(YQ) 'del(.. | ."x-kubernetes-validations"?) | del(.metadata.creationTimestamp)' | grep -v "DO NOT EDIT" >>$@
	cat $(helm_chart_source_dir)/crd_bases/crd.footer.yaml >>$@

$(helm_chart_source_dir)/templates/venafi-connection-crd.yaml: $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml $(helm_chart_source_dir)/crd_bases/crd.header.yaml $(helm_chart_source_dir)/crd_bases/crd.footer.yaml | $(NEEDS_YQ)
	cat $(helm_chart_source_dir)/crd_bases/crd.header.yaml >$@
	$(YQ) -I2 '{"spec": .spec}' $< | $(YQ) 'del(.metadata.creationTimestamp)' | grep -v "DO NOT EDIT" >>$@
	cat $(helm_chart_source_dir)/crd_bases/crd.footer.yaml >>$@

# The generate-crds target doesn't need to be run anymore when running
# "generate". Let's replace it with "generate-crds-venconn".
shared_generate_targets := $(filter-out generate-crds,$(shared_generate_targets))
shared_generate_targets += generate-crds-venconn

.PHONY: test-e2e-gke
## Run a basic E2E test on a GKE cluster
## Build and install venafi-kubernetes-agent for VenafiConnection based authentication.
## Wait for it to log a message indicating successful data upload.
## See `hack/e2e/test.sh` for the full test script.
## @category Testing
test-e2e-gke: | $(NEEDS_HELM) $(NEEDS_STEP) $(NEEDS_VENCTL)
	COVERAGE_HOST_PATH="$(COVERAGE_HOST_PATH)" ./hack/e2e/test.sh
	#./hack/e2e/test.sh

.PHONY: test-helm
## Run `helm unittest`.
## @category Testing
test-helm: | $(NEEDS_HELM-UNITTEST)
	$(HELM-UNITTEST) ./deploy/charts/{venafi-kubernetes-agent,disco-agent}

.PHONY: test-helm-snapshot
## Update the `helm unittest` snapshots.
## @category Testing
test-helm-snapshot: | $(NEEDS_HELM-UNITTEST)
	$(HELM-UNITTEST) ./deploy/charts/{venafi-kubernetes-agent,disco-agent} -u

.PHONY: helm-plugins
## Install required helm plugins
helm-plugins: $(NEEDS_HELM)
	@if ! $(HELM) plugin list | grep -q diff; then \
		echo ">>> Installing helm-diff plugin"; \
		$(HELM) plugin install https://github.com/databus23/helm-diff; \
	else \
		echo "helm-diff plugin already installed"; \
	fi

# https://docs.venafi.cloud/vaas/venctl/c-venctl-releases/
venctl_linux_amd64_SHA256SUM=26e7b7a7e134f1cf1f3ffacf4ae53ec6849058db5007ce4088d51f404ededb4a
venctl_darwin_amd64_SHA256SUM=2e76693901abcb2c018f66d3a10558c66ca09d1a3be912258bcd6c58e89aae80
venctl_darwin_arm64_SHA256SUM=4350912d67683773302655e2a0151320514d1ccf82ee99c895e6780f86b6f031

.PRECIOUS: $(DOWNLOAD_DIR)/tools/venctl@$(VENCTL_VERSION)_$(HOST_OS)_$(HOST_ARCH)
$(DOWNLOAD_DIR)/tools/venctl@$(VENCTL_VERSION)_$(HOST_OS)_$(HOST_ARCH): | $(DOWNLOAD_DIR)/tools
	@source $(lock_script) $@; \
		$(CURL) https://dl.venafi.cloud/venctl/$(VENCTL_VERSION)/venctl-$(HOST_OS)-$(HOST_ARCH).zip -o $(outfile).zip; \
		$(checkhash_script) $(outfile).zip $(venctl_$(HOST_OS)_$(HOST_ARCH)_SHA256SUM); \
		unzip -p $(outfile).zip venctl > $(outfile); \
		chmod +x $(outfile); \
		rm -f $(outfile).zip

# https://github.com/smallstep/cli/releases/
step_linux_amd64_SHA256SUM=2908f3c7d90181eec430070b231da5c0861e37537bf8e2388d031d3bd6c7b8c6
step_linux_arm64_SHA256SUM=96636a6cc980d53a98c72aa3b99e04f0b874a733d9ddf43fc6b0f1725f425c37
step_darwin_amd64_SHA256SUM=f6e9a9078cfc5f559c8213e023df6e8ebf8d9d36ffbd82749a41ee1c40a23623
step_darwin_arm64_SHA256SUM=b856702ee138a9badbe983e88758c0330907ea4f97e429000334ba038597db5b

.PRECIOUS: $(DOWNLOAD_DIR)/tools/step@$(STEP_VERSION)_$(HOST_OS)_$(HOST_ARCH)
$(DOWNLOAD_DIR)/tools/step@$(STEP_VERSION)_$(HOST_OS)_$(HOST_ARCH): | $(DOWNLOAD_DIR)/tools
	@source $(lock_script) $@; \
		$(CURL) https://dl.smallstep.com/gh-release/cli/gh-release-header/v$(STEP_VERSION)/step_$(HOST_OS)_$(STEP_VERSION)_$(HOST_ARCH).tar.gz -o $(outfile).tar.gz; \
		$(checkhash_script) $(outfile).tar.gz $(step_$(HOST_OS)_$(HOST_ARCH)_SHA256SUM); \
		tar xfO $(outfile).tar.gz step_$(STEP_VERSION)/bin/step > $(outfile); \
		chmod +x $(outfile); \
		rm -f $(outfile).tar.gz

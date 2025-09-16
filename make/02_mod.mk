include make/test-unit.mk

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
test-e2e-gke:
	@ls -A $(CURDIR)/hack/e2e
	$(CURDIR)/hack/e2e/test.sh

.PHONY: test-helm
## Run `helm unittest`.
## @category Testing
test-helm: | $(NEEDS_HELM-UNITTEST)
	$(HELM-UNITTEST) ./deploy/charts/venafi-kubernetes-agent/

.PHONY: test-helm-snapshot
## Update the `helm unittest` snapshots.
## @category Testing
test-helm-snapshot: | $(NEEDS_HELM-UNITTEST)
	$(HELM-UNITTEST) ./deploy/charts/venafi-kubernetes-agent/ -u


.PHONY: verify-govulncheck
## Verify all Go modules for vulnerabilities using govulncheck Copied from makefile-modules
## @category [shared] Generate/ Verify
#
# Runs `govulncheck` on all Go modules related to the project.
# Ignores Go modules among the temporary build artifacts in _bin, to avoid
# scanning the code of the vendored Go, after running make vendor-go.
# Ignores Go modules in make/_shared, because those will be checked in centrally
# in the makefile_modules repository.
verify-govulncheck: | $(NEEDS_GOVULNCHECK)
	@find . -name go.mod -not \( -path "./$(bin_dir)/*" -or -path "./make/_shared/*" \) \
		| while read d; do \
				target=$$(dirname $${d}); \
				echo "Running 'GOTOOLCHAIN=go$(VENDORED_GO_VERSION) $(bin_dir)/tools/govulncheck ./...' in directory '$${target}'"; \
				pushd "$${target}" >/dev/null; \
				GOTOOLCHAIN=go$(VENDORED_GO_VERSION) $(GOVULNCHECK) ./... || exit; \
				popd >/dev/null; \
				echo ""; \
			done

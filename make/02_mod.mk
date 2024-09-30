include make/test-unit.mk

GITHUB_OUTPUT ?= /dev/stderr
.PHONY: release
## Publish all release artifacts (image + helm chart)
## @category [shared] Release
release: $(helm_chart_archive)
	$(MAKE) oci-push-preflight
	$(HELM) push "$(helm_chart_archive)" "$(helm_chart_repo_base)"

	@echo "RELEASE_OCI_preflight_IMAGE=$(oci_preflight_image_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_OCI_preflight_TAG=$(oci_preflight_image_tag)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_NAME=$(helm_chart_name)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_VERSION=$(helm_chart_version)" >> "$(GITHUB_OUTPUT)"
	@echo "RELEASE_HELM_CHART_TAR=$(helm_chart_archive)" >> "$(GITHUB_OUTPUT)"

	@echo "Release complete!"

.PHONY: generate-crds-venconn
## Pulls the VenafiConnection CRD from the venafi-connection-lib Go module.
## @category [shared] Generate/ Verify
#
# We aren't using "generate-crds" because "generate-crds" only work for projects
# from which controller-gen can be used to generate the plain CRDs (plain CRDs =
# the non-templated CRDs). In this project, we generate the plain CRDs using
# `run ./make/connection_crd` instead.
generate-crds-venconn: $(addprefix $(helm_chart_source_dir)/templates/,venafi-connection-crd.yaml venafi-connection-crd.without-validations.yaml)

$(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml: go.mod | $(NEEDS_GO)
	$(GO) run ./make/connection_crd >$@

$(helm_chart_source_dir)/templates/venafi-connection-crd.without-validations.yaml: $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml $(helm_chart_source_dir)/crd_bases/crd.header.yaml $(helm_chart_source_dir)/crd_bases/crd.footer.yaml | $(NEEDS_YQ)
	cat $(helm_chart_source_dir)/crd_bases/crd.header.yaml >$@
	$(YQ) 'del(.. | ."x-kubernetes-validations"?) | del(.metadata.creationTimestamp)' $< | grep -v "DO NOT EDIT" >>$@
	cat $(helm_chart_source_dir)/crd_bases/crd.footer.yaml >>$@

$(helm_chart_source_dir)/templates/venafi-connection-crd.yaml: $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml $(helm_chart_source_dir)/crd_bases/crd.header.yaml $(helm_chart_source_dir)/crd_bases/crd.footer.yaml | $(NEEDS_YQ)
	cat $(helm_chart_source_dir)/crd_bases/crd.header.yaml >$@
	$(YQ) 'del(.metadata.creationTimestamp)' $< | grep -v "DO NOT EDIT" >>$@
	cat $(helm_chart_source_dir)/crd_bases/crd.footer.yaml >>$@

# The generate-crds target doesn't need to be run anymore when running
# "generate". Let's replace it with "generate-crds-venconn".
shared_generate_targets := $(filter-out generate-crds,$(shared_generate_targets))
shared_generate_targets += generate-crds-venconn


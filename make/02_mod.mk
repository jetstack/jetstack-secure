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
	./hack/e2e/test.sh

_bin/artifacts/preflight:  | $(NEEDS_GO)
	$(GO) build -o $@ .

examples/venafi-kubernetes-agent.yaml: $(helm_chart_archive) $(helm_chart_source_dir)/templates/configmap.yaml | $(NEEDS_HELM) $(NEEDS_YQ)
	$(HELM) template --show-only templates/configmap.yaml $(helm_chart_archive) \
	| $(YQ) '.data."config.yaml" | @yamld | .cluster_id |= "example-cluster-1" | .organization_id |= "example-organization-1"' > $@

.PHONY: build
build: _bin/artifacts/preflight

.PHONY: generate-example-venafi-kubernetes-agent
generate-example-venafi-kubernetes-agent: examples/venafi-kubernetes-agent.yaml

.PHONY: generate-helm-rbac
verify-helm-rbac: _bin/artifacts/preflight examples/venafi-kubernetes-agent.yaml | $(NEEDS_HELM)
	diff -u \
		<($(HELM) template deploy/charts/venafi-kubernetes-agent --show-only templates/rbac.yaml --namespace venafi --set fullnameOverride=venafi-kubernetes-agent | grep -v '# Source: ' | yq 'del(.metadata.labels)' | yq '[.]' | yq 'sort_by(.metadata.name)' -o yaml -P) \
		<(_bin/artifacts/preflight agent rbac -c examples/venafi-kubernetes-agent.yaml | yq '[.]' | yq 'sort_by(.metadata.name)' -o yaml -P)

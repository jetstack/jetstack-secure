repo_name := github.com/jetstack/preflight

license_ignore := gitlab.com/venafi,github.com/jetstack

kind_cluster_name := preflight
kind_cluster_config := $(bin_dir)/scratch/kind_cluster.yaml

build_names := preflight

go_preflight_main_dir := .
go_preflight_mod_dir := .
go_preflight_ldflags := \
	-X $(repo_name)/pkg/version.PreflightVersion=$(VERSION) \
	-X $(repo_name)/pkg/version.Commit=$(GITCOMMIT) \
	-X $(repo_name)/pkg/version.BuildDate=$(shell date "+%F-%T-%Z") \
	-X $(repo_name)/pkg/client.ClientID=k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo \
	-X $(repo_name)/pkg/client.ClientSecret=f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa \
	-X $(repo_name)/pkg/client.AuthServerDomain=auth.jetstack.io

oci_preflight_base_image_flavor := static
oci_preflight_image_name := quay.io/jetstack/venafi-agent
oci_preflight_image_tag := $(VERSION)
oci_preflight_image_name_development := jetstack.local/venafi-agent

# Annotations are the standardised set of annotations we set on every component we publish
oci_preflight_build_args := \
	--image-annotation="org.opencontainers.image.vendor"="CyberArk Software Ltd." \
	--image-annotation="org.opencontainers.image.licenses"="EULA - https://www.cyberark.com/contract-terms/" \
	--image-annotation="org.opencontainers.image.authors"="support@venafi.cloud" \
	--image-annotation="org.opencontainers.image.title"="Venafi Kubernetes Agent" \
	--image-annotation="org.opencontainers.image.description"="Gathers machine identity data from Kubernetes clusters." \
	--image-annotation="org.opencontainers.image.url"="https://www.cyberark.com/products/certificate-manager-for-kubernetes/" \
	--image-annotation="org.opencontainers.image.documentation"="https://docs.venafi.cloud/vaas/k8s-components/c-tlspk-agent-overview/" \
	--image-annotation="org.opencontainers.image.version"="$(VERSION)" \
	--image-annotation="org.opencontainers.image.revision"="$(GITCOMMIT)"

deploy_name := venafi-kubernetes-agent
deploy_namespace := venafi

helm_chart_source_dir := deploy/charts/venafi-kubernetes-agent
helm_chart_image_name := quay.io/jetstack/charts/venafi-kubernetes-agent
helm_chart_version := $(VERSION)
helm_labels_template_name := preflight.labels

# We skip using the upstream govulncheck targets because we need to customise the workflow YAML
# locally. We provide the targets in this repo instead, and manually maintain the workflow.
govulncheck_skip := true

# Allows us to replace the Helm values.yaml's image.repository and image.tag
# with the right values.
define helm_values_mutation_function
$(YQ) \
	'( .image.repository = "$(oci_preflight_image_name)" ) | \
	( .image.tag = "$(oci_preflight_image_tag)" )' \
	$1 --inplace
endef

golangci_lint_config := .golangci.yaml
go_header_file := /dev/null

include make/extra_tools.mk

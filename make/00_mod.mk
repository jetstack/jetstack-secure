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

deploy_name := venafi-kubernetes-agent
deploy_namespace := venafi

helm_chart_source_dir := deploy/charts/venafi-kubernetes-agent
helm_chart_image_name := quay.io/jetstack/charts/venafi-kubernetes-agent
helm_chart_version := $(VERSION)
helm_labels_template_name := preflight.labels

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

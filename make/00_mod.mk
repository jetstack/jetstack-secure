repo_name := github.com/jetstack/preflight

kind_cluster_name := preflight
kind_cluster_config := $(bin_dir)/scratch/kind_cluster.yaml

build_names := preflight

goos:=
GOARCH:=$(shell go env GOARCH)

go_preflight_main_dir := .
go_preflight_mod_dir := .
go_preflight_ldflags := \
	-X $(repo_name)/pkg/version.PreflightVersion=$(VERSION) \
	-X $(repo_name)/pkg/version.Commit=$(GITCOMMIT) \
	-X $(repo_name)/pkg/version.BuildDate="$(shell date -uR)" \
	-X $(repo_name)/pkg/client.ClientID="k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo" \
	-X $(repo_name)/pkg/client.ClientSecret="f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa" \
	-X $(repo_name)/pkg/client.AuthServerDomain="auth.jetstack.io"

oci_preflight_base_image_flavor := static
oci_preflight_image_name := quay.io/jetstack/preflight
oci_preflight_image_tag := $(VERSION)
oci_preflight_image_name_development := jetstack.local/preflight

deploy_name := venafi-kubernetes-agent
deploy_namespace := venafi

helm_chart_source_dir := deploy/charts/venafi-kubernetes-agent
helm_chart_name := venafi-kubernetes-agent
helm_chart_version := $(VERSION:v%=%)
helm_labels_template_name := preflight.labels
helm_docs_use_helm_tool := 1
helm_generate_schema := 1
helm_verify_values := 1

golangci_lint_config := .golangci.yaml

define helm_values_mutation_function
$(YQ) \
	'( .image.repository = "$(oci_preflight_image_name)" ) | \
	( .image.tag = "$(oci_preflight_image_tag)" )' \
	$1 --inplace
endef

go_header_file := /dev/null
build_names += ngts
go_ngts_main_dir := ./cmd/ark
go_ngts_mod_dir := .
go_ngts_ldflags := \
	-X $(gomodule_name)/pkg/version.PreflightVersion=$(VERSION) \
	-X $(gomodule_name)/pkg/version.Commit=$(GITCOMMIT) \
	-X $(gomodule_name)/pkg/version.BuildDate=$(shell date "+%F-%T-%Z")

oci_ngts_base_image_flavor := static
oci_ngts_image_name := quay.io/jetstack/discovery-agent
oci_ngts_image_tag := $(VERSION)
oci_ngts_image_name_development := jetstack.local/discovery-agent

# Annotations are the standardised set of annotations we set on every component we publish
oci_ngts_build_args := \
	--image-annotation="org.opencontainers.image.source"="https://github.com/jetstack/jetstack-secure" \
	--image-annotation="org.opencontainers.image.vendor"="Palo Alto Networks" \
	--image-annotation="org.opencontainers.image.licenses"="Apache-2.0" \
	--image-annotation="org.opencontainers.image.authors"="Palo Alto Networks" \
	--image-annotation="org.opencontainers.image.title"="Discovery Agent for NGTS" \
	--image-annotation="org.opencontainers.image.description"="Gathers machine identity data from Kubernetes clusters for NGTS." \
	--image-annotation="org.opencontainers.image.url"="https://www.paloaltonetworks.com/" \
	--image-annotation="org.opencontainers.image.documentation"="https://docs.paloaltonetworks.com/" \
	--image-annotation="org.opencontainers.image.version"="$(VERSION)" \
	--image-annotation="org.opencontainers.image.revision"="$(GITCOMMIT)"


define ngts_helm_values_mutation_function
echo "no mutations defined for this chart"
endef

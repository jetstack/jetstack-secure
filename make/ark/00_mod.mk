build_names += ark
go_ark_main_dir := ./cmd/ark
go_ark_mod_dir := .
go_ark_ldflags := \
	-X $(repo_name)/pkg/version.PreflightVersion=$(VERSION) \
	-X $(repo_name)/pkg/version.Commit=$(GITCOMMIT) \
	-X $(repo_name)/pkg/version.BuildDate=$(shell date "+%F-%T-%Z") \

oci_ark_base_image_flavor := static
oci_ark_image_name := quay.io/jetstack/ark-agent
oci_ark_image_tag := $(VERSION)
oci_ark_image_name_development := jetstack.local/ark-agent

# Annotations are the standardised set of annotations we set on every component we publish
oci_ark_build_args := \
	--image-annotation="org.opencontainers.image.source"="https://github.com/jetstack/jetstack-secure" \
	--image-annotation="org.opencontainers.image.vendor"="CyberArk Software Ltd." \
	--image-annotation="org.opencontainers.image.licenses"="EULA - https://www.cyberark.com/contract-terms/" \
	--image-annotation="org.opencontainers.image.authors"="TODO" \
	--image-annotation="org.opencontainers.image.title"="CyberArk Discovery and Context Agent" \
	--image-annotation="org.opencontainers.image.description"="Gathers machine identity data from Kubernetes clusters." \
	--image-annotation="org.opencontainers.image.url"="TODO" \
	--image-annotation="org.opencontainers.image.documentation"="TODO" \
	--image-annotation="org.opencontainers.image.version"="$(VERSION)" \
	--image-annotation="org.opencontainers.image.revision"="$(GITCOMMIT)"


define ark_helm_values_mutation_function
$(YQ) \
	'( .image.repository = "$(oci_ark_image_name)" ) | \
	( .image.tag = "$(oci_ark_image_tag)" )' \
	$1 --inplace
endef

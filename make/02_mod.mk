.PHONY: generate-manifests
generate-manifests: ## Generates jetstack.io_venaficonnections.yaml.
generate-manifests: | $(NEEDS_GO) $(NEEDS_YQ)
	$(GO) run ./make/connection_crd > $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml
	@echo "# DO NOT EDIT. Use 'make generate-manifests' to regenerate." >$(helm_chart_source_dir)/templates/venafi-connection-crd.without-validations.yaml
	$(YQ) 'del(.. | ."x-kubernetes-validations"?) | del(.metadata.creationTimestamp)' $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml >>$(helm_chart_source_dir)/templates/venafi-connection-crd.without-validations.yaml

	@echo "# DO NOT EDIT. Use 'make generate-manifests' to regenerate." >$(helm_chart_source_dir)/templates/venafi-connection-crd.yaml
	$(YQ) 'del(.metadata.creationTimestamp)' $(helm_chart_source_dir)/crd_bases/jetstack.io_venaficonnections.yaml >> $(helm_chart_source_dir)/templates/venafi-connection-crd.yaml

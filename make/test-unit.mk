.PHONY: test-unit
## Unit tests
## @category Testing
test-unit: | $(NEEDS_GO) $(NEEDS_GOTESTSUM) $(ARTIFACTS) $(NEEDS_ETCD) $(NEEDS_KUBE-APISERVER)
	$(GOTESTSUM) \
		--junitfile=$(ARTIFACTS)/junit-go-e2e.xml \
		-- \
		-coverprofile=$(ARTIFACTS)/filtered.cov \
		./api/... ./pkg/... \
		-- \
		-ldflags $(go_preflight_ldflags)

    export KUBEBUILDER_ASSETS=$(CURDIR)/$(bin_dir)/tools
	$(GO) tool cover -html=$(ARTIFACTS)/filtered.cov -o=$(ARTIFACTS)/filtered.html

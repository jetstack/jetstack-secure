.PHONY: test-unit
## Unit tests
## @category Testing
test-unit: | $(NEEDS_GO) $(NEEDS_GOTESTSUM) $(ARTIFACTS) $(NEEDS_ETCD) $(NEEDS_KUBE-APISERVER)
	KUBEBUILDER_ASSETS=$(CURDIR)/$(bin_dir)/tools \
	$(GOTESTSUM) \
		--junitfile=$(ARTIFACTS)/junit-go-e2e.xml \
		-- \
		-coverprofile=$(ARTIFACTS)/filtered.cov \
		./... \
		-- \
		-ldflags $(go_preflight_ldflags)

	$(GO) tool cover -func=$(ARTIFACTS)/filtered.cov
	$(GO) tool cover -html=$(ARTIFACTS)/filtered.cov -o=$(ARTIFACTS)/filtered.html

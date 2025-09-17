ADDITIONAL_TOOLS :=
ADDITIONAL_GO_DEPENDENCIES :=

# https://pkg.go.dev/github.com/helm-unittest/helm-unittest?tab=versions
ADDITIONAL_TOOLS += helm-unittest=v0.8.2
ADDITIONAL_GO_DEPENDENCIES += helm-unittest=github.com/helm-unittest/helm-unittest/cmd/helm-unittest

ADDITIONAL_TOOLS += venctl=1.16.0
ADDITIONAL_TOOLS += step=0.28.2

# https://docs.venafi.cloud/vaas/venctl/c-venctl-releases/
venctl_linux_amd64_SHA256SUM=26e7b7a7e134f1cf1f3ffacf4ae53ec6849058db5007ce4088d51f404ededb4a
venctl_darwin_amd64_SHA256SUM=2e76693901abcb2c018f66d3a10558c66ca09d1a3be912258bcd6c58e89aae80
venctl_darwin_arm64_SHA256SUM=4350912d67683773302655e2a0151320514d1ccf82ee99c895e6780f86b6f031

.PRECIOUS: $(DOWNLOAD_DIR)/tools/venctl@$(VENCTL_VERSION)_$(HOST_OS)_$(HOST_ARCH)
$(DOWNLOAD_DIR)/tools/venctl@$(VENCTL_VERSION)_$(HOST_OS)_$(HOST_ARCH): | $(DOWNLOAD_DIR)/tools
	@source $(lock_script) $@; \
		$(CURL) https://dl.venafi.cloud/venctl/$(VENCTL_VERSION)/venctl-$(HOST_OS)-$(HOST_ARCH).zip -o $(outfile).zip; \
		$(checkhash_script) $(outfile).zip $(venctl_$(HOST_OS)_$(HOST_ARCH)_SHA256SUM); \
		unzip -p $(outfile).zip venctl > $(outfile); \
		chmod +x $(outfile); \
		rm -f $(outfile).zip

# https://github.com/smallstep/cli/releases/
step_linux_amd64_SHA256SUM=2908f3c7d90181eec430070b231da5c0861e37537bf8e2388d031d3bd6c7b8c6
step_linux_arm64_SHA256SUM=96636a6cc980d53a98c72aa3b99e04f0b874a733d9ddf43fc6b0f1725f425c37
step_darwin_amd64_SHA256SUM=f6e9a9078cfc5f559c8213e023df6e8ebf8d9d36ffbd82749a41ee1c40a23623
step_darwin_arm64_SHA256SUM=b856702ee138a9badbe983e88758c0330907ea4f97e429000334ba038597db5b

.PRECIOUS: $(DOWNLOAD_DIR)/tools/step@$(STEP_VERSION)_$(HOST_OS)_$(HOST_ARCH)
$(DOWNLOAD_DIR)/tools/step@$(STEP_VERSION)_$(HOST_OS)_$(HOST_ARCH): | $(DOWNLOAD_DIR)/tools
	@source $(lock_script) $@; \
		$(CURL) https://dl.smallstep.com/gh-release/cli/gh-release-header/v$(STEP_VERSION)/step_$(HOST_OS)_$(STEP_VERSION)_$(HOST_ARCH).tar.gz -o $(outfile).tar.gz; \
		$(checkhash_script) $(outfile).tar.gz $(step_$(HOST_OS)_$(HOST_ARCH)_SHA256SUM); \
		tar xfO $(outfile).tar.gz step_$(STEP_VERSION)/bin/step > $(outfile); \
		chmod +x $(outfile); \
		rm -f $(outfile).tar.gz
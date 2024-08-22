ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION:=$(shell $(ROOT_DIR)/hack/getversion)
COMMIT:=$(shell git rev-list -1 HEAD)
DATE:=$(shell date -uR)
GOVERSION:=$(shell go version | awk '{print $$3 " " $$4}')
GOOS:=$(shell go env GOOS)
GOARCH:=$(shell go env GOARCH)

export GOPRIVATE=github.com/jetstack/venafi-connection-lib

BIN_NAME:=preflight

DOCKER_IMAGE?=quay.io/jetstack/preflight
DOCKER_IMAGE_TAG?=$(DOCKER_IMAGE):$(COMMIT)

# BUILD_IN decides if the binaries will be built in `docker` or in the `host`.
BUILD_IN?=docker

# OAuth2 config for the agent to work with platform.jetstack.io
OAUTH_CLIENT_ID?=k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo
OAUTH_CLIENT_SECRET?=f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa
OAUTH_AUTH_SERVER_DOMAIN?=auth.jetstack.io

define LDFLAGS
-X "github.com/jetstack/preflight/pkg/version.PreflightVersion=$(VERSION)" \
-X "github.com/jetstack/preflight/pkg/version.Platform=$(GOOS)/$(GOARCH)" \
-X "github.com/jetstack/preflight/pkg/version.Commit=$(COMMIT)" \
-X "github.com/jetstack/preflight/pkg/version.BuildDate=$(DATE)" \
-X "github.com/jetstack/preflight/pkg/version.GoVersion=$(GOVERSION)" \
-X "github.com/jetstack/preflight/pkg/client.ClientID=$(OAUTH_CLIENT_ID)" \
-X "github.com/jetstack/preflight/pkg/client.ClientSecret=$(OAUTH_CLIENT_SECRET)" \
-X "github.com/jetstack/preflight/pkg/client.AuthServerDomain=$(OAUTH_AUTH_SERVER_DOMAIN)"
endef

GO_BUILD:=CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
GO_INSTALL:=CGO_ENABLED=0 go install -ldflags '$(LDFLAGS)'

export GO111MODULE=on

clean:
	cd $(ROOT_DIR) && rm -rf ./builds ./bundles

# Golang cli

.PHONY: build
build:
	cd $(ROOT_DIR) && $(GO_BUILD) -o builds/preflight .

install:
	cd $(ROOT_DIR) && $(GO_INSTALL)

export KUBEBUILDER_ASSETS=$(ROOT_DIR)/_bin/tools
test: _bin/tools/etcd _bin/tools/kube-apiserver
test:
	cd $(ROOT_DIR) && go test ./...

vet:
	cd $(ROOT_DIR) && go vet ./...


.PHONY: ./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME)
./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME):
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o ./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME) .
.PHONY: ./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME)
./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME):
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(GO_BUILD) -o ./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME) .

build-all-platforms: build-all-platforms-in-$(BUILD_IN)

build-all-platforms-in-host:
	$(MAKE) GOOS=linux   GOARCH=amd64       ./builds/linux/amd64/$(BIN_NAME)
	$(MAKE) GOOS=linux   GOARCH=arm64       ./builds/linux/arm64/$(BIN_NAME)
	$(MAKE) GOOS=linux   GOARCH=arm GOARM=7 ./builds/linux/arm/v7/$(BIN_NAME)
	$(MAKE) GOOS=darwin  GOARCH=amd64       ./builds/darwin/amd64/$(BIN_NAME)
	$(MAKE) GOOS=windows GOARCH=amd64       ./builds/windows/amd64/$(BIN_NAME)

build-all-platforms-in-docker:
	rm -rf ./builds
	docker buildx build --load --rm -t preflight-bin -f ./builder.dockerfile \
		--build-arg oauth_client_id=$(OAUTH_CLIENT_ID) \
		--build-arg oauth_client_secret=$(OAUTH_CLIENT_SECRET) \
		--build-arg oauth_auth_server_domain=$(OAUTH_AUTH_SERVER_DOMAIN) \
		--ssh default \
		.
	docker rm -f preflight-bin-container 2>/dev/null || true
	docker create --rm --name=preflight-bin-container preflight-bin
	docker cp preflight-bin-container:/go/github.com/jetstack/preflight/builds ./builds
	docker rm preflight-bin-container
	docker rmi preflight-bin

# Docker image
PLATFORMS?=linux/arm/v7,linux/arm64/v8,linux/amd64
BUILDX_EXTRA_ARGS?=

push_buildx_args=--push $(BUILDX_EXTRA_ARGS)
push-canary_buildx_args=--tag $(DOCKER_IMAGE):canary --push $(BUILDX_EXTRA_ARGS)
build_buildx_args=$(BUILDX_EXTRA_ARGS)

.PHONY: _docker-%
_docker-%: build-all-platforms
	docker buildx build --platform $(PLATFORMS) \
	--tag $(DOCKER_IMAGE):$(VERSION) \
	--tag $(DOCKER_IMAGE):latest \
	--tag $(DOCKER_IMAGE):canary \
	$($*_buildx_args) \
	.

build-docker-image: _docker-build
push-docker-image: _docker-push

export COSIGN_REPOSITORY?=ghcr.io/jetstack/jetstack-secure/cosign
export COSIGN_EXPERIMENTAL=1

.PHONY: sign-docker-image
sign-docker-image:
	@cosign sign -y $(DOCKER_IMAGE):$(VERSION)

.PHONY: sbom-docker-image
sbom-docker-image:
	@syft $(DOCKER_IMAGE):$(VERSION) -o cyclonedx > bom.xml
	@cosign attach sbom --sbom bom.xml --type cyclonedx $(DOCKER_IMAGE):$(VERSION)
	@cosign sign -y --attachment sbom $(DOCKER_IMAGE):$(VERSION)

.PHONY: attest-docker-image
attest-docker-image:
	@cosign attest -y --type slsaprovenance --predicate predicate.json $(DOCKER_IMAGE):$(VERSION)

# A pre-commit hook is configured on this repository and can be installed using https://pre-commit.com/#3-install-the-git-hook-scripts
# This target can be used instead if the pre-commit hook is not desired
.PHONY: update-helm-docs
update-helm-docs:
	go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.11.0
	helm-docs --chart-search-root=deploy/charts/

# CI

export PATH:=$(GOPATH)/bin:$(PATH)

ci-deps:
	echo "ci-deps is going to be disabled. We are adopting Github actions"
	go install golang.org/x/lint/golint

ci-test: ci-deps test lint

ci-build: ci-test build build-docker-image build-all-platforms bundle-all-platforms push-docker-image-canary
	echo "ci-build is going to be disabled. We are adopting Github actions"

ci-publish: ci-build push-docker-image
	echo "ci-publish is going to be disabled. We are adopting Github actions"

# NOTE(mael): The download targets for yq, etcd, and kube-apiserver are a lesser
# and suboptimal version of what's in venafi-enhanced-issuer. We will migrate to
# makefile-modules and klone soon, so I didn't want to work too hard on this.

YQ_linux_amd64_SHA256SUM=bd695a6513f1196aeda17b174a15e9c351843fb1cef5f9be0af170f2dd744f08
YQ_darwin_amd64_SHA256SUM=b2ff70e295d02695b284755b2a41bd889cfb37454e1fa71abc3a6ec13b2676cf
YQ_darwin_arm64_SHA256SUM=e9fc15db977875de982e0174ba5dc2cf5ae4a644e18432a4262c96d4439b1686
YQ_VERSION=v4.35.1

_bin/downloaded/tools/yq@$(YQ_VERSION)_%:
	mkdir -p _bin/downloaded/tools
	curl -L https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$* -o $@
	./make/util/checkhash.sh $@ $(YQ_$*_SHA256SUM)
	chmod +x $@

HOST_OS=$(shell uname | tr '[:upper:]' '[:lower:]')
HOST_ARCH=$(shell uname -m | sed 's/x86_64/amd64/')

_bin/tools/yq: _bin/downloaded/tools/yq@$(YQ_VERSION)_$(HOST_OS)_$(HOST_ARCH)
	@mkdir -p _bin/tools
	@cd $(dir $@) && ln -sf $(patsubst _bin/%,../%,$<) $(notdir $@)

KUBEBUILDER_TOOLS_linux_amd64_SHA256SUM=f9699df7b021f71a1ab55329b36b48a798e6ae3a44d2132255fc7e46c6790d4d
KUBEBUILDER_TOOLS_darwin_amd64_SHA256SUM=e1913674bacaa70c067e15649237e1f67d891ba53f367c0a50786b4a274ee047
KUBEBUILDER_TOOLS_darwin_arm64_SHA256SUM=0422632a2bbb0d4d14d7d8b0f05497a4d041c11d770a07b7a55c44bcc5e8ce66
KUBEBUILDER_ASSETS_VERSION=1.27.1

_bin/downloaded/tools/etcd@$(KUBEBUILDER_ASSETS_VERSION)_%: _bin/downloaded/tools/kubebuilder_tools_$(KUBEBUILDER_ASSETS_VERSION)_%.tar.gz | _bin/downloaded/tools
	./make/util/checkhash.sh $< $(KUBEBUILDER_TOOLS_$*_SHA256SUM)
	@# O writes the specified file to stdout
	tar xfO $< kubebuilder/bin/etcd > $@ && chmod 775 $@

_bin/downloaded/tools/kube-apiserver@$(KUBEBUILDER_ASSETS_VERSION)_%: _bin/downloaded/tools/kubebuilder_tools_$(KUBEBUILDER_ASSETS_VERSION)_%.tar.gz | _bin/downloaded/tools
	./make/util/checkhash.sh $< $(KUBEBUILDER_TOOLS_$*_SHA256SUM)
	@# O writes the specified file to stdout
	tar xfO $< kubebuilder/bin/kube-apiserver > $@ && chmod 775 $@

_bin/downloaded/tools/kubebuilder_tools_$(KUBEBUILDER_ASSETS_VERSION)_$(HOST_OS)_$(HOST_ARCH).tar.gz: | _bin/downloaded/tools
	curl -L https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-$(KUBEBUILDER_ASSETS_VERSION)-$(HOST_OS)-$(HOST_ARCH).tar.gz -o $@

_bin/downloaded/tools:
	@mkdir -p $@

_bin/tools/etcd: _bin/downloaded/tools/etcd@$(KUBEBUILDER_ASSETS_VERSION)_$(HOST_OS)_$(HOST_ARCH)
	@mkdir -p _bin/tools
	@cd $(dir $@) && ln -sf $(patsubst _bin/%,../%,$<) $(notdir $@)

_bin/tools/kube-apiserver: _bin/downloaded/tools/kube-apiserver@$(KUBEBUILDER_ASSETS_VERSION)_$(HOST_OS)_$(HOST_ARCH)
	@mkdir -p _bin/tools
	@cd $(dir $@) && ln -sf $(patsubst _bin/%,../%,$<) $(notdir $@)
